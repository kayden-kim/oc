package stats

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
)

func LoadWindowReport(dir string, label string, start time.Time, end time.Time) (WindowReport, error) {
	dbPath, err := opencodedb.DBPath()
	if err != nil {
		return WindowReport{}, err
	}
	db, err := sql.Open("sqlite", opencodedb.SQLiteDSN(dbPath))
	if err != nil {
		return WindowReport{}, err
	}
	defer db.Close()
	return buildWindowReport(db, dir, label, start, end)
}

func buildWindowReport(db *sql.DB, dir string, label string, start time.Time, end time.Time) (WindowReport, error) {
	report := WindowReport{Label: label, Start: start, End: end}
	sessionAgg := map[string]*SessionUsage{}
	modelAgg := map[string]*ModelUsage{}
	projectAgg := map[string]projectUsage{}
	agentAgg := map[string]int{}
	agentModelAgg := map[string]int{}
	skillAgg := map[string]int{}
	toolAgg := map[string]int{}
	seenMessageCost := map[string]struct{}{}
	seenSessions := map[string]struct{}{}
	eventTimes := make([]int64, 0)
	loc := start.Location()
	if loc == nil {
		loc = time.Local
	}

	messageRows, err := loadWindowMessages(db, dir, start, end)
	if err != nil {
		return WindowReport{}, err
	}
	for _, row := range messageRows {
		if row.Summary || strings.EqualFold(row.Agent, "compaction") || row.Role != "assistant" {
			continue
		}
		report.Messages++
		report.Cost += row.Cost
		if row.Agent != "" {
			report.TotalSubtasks++
			agentAgg[row.Agent]++
		}
		seenSessions[row.SessionID] = struct{}{}
		eventTimes = append(eventTimes, row.CreatedAt)
		session := ensureSessionUsage(sessionAgg, row.SessionID, row.Title)
		session.Messages++
		session.Cost += row.Cost
	}

	partRows, err := loadWindowParts(db, dir, start, end)
	if err != nil {
		return WindowReport{}, err
	}
	for _, row := range partRows {
		if row.Summary || strings.EqualFold(row.MessageAgent, "compaction") || row.Type == "compaction" {
			continue
		}
		seenSessions[row.SessionID] = struct{}{}
		eventTimes = append(eventTimes, row.CreatedAt)
		session := ensureSessionUsage(sessionAgg, row.SessionID, row.Title)
		if row.Type == "tool" {
			report.TotalToolCalls++
			if row.Tool != "" {
				toolAgg[row.Tool]++
			}
			if row.Tool == "skill" {
				report.TotalSkillCalls++
				if row.SkillName != "" {
					skillAgg[row.SkillName]++
				}
			}
		}
		if row.Type != "step-finish" {
			continue
		}

		modelKey := modelLabel(row.ProviderID, row.ModelID)
		if strings.TrimSpace(modelKey) == "" {
			modelKey = row.ModelID
		}
		model := ensureModelUsage(modelAgg, modelKey)
		report.InputTokens += row.InputTokens
		report.OutputTokens += row.OutputTokens
		report.CacheReadTokens += row.CacheReadTokens
		report.CacheWriteTokens += row.CacheWriteTokens
		report.ReasoningTokens += row.ReasoningTokens
		model.InputTokens += row.InputTokens
		model.OutputTokens += row.OutputTokens
		model.CacheReadTokens += row.CacheReadTokens
		model.CacheWriteTokens += row.CacheWriteTokens
		model.ReasoningTokens += row.ReasoningTokens
		totalTokens := row.InputTokens + row.OutputTokens + row.CacheReadTokens + row.CacheWriteTokens + row.ReasoningTokens
		model.TotalTokens += totalTokens
		report.Tokens += totalTokens
		session.Tokens += totalTokens
		stamp := opencodedb.UnixTimestampToTime(row.CreatedAt).In(loc)
		report.HalfHourSlots[stamp.Hour()*2+stamp.Minute()/30] += totalTokens
		agentName := strings.TrimSpace(row.Agent)
		if agentName == "" {
			agentName = strings.TrimSpace(row.MessageAgent)
		}
		modelName := strings.TrimSpace(row.ModelID)
		if agentName != "" && modelName != "" && !strings.EqualFold(agentName, "compaction") {
			agentModelAgg[agentModelUsageKey(agentName, modelName)]++
			report.TotalAgentModelCalls++
		}
		projectKey := normalizeProjectUsageKey(row.Directory)
		if projectKey != "" {
			usage := projectAgg[projectKey]
			usage.Tokens += totalTokens
			projectAgg[projectKey] = usage
		}

		if row.MessageCost > 0 {
			if _, ok := seenMessageCost[row.MessageID]; !ok {
				seenMessageCost[row.MessageID] = struct{}{}
				model.Cost += row.MessageCost
				projectKey := normalizeProjectUsageKey(row.Directory)
				if projectKey != "" {
					usage := projectAgg[projectKey]
					usage.Cost += row.MessageCost
					projectAgg[projectKey] = usage
				}
			}
			continue
		}

		cost := row.Cost
		if cost <= 0 {
			event := partEvent{
				ProviderID:       row.ProviderID,
				ModelID:          row.ModelID,
				InputTokens:      row.InputTokens,
				OutputTokens:     row.OutputTokens,
				ReasoningTokens:  row.ReasoningTokens,
				CacheReadTokens:  row.CacheReadTokens,
				CacheWriteTokens: row.CacheWriteTokens,
			}
			estimatedCost, err := estimatePartCost(event)
			if err != nil {
				return WindowReport{}, fmt.Errorf("estimate window cost: %w", err)
			}
			cost = estimatedCost
		}
		report.Cost += cost
		model.Cost += cost
		session.Cost += cost
		projectKey = normalizeProjectUsageKey(row.Directory)
		if projectKey != "" {
			usage := projectAgg[projectKey]
			usage.Cost += cost
			projectAgg[projectKey] = usage
		}
	}

	report.Sessions = len(seenSessions)
	report.Models = collectSortedModels(modelAgg)
	report.AllSessions = collectSortedSessions(sessionAgg)
	report.TopSessions = append([]SessionUsage(nil), report.AllSessions...)
	if len(report.TopSessions) > 8 {
		report.TopSessions = report.TopSessions[:8]
	}
	report.TopProjects = topUsageAmountsWithCosts(projectAgg, unlimitedUsageItems)
	report.TopAgents = topUsageCounts(agentAgg, unlimitedUsageItems)
	report.TopAgentModels = topUsageCounts(agentModelAgg, unlimitedUsageItems)
	report.TopSkills = topUsageCounts(skillAgg, unlimitedUsageItems)
	report.TopTools = topUsageCounts(toolAgg, unlimitedUsageItems)
	report.UniqueProjectCount = len(projectAgg)
	for _, usage := range projectAgg {
		report.TotalProjectCost += usage.Cost
	}
	if report.CodeLines, err = loadWindowCodeLines(db, dir, start, end); err != nil {
		return WindowReport{}, err
	}
	if report.ChangedFiles, err = loadWindowChangedFiles(db, dir, start, end); err != nil {
		return WindowReport{}, err
	}
	report.ActiveMinutes = computeSessionMinutes(eventTimes, defaultSessionGapMinutes)
	return report, nil
}

