package stats

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

func LoadWindowReport(dir string, label string, start time.Time, end time.Time) (WindowReport, error) {
	dbPath, err := opencodeDBPath()
	if err != nil {
		return WindowReport{}, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
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
	seenMessageCost := map[string]struct{}{}
	seenSessions := map[string]struct{}{}

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
		seenSessions[row.SessionID] = struct{}{}
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
		session := ensureSessionUsage(sessionAgg, row.SessionID, row.Title)
		if row.Type != "step-finish" {
			continue
		}

		model := ensureModelUsage(modelAgg, row.ModelID)
		model.InputTokens += row.InputTokens
		model.OutputTokens += row.OutputTokens
		model.CacheReadTokens += row.CacheReadTokens
		model.CacheWriteTokens += row.CacheWriteTokens
		model.ReasoningTokens += row.ReasoningTokens
		totalTokens := row.InputTokens + row.OutputTokens + row.CacheReadTokens + row.CacheWriteTokens + row.ReasoningTokens
		model.TotalTokens += totalTokens
		report.Tokens += totalTokens
		session.Tokens += totalTokens

		if row.MessageCost > 0 {
			if _, ok := seenMessageCost[row.MessageID]; !ok {
				seenMessageCost[row.MessageID] = struct{}{}
				model.Cost += row.MessageCost
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
	}

	report.Sessions = len(seenSessions)
	report.Models = collectSortedModels(modelAgg)
	report.TopSessions = collectSortedSessions(sessionAgg)
	if len(report.TopSessions) > 8 {
		report.TopSessions = report.TopSessions[:8]
	}
	return report, nil
}

func loadWindowMessages(db *sql.DB, dir string, start, end time.Time) ([]windowMessageRow, error) {
	query := `
		SELECT m.id, m.session_id, CAST(COALESCE(s.title, '') AS TEXT), m.time_created,
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
		if err := rows.Scan(&row.MessageID, &row.SessionID, &row.Title, &row.CreatedAt, &row.Role, &row.Cost, &summary, &row.Agent); err != nil {
			return nil, err
		}
		row.Summary = summary != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

func loadWindowParts(db *sql.DB, dir string, start, end time.Time) ([]windowPartRow, error) {
	query := `
		SELECT p.message_id, p.session_id, CAST(COALESCE(s.title, '') AS TEXT), p.time_created,
		       CAST(COALESCE(json_extract(p.data, '$.type'), '') AS TEXT),
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
		if err := rows.Scan(&row.MessageID, &row.SessionID, &row.Title, &row.CreatedAt, &row.Type, &row.ProviderID, &row.ModelID, &row.Cost, &row.MessageCost, &row.InputTokens, &row.OutputTokens, &row.ReasoningTokens, &row.CacheReadTokens, &row.CacheWriteTokens, &summary, &row.MessageAgent); err != nil {
			return nil, err
		}
		row.Summary = summary != 0
		result = append(result, row)
	}
	return result, rows.Err()
}

func ensureSessionUsage(m map[string]*SessionUsage, id, title string) *SessionUsage {
	if usage, ok := m[id]; ok {
		if usage.Title == "" && title != "" {
			usage.Title = title
		}
		return usage
	}
	usage := &SessionUsage{ID: id, Title: title}
	m[id] = usage
	return usage
}

func ensureModelUsage(m map[string]*ModelUsage, model string) *ModelUsage {
	key := strings.TrimSpace(model)
	if usage, ok := m[key]; ok {
		return usage
	}
	usage := &ModelUsage{Model: key}
	m[key] = usage
	return usage
}

func collectSortedModels(m map[string]*ModelUsage) []ModelUsage {
	result := make([]ModelUsage, 0, len(m))
	for _, item := range m {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Cost == result[j].Cost {
			return result[i].TotalTokens > result[j].TotalTokens
		}
		return result[i].Cost > result[j].Cost
	})
	return result
}

func collectSortedSessions(m map[string]*SessionUsage) []SessionUsage {
	result := make([]SessionUsage, 0, len(m))
	for _, item := range m {
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Cost == result[j].Cost {
			if result[i].Tokens == result[j].Tokens {
				return result[i].Messages > result[j].Messages
			}
			return result[i].Tokens > result[j].Tokens
		}
		return result[i].Cost > result[j].Cost
	})
	return result
}