func loadWindowCodeLines(db *sql.DB, dir string, start, end time.Time) (int, error) {
	hasColumns, err := hasSessionSummaryColumns(db)
	if err != nil {
		return 0, err
	}
	if !hasColumns {
		return 0, nil
	}

	query := `
		SELECT
			CAST(COALESCE(s.summary_additions, 0) AS INTEGER),
			CAST(COALESCE(s.summary_deletions, 0) AS INTEGER)
		FROM session s
		WHERE %s s.time_updated >= ? AND s.time_updated < ?
	`
	args := []any{start.UnixMilli(), end.UnixMilli()}
	wherePrefix := ""
	if dir != "" {
		wherePrefix = scopedDirectoryClause() + " AND "
		args = []any{scopedDirectoryArg(dir), start.UnixMilli(), end.UnixMilli()}
	}
	rows, err := db.Query(fmt.Sprintf(query, wherePrefix), args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var additions int
		var deletions int
		if err := rows.Scan(&additions, &deletions); err != nil {
			return 0, err
		}
		total += additions + deletions
	}
	return total, rows.Err()
}

func loadWindowChangedFiles(db *sql.DB, dir string, start, end time.Time) (int, error) {
	query := `
		SELECT
			CAST(COALESCE(p.data, '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM part p
		%s
		LEFT JOIN message m ON m.id = p.message_id
		WHERE p.time_created >= ? AND p.time_created < ?
	`
	joinClause := ""
	args := []any{start.UnixMilli(), end.UnixMilli()}
	if dir != "" {
		joinClause = "JOIN session s ON s.id = p.session_id"
		query = strings.Replace(query, "%s", joinClause, 1)
		query = strings.Replace(query, "WHERE p.time_created >= ? AND p.time_created < ?", "WHERE "+scopedDirectoryClause()+" AND p.time_created >= ? AND p.time_created < ?", 1)
		args = []any{scopedDirectoryArg(dir), start.UnixMilli(), end.UnixMilli()}
	} else {
		query = strings.Replace(query, "%s", "", 1)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	seen := map[string]struct{}{}
	for rows.Next() {
		var raw string
		var summary int64
		var messageAgent string
		if err := rows.Scan(&raw, &summary, &messageAgent); err != nil {
			return 0, err
		}
		if summary != 0 || strings.EqualFold(messageAgent, "compaction") {
			continue
		}
		for _, file := range extractChangedFilesFromPart(raw) {
			normalized := normalizeChangedFilePath(file)
			if normalized == "" {
				continue
			}
			seen[normalized] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return len(seen), nil
}

func loadWindowMessages(db *sql.DB, dir string, start, end time.Time) ([]windowMessageRow, error) {
	query := `
		SELECT m.id, m.session_id, CAST(COALESCE(s.title, '') AS TEXT), CAST(COALESCE(s.directory, '') AS TEXT), m.time_created,
		       CAST(COALESCE(json_extract(m.data, '$.role'), '') AS TEXT),
		       CAST(COALESCE(json_extract(m.data, '$.cost'), 0) AS REAL),
		       CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
		       CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM message m
		LEFT JOIN session s ON s.id = m.session_id
		WHERE m.time_created >= ? AND m.time_created < ?
	`
	args := []any{start.UnixMilli(), end.UnixMilli()}
	if dir != "" {
		query += " AND " + scopedDirectoryClause()
		args = append(args, scopedDirectoryArg(dir))
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []windowMessageRow{}
	for rows.Next() {
		var row windowMessageRow
		var summary int64
		if err := rows.Scan(&row.MessageID, &row.SessionID, &row.Title, &row.Directory, &row.CreatedAt, &row.Role, &row.Cost, &summary, &row.Agent); err != nil {
			return nil, err
		}
		row.Summary = summary != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadWindowParts(db *sql.DB, dir string, start, end time.Time) ([]windowPartRow, error) {
	query := `
		SELECT p.message_id, p.session_id, CAST(COALESCE(s.title, '') AS TEXT), CAST(COALESCE(s.directory, '') AS TEXT), p.time_created,
		       CAST(COALESCE(json_extract(p.data, '$.type'), '') AS TEXT),
		       CAST(COALESCE(json_extract(p.data, '$.tool'), '') AS TEXT),
		       CAST(COALESCE(json_type(p.data, '$.state.input.name'), '') AS TEXT),
		       CAST(COALESCE(json_extract(p.data, '$.state.input.name'), '') AS TEXT),
		       CAST(COALESCE(json_extract(p.data, '$.agent'), '') AS TEXT),
		       CAST(COALESCE(json_extract(m.data, '$.providerID'), '') AS TEXT),
		       CAST(COALESCE(json_extract(m.data, '$.modelID'), '') AS TEXT),
		       CAST(COALESCE(json_extract(p.data, '$.cost'), 0) AS REAL),
		       CAST(COALESCE(json_extract(m.data, '$.cost'), 0) AS REAL),
		       CAST(COALESCE(json_extract(p.data, '$.tokens.input'), 0) AS INTEGER),
		       CAST(COALESCE(json_extract(p.data, '$.tokens.output'), 0) AS INTEGER),
		       CAST(COALESCE(json_extract(p.data, '$.tokens.reasoning'), 0) AS INTEGER),
		       CAST(COALESCE(json_extract(p.data, '$.tokens.cache.read'), 0) AS INTEGER),
		       CAST(COALESCE(json_extract(p.data, '$.tokens.cache.write'), 0) AS INTEGER),
		       CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
		       CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM part p
		LEFT JOIN message m ON m.id = p.message_id
		LEFT JOIN session s ON s.id = p.session_id
		WHERE p.time_created >= ? AND p.time_created < ?
	`
	args := []any{start.UnixMilli(), end.UnixMilli()}
	if dir != "" {
		query += " AND " + scopedDirectoryClause()
		args = append(args, scopedDirectoryArg(dir))
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []windowPartRow{}
	for rows.Next() {
		var row windowPartRow
		var summary int64
		var skillNameType string
		if err := rows.Scan(&row.MessageID, &row.SessionID, &row.Title, &row.Directory, &row.CreatedAt, &row.Type, &row.Tool, &skillNameType, &row.SkillName, &row.Agent, &row.ProviderID, &row.ModelID, &row.Cost, &row.MessageCost, &row.InputTokens, &row.OutputTokens, &row.ReasoningTokens, &row.CacheReadTokens, &row.CacheWriteTokens, &summary, &row.MessageAgent); err != nil {
			return nil, err
		}
		if skillNameType != "text" {
			row.SkillName = ""
		} else {
			row.SkillName = strings.TrimSpace(row.SkillName)
		}
		row.Summary = summary != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

func LoadMonthDailyReport(dir string, monthStart time.Time) (MonthDailyReport, error) {
	dbPath, err := opencodedb.DBPath()
	if err != nil {
		return MonthDailyReport{}, err
	}
	db, err := sql.Open("sqlite", opencodedb.SQLiteDSN(dbPath))
	if err != nil {
		return MonthDailyReport{}, err
	}
	defer db.Close()
	return buildMonthDailyReport(db, dir, monthStart)
}

func LoadYearMonthlyReport(dir string, endMonth time.Time) (YearMonthlyReport, error) {
	dbPath, err := opencodedb.DBPath()
	if err != nil {
		return YearMonthlyReport{}, err
	}
	db, err := sql.Open("sqlite", opencodedb.SQLiteDSN(dbPath))
	if err != nil {
		return YearMonthlyReport{}, err
	}
	defer db.Close()
	return buildYearMonthlyReport(db, dir, endMonth)
}