func LoadMonthDailyReport(dir string, monthStart time.Time) (MonthDailyReport, error) {
	dbPath, err := opencodeDBPath()
	if err != nil {
		return MonthDailyReport{}, err
	}
	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return MonthDailyReport{}, err
	}
	defer db.Close()
	return buildMonthDailyReport(db, dir, monthStart)
}

func buildMonthDailyReport(db *sql.DB, dir string, monthStart time.Time) (MonthDailyReport, error) {
	// Normalize monthStart to beginning of month
	monthStart = startOfMonth(monthStart)
	monthEnd := monthStart.AddDate(0, 1, 0)

	report := MonthDailyReport{
		MonthStart: monthStart,
		MonthEnd:   monthEnd,
		Days:       []DailySummary{},
	}

	// Map to accumulate daily stats
	dayMap := make(map[string]*DailySummary)

	// Load all messages and parts for the month
	messageRows, err := loadWindowMessages(db, dir, monthStart, monthEnd)
	if err != nil {
		return MonthDailyReport{}, err
	}

	seenSessions := make(map[string]struct{})

	// Process messages
	for _, row := range messageRows {
		if row.Summary || strings.EqualFold(row.Agent, "compaction") || row.Role != "assistant" {
			continue
		}

		date := startOfDay(unixTimestampToTime(row.CreatedAt))
		key := dayKey(date)
		if _, ok := dayMap[key]; !ok {
			dayMap[key] = &DailySummary{Date: date, FocusTag: "--"}
		}

		dayMap[key].Messages++
		dayMap[key].Cost += row.Cost
		seenSessions[row.SessionID] = struct{}{}
	}

	partRows, err := loadWindowParts(db, dir, monthStart, monthEnd)
	if err != nil {
		return MonthDailyReport{}, err
	}

	// Process parts (tokens and cost)
	seenMessageCost := make(map[string]struct{})

	for _, row := range partRows {
		if row.Summary || strings.EqualFold(row.MessageAgent, "compaction") || row.Type == "compaction" {
			continue
		}

		seenSessions[row.SessionID] = struct{}{}
		date := startOfDay(unixTimestampToTime(row.CreatedAt))
		key := dayKey(date)
		if _, ok := dayMap[key]; !ok {
			dayMap[key] = &DailySummary{Date: date, FocusTag: "--"}
		}

		if row.Type != "step-finish" {
			continue
		}

		// Accumulate tokens
		totalTokens := row.InputTokens + row.OutputTokens + row.CacheReadTokens + row.CacheWriteTokens + row.ReasoningTokens
		dayMap[key].Tokens += totalTokens
		report.TotalTokens += totalTokens

		// Handle cost accounting
		if row.MessageCost > 0 {
			if _, ok := seenMessageCost[row.MessageID]; !ok {
				seenMessageCost[row.MessageID] = struct{}{}
				dayMap[key].Cost += row.MessageCost
				report.TotalCost += row.MessageCost
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
				return MonthDailyReport{}, fmt.Errorf("estimate month daily cost: %w", err)
			}
			cost = estimatedCost
		}
		dayMap[key].Cost += cost
		report.TotalCost += cost
	}

	// Collect and sort daily summaries
	days := make([]DailySummary, 0, len(dayMap))
	allTokens := make([]int64, 0, len(dayMap))
	allCosts := make([]float64, 0, len(dayMap))

	for _, day := range dayMap {
		if day.Messages > 0 || day.Tokens > 0 || day.Cost > 0 {
			report.ActiveDays++
		}
		report.TotalMessages += day.Messages
		days = append(days, *day)
		if day.Tokens > 0 {
			allTokens = append(allTokens, day.Tokens)
		}
		if day.Cost > 0 {
			allCosts = append(allCosts, day.Cost)
		}
	}

	// Sort by date
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date.Before(days[j].Date)
	})

	// Derive focus tags for each day
	for i := range days {
		days[i].FocusTag = deriveFocusTag(days[i].Tokens, days[i].Cost, allTokens, allCosts)
	}

	// Count unique sessions
	report.TotalSessions = len(seenSessions)

	report.Days = days
	return report, nil
}
