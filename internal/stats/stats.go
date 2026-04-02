package stats

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kayden-kim/oc/internal/opencodedb"
)

const dayWindow = 30

const defaultSessionGapMinutes = 15

const unlimitedUsageItems = 0

func LoadGlobal() (Report, error) {
	return loadAtWithOptions(time.Now(), "", Options{})
}

func LoadForDir(dir string) (Report, error) {
	return loadAtWithOptions(time.Now(), dir, Options{})
}

func LoadGlobalWithOptions(options Options) (Report, error) {
	return loadAtWithOptions(time.Now(), "", options)
}

func LoadForDirWithOptions(dir string, options Options) (Report, error) {
	return loadAtWithOptions(time.Now(), dir, options)
}

func loadGlobalAt(now time.Time) (Report, error) {
	return loadAtWithOptions(now, "", Options{})
}

func loadForDirAt(dir string, now time.Time) (Report, error) {
	return loadAtWithOptions(now, dir, Options{})
}

func loadForDirAtWithOptions(dir string, now time.Time, options Options) (Report, error) {
	return loadAtWithOptions(now, dir, options)
}

func normalizeProjectUsageKey(directory string) string {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "(unknown project)"
	}
	cleaned := filepath.Clean(directory)
	if runtime.GOOS == "windows" {
		return strings.ToLower(filepath.ToSlash(cleaned))
	}
	return cleaned
}

func buildEmptyDays(now time.Time) []Day {
	start := startOfDay(now).AddDate(0, 0, -(dayWindow - 1))
	days := make([]Day, 0, dayWindow)
	for i := 0; i < dayWindow; i++ {
		days = append(days, Day{
			Date:              start.AddDate(0, 0, i),
			ToolCounts:        map[string]int{},
			SkillCounts:       map[string]int{},
			AgentCounts:       map[string]int{},
			AgentModelCounts:  map[string]int{},
			ModelCounts:       map[string]int64{},
			ModelCosts:        map[string]float64{},
			eventTimes:        nil,
			UniqueTools:       map[string]struct{}{},
			UniqueSkills:      map[string]struct{}{},
			UniqueAgents:      map[string]struct{}{},
			UniqueAgentModels: map[string]struct{}{},
		})
	}
	return days
}

func mergeMessageStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	query := `
		SELECT
			m.time_created,
			CAST(COALESCE(json_extract(m.data, '$.role'), '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.cost'), 0) AS REAL),
			CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM message m
		%s
		WHERE m.time_created >= ?
	`
	joinClause := ""
	args := []any{since}
	if dir != "" {
		joinClause = "JOIN session s ON s.id = m.session_id"
		query = strings.Replace(query, "%s", joinClause, 1)
		query = strings.Replace(query, "WHERE m.time_created >= ?", "WHERE "+scopedDirectoryClause()+" AND m.time_created >= ?", 1)
		args = []any{scopedDirectoryArg(dir), since}
	} else {
		query = strings.Replace(query, "%s", "", 1)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var event messageEvent
		var summary int64
		if err := rows.Scan(&event.CreatedAt, &event.Role, &event.Cost, &summary, &event.Agent); err != nil {
			return err
		}
		event.Summary = summary != 0
		if event.Summary || strings.EqualFold(event.Agent, "compaction") {
			continue
		}
		if event.Role != "assistant" {
			continue
		}
		day := dayMap[dayKey(opencodedb.UnixTimestampToTime(event.CreatedAt).In(loc))]
		if day == nil {
			continue
		}
		day.AssistantMessages++
		day.eventTimes = append(day.eventTimes, event.CreatedAt)
		if event.Agent != "" {
			day.Subtasks++
			day.AgentCounts[event.Agent]++
			day.UniqueAgents[event.Agent] = struct{}{}
		}
		day.Cost += event.Cost
	}

	return rows.Err()
}

func mergePartStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	query := `
		SELECT
			p.message_id,
			p.time_created,
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
		%s
		LEFT JOIN message m ON m.id = p.message_id
		WHERE p.time_created >= ?
	`
	joinClause := ""
	args := []any{since}
	if dir != "" {
		joinClause = "JOIN session s ON s.id = p.session_id"
		query = strings.Replace(query, "%s", joinClause, 1)
		query = strings.Replace(query, "WHERE p.time_created >= ?", "WHERE "+scopedDirectoryClause()+" AND p.time_created >= ?", 1)
		args = []any{scopedDirectoryArg(dir), since}
	} else {
		query = strings.Replace(query, "%s", "", 1)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	seenModelMessageCost := map[string]struct{}{}
	for rows.Next() {
		var event partEvent
		var summary int64
		var skillNameType string
		if err := rows.Scan(
			&event.MessageID,
			&event.CreatedAt,
			&event.Type,
			&event.Tool,
			&skillNameType,
			&event.SkillName,
			&event.Agent,
			&event.ProviderID,
			&event.ModelID,
			&event.Cost,
			&event.MessageCost,
			&event.InputTokens,
			&event.OutputTokens,
			&event.ReasoningTokens,
			&event.CacheReadTokens,
			&event.CacheWriteTokens,
			&summary,
			&event.MessageAgent,
		); err != nil {
			return err
		}
		if skillNameType != "text" {
			event.SkillName = ""
		} else {
			event.SkillName = strings.TrimSpace(event.SkillName)
		}
		event.Summary = summary != 0
		if event.Summary || strings.EqualFold(event.MessageAgent, "compaction") || event.Type == "compaction" {
			continue
		}
		day := dayMap[dayKey(opencodedb.UnixTimestampToTime(event.CreatedAt).In(loc))]
		if day == nil {
			continue
		}
		day.eventTimes = append(day.eventTimes, event.CreatedAt)
		switch event.Type {
		case "tool":
			day.ToolCalls++
			if event.Tool != "" {
				day.ToolCounts[event.Tool]++
				day.UniqueTools[event.Tool] = struct{}{}
			}
			if event.Tool == "skill" {
				day.SkillCalls++
				if event.SkillName != "" {
					day.SkillCounts[event.SkillName]++
					day.UniqueSkills[event.SkillName] = struct{}{}
				}
			}
		case "step-finish":
			day.StepFinishes++
			stepCost := 0.0
			if event.MessageCost > 0 {
				if _, ok := seenModelMessageCost[event.MessageID]; !ok {
					seenModelMessageCost[event.MessageID] = struct{}{}
					stepCost = event.MessageCost
				}
			} else if event.Cost > 0 {
				day.Cost += event.Cost
				stepCost = event.Cost
			} else {
				estimatedCost, err := estimatePartCost(event)
				if err != nil {
					return fmt.Errorf("estimate step-finish cost: %w", err)
				}
				day.Cost += estimatedCost
				stepCost = estimatedCost
			}
			stepTokens := event.InputTokens + event.OutputTokens + event.ReasoningTokens + event.CacheReadTokens + event.CacheWriteTokens
			day.Tokens += stepTokens
			day.ReasoningTokens += event.ReasoningTokens
			st := opencodedb.UnixTimestampToTime(event.CreatedAt).In(loc)
			day.SlotTokens[st.Hour()*2+st.Minute()/30] += stepTokens
			name := modelLabel(event.ProviderID, event.ModelID)
			modelName := strings.TrimSpace(event.ModelID)
			if name != "" {
				day.ModelCounts[name] += stepTokens
				day.ModelCosts[name] += stepCost
			}
			agentName := strings.TrimSpace(event.Agent)
			if agentName == "" {
				agentName = strings.TrimSpace(event.MessageAgent)
			}
			if agentName != "" && !strings.EqualFold(agentName, "compaction") && modelName != "" {
				key := agentModelUsageKey(agentName, modelName)
				day.AgentModelCounts[key]++
				day.UniqueAgentModels[key] = struct{}{}
			}
		}
	}

	return rows.Err()
}

func mergeSessionCodeStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	hasColumns, err := hasSessionSummaryColumns(db)
	if err != nil {
		return err
	}
	if !hasColumns {
		return nil
	}

	query := `
		SELECT
			s.time_updated,
			CAST(COALESCE(s.summary_additions, 0) AS INTEGER),
			CAST(COALESCE(s.summary_deletions, 0) AS INTEGER)
		FROM session s
		WHERE %s s.time_updated >= ?
	`
	args := []any{since}
	wherePrefix := ""
	if dir != "" {
		wherePrefix = scopedDirectoryClause() + " AND "
		args = []any{scopedDirectoryArg(dir), since}
	}
	rows, err := db.Query(fmt.Sprintf(query, wherePrefix), args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var updatedAt int64
		var additions int
		var deletions int
		if err := rows.Scan(&updatedAt, &additions, &deletions); err != nil {
			return err
		}
		day := dayMap[dayKey(opencodedb.UnixTimestampToTime(updatedAt).In(loc))]
		if day == nil {
			continue
		}
		day.CodeLines += additions + deletions
	}

	return rows.Err()
}

func mergePartFileStats(db *sql.DB, dir string, since int64, loc *time.Location, dayMap map[string]*Day) error {
	query := `
		SELECT
			p.time_created,
			CAST(COALESCE(p.data, '') AS TEXT),
			CAST(COALESCE(json_extract(m.data, '$.summary'), 0) AS INTEGER),
			CAST(COALESCE(json_extract(m.data, '$.agent'), '') AS TEXT)
		FROM part p
		%s
		LEFT JOIN message m ON m.id = p.message_id
		WHERE p.time_created >= ?
	`
	joinClause := ""
	args := []any{since}
	if dir != "" {
		joinClause = "JOIN session s ON s.id = p.session_id"
		query = strings.Replace(query, "%s", joinClause, 1)
		query = strings.Replace(query, "WHERE p.time_created >= ?", "WHERE "+scopedDirectoryClause()+" AND p.time_created >= ?", 1)
		args = []any{scopedDirectoryArg(dir), since}
	} else {
		query = strings.Replace(query, "%s", "", 1)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	seenByDay := make(map[string]map[string]struct{}, len(dayMap))
	for rows.Next() {
		var createdAt int64
		var raw string
		var summary int64
		var messageAgent string
		if err := rows.Scan(&createdAt, &raw, &summary, &messageAgent); err != nil {
			return err
		}
		if summary != 0 || strings.EqualFold(messageAgent, "compaction") {
			continue
		}
		bucketKey := dayKey(opencodedb.UnixTimestampToTime(createdAt).In(loc))
		day := dayMap[bucketKey]
		if day == nil {
			continue
		}
		files := extractChangedFilesFromPart(raw)
		if len(files) == 0 {
			continue
		}
		seen := seenByDay[bucketKey]
		if seen == nil {
			seen = map[string]struct{}{}
			seenByDay[bucketKey] = seen
		}
		for _, file := range files {
			normalized := normalizeChangedFilePath(file)
			if normalized == "" {
				continue
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			day.ChangedFiles++
		}
	}

	return rows.Err()
}

func extractChangedFilesFromPart(raw string) []string {
	type partPayload struct {
		Type  string   `json:"type"`
		Tool  string   `json:"tool"`
		Files []string `json:"files"`
		State struct {
			Status string `json:"status"`
			Input  struct {
				FilePath  string `json:"filePath"`
				PatchText string `json:"patchText"`
			} `json:"input"`
		} `json:"state"`
	}

	var payload partPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}

	fileSet := map[string]struct{}{}
	addFile := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		fileSet[path] = struct{}{}
	}

	switch payload.Type {
	case "patch":
		for _, file := range payload.Files {
			addFile(file)
		}
	case "tool":
		if payload.State.Status != "completed" {
			break
		}
		switch payload.Tool {
		case "write", "edit":
			addFile(payload.State.Input.FilePath)
		case "apply_patch":
			for _, file := range extractFilesFromPatchText(payload.State.Input.PatchText) {
				addFile(file)
			}
		}
	}

	if len(fileSet) == 0 {
		return nil
	}
	files := make([]string, 0, len(fileSet))
	for file := range fileSet {
		files = append(files, file)
	}
	return files
}

func extractFilesFromPatchText(patchText string) []string {
	if strings.TrimSpace(patchText) == "" {
		return nil
	}
	files := map[string]struct{}{}
	for _, line := range strings.Split(patchText, "\n") {
		switch {
		case strings.HasPrefix(line, "*** Add File: "):
			files[strings.TrimSpace(strings.TrimPrefix(line, "*** Add File: "))] = struct{}{}
		case strings.HasPrefix(line, "*** Update File: "):
			files[strings.TrimSpace(strings.TrimPrefix(line, "*** Update File: "))] = struct{}{}
		case strings.HasPrefix(line, "*** Delete File: "):
			files[strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File: "))] = struct{}{}
		}
	}
	if len(files) == 0 {
		return nil
	}
	result := make([]string, 0, len(files))
	for file := range files {
		result = append(result, file)
	}
	return result
}

func normalizeChangedFilePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(filepath.ToSlash(path))
	}
	return path
}

func hasSessionSummaryColumns(db *sql.DB) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(session)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	hasAdditions := false
	hasDeletions := false
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		switch name {
		case "summary_additions":
			hasAdditions = true
		case "summary_deletions":
			hasDeletions = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return hasAdditions && hasDeletions, nil
}

func buildReport(days []Day, now time.Time, options Options) Report {
	if days == nil {
		days = buildEmptyDays(now)
	}

	report := Report{Days: days, HighestBurnDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, AgentModelCounts: map[string]int{}, ModelCounts: map[string]int64{}, ModelCosts: map[string]float64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}, UniqueAgentModels: map[string]struct{}{}}, HighestCodeDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, AgentModelCounts: map[string]int{}, ModelCounts: map[string]int64{}, ModelCosts: map[string]float64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}, UniqueAgentModels: map[string]struct{}{}}, HighestChangedFilesDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, AgentModelCounts: map[string]int{}, ModelCounts: map[string]int64{}, ModelCosts: map[string]float64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}, UniqueAgentModels: map[string]struct{}{}}, LongestSessionDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, AgentModelCounts: map[string]int{}, ModelCounts: map[string]int64{}, ModelCosts: map[string]float64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}, UniqueAgentModels: map[string]struct{}{}}, MostEfficientDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, AgentModelCounts: map[string]int{}, ModelCounts: map[string]int64{}, ModelCosts: map[string]float64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}, UniqueAgentModels: map[string]struct{}{}}}
	allTools := map[string]struct{}{}
	allSkills := map[string]struct{}{}
	allAgents := map[string]struct{}{}
	allAgentModels := map[string]struct{}{}
	toolTotals := map[string]int{}
	skillTotals := map[string]int{}
	agentTotals := map[string]int{}
	agentModelTotals := map[string]int{}
	modelTotals := map[string]int64{}
	modelCostTotals := map[string]float64{}
	for i, day := range days {
		day.SessionMinutes = computeSessionMinutes(day.eventTimes, options.SessionGapMinutes)
		days[i] = day
		if isActiveDay(day) {
			report.ActiveDays++
			report.WeekdayActiveCounts[int(day.Date.Weekday())]++
		}
		if isAgentDay(day) {
			report.AgentDays++
			report.WeekdayAgentCounts[int(day.Date.Weekday())]++
		}
		report.TotalToolCalls += day.ToolCalls
		report.TotalSkillCalls += day.SkillCalls
		report.TotalSubtasks += day.Subtasks
		report.ThirtyDaySessionMinutes += day.SessionMinutes
		report.ThirtyDayCodeLines += day.CodeLines
		report.ThirtyDayChangedFiles += day.ChangedFiles
		for tool := range day.UniqueTools {
			allTools[tool] = struct{}{}
		}
		for tool, count := range day.ToolCounts {
			toolTotals[tool] += count
		}
		for skill := range day.UniqueSkills {
			allSkills[skill] = struct{}{}
		}
		for skill, count := range day.SkillCounts {
			skillTotals[skill] += count
		}
		for agent := range day.UniqueAgents {
			allAgents[agent] = struct{}{}
		}
		for agent, count := range day.AgentCounts {
			agentTotals[agent] += count
		}
		for agentModel := range day.UniqueAgentModels {
			allAgentModels[agentModel] = struct{}{}
		}
		for agentModel, count := range day.AgentModelCounts {
			agentModelTotals[agentModel] += count
			report.TotalAgentModelCalls += count
		}
		for name, amount := range day.ModelCounts {
			modelTotals[name] += amount
			report.TotalModelTokens += amount
		}
		for name, cost := range day.ModelCosts {
			modelCostTotals[name] += cost
			report.TotalModelCost += cost
		}
		report.ThirtyDayCost += day.Cost
		report.ThirtyDayTokens += day.Tokens
		if i >= len(days)-7 {
			if isActiveDay(day) {
				report.WeeklyActiveDays++
			}
			if isAgentDay(day) {
				report.WeeklyAgentDays++
			}
		}
		if day.Cost > report.HighestBurnDay.Cost {
			report.HighestBurnDay = day
		}
		if day.CodeLines > report.HighestCodeDay.CodeLines {
			report.HighestCodeDay = day
		}
		if day.ChangedFiles > report.HighestChangedFilesDay.ChangedFiles {
			report.HighestChangedFilesDay = day
		}
		if day.SessionMinutes > report.LongestSessionDay.SessionMinutes {
			report.LongestSessionDay = day
		}
		if isActiveDay(day) && (report.MostEfficientDay.Date.IsZero() || efficiencyScore(day) < efficiencyScore(report.MostEfficientDay)) {
			report.MostEfficientDay = day
		}
	}
	report.UniqueToolCount = len(allTools)
	report.UniqueSkillCount = len(allSkills)
	report.UniqueAgentCount = len(allAgents)
	report.UniqueAgentModelCount = len(allAgentModels)
	report.UniqueModelCount = len(modelTotals)
	report.TopTools = topUsageCounts(toolTotals, unlimitedUsageItems)
	report.TopSkills = topUsageCounts(skillTotals, unlimitedUsageItems)
	report.TopAgents = topUsageCounts(agentTotals, unlimitedUsageItems)
	report.TopAgentModels = topUsageCounts(agentModelTotals, unlimitedUsageItems)
	report.TopModels = topUsageAmountsWithCostsFromMaps(modelTotals, modelCostTotals, unlimitedUsageItems)
	report.CurrentStreak = currentStreak(days)
	report.BestStreak = bestStreak(days)
	report.CurrentHourlyStreakSlots = currentHourlyStreakSlots(days)
	report.BestHourlyStreakSlots = bestHourlyStreakSlots(days)
	if len(days) > 0 {
		today := days[len(days)-1]
		report.TodayCost = today.Cost
		report.TodayTokens = today.Tokens
		report.TodaySessionMinutes = today.SessionMinutes
		report.TodayCodeLines = today.CodeLines
		report.TodayChangedFiles = today.ChangedFiles
		report.TodayReasoningShare = reasoningShare(today)
	}
	if len(days) > 1 {
		yesterday := days[len(days)-2]
		report.YesterdayCost = yesterday.Cost
		report.YesterdayTokens = yesterday.Tokens
		report.YesterdaySessionMinutes = yesterday.SessionMinutes
		report.YesterdayCodeLines = yesterday.CodeLines
		report.YesterdayChangedFiles = yesterday.ChangedFiles
	}
	if len(days) > 1 {
		nowSlot := now.Hour()*2 + now.Minute()/30
		today := days[len(days)-1]
		yesterday := days[len(days)-2]
		for i := 0; i < 48; i++ {
			srcSlot := (nowSlot + 1 + i) % 48
			if srcSlot > nowSlot {
				report.Rolling24hSlots[i] = yesterday.SlotTokens[srcSlot]
			} else {
				report.Rolling24hSlots[i] = today.SlotTokens[srcSlot]
			}
		}
		var rollingEvents []int64
		cutoff := now.Add(-24 * time.Hour).UnixMilli()
		for _, evt := range today.eventTimes {
			if evt >= cutoff {
				rollingEvents = append(rollingEvents, evt)
			}
		}
		for _, evt := range yesterday.eventTimes {
			if evt >= cutoff {
				rollingEvents = append(rollingEvents, evt)
			}
		}
		report.Rolling24hSessionMinutes = computeSessionMinutes(rollingEvents, options.SessionGapMinutes)
	}
	report.RecentReasoningShare = recentReasoningShare(days)
	return report
}

func normalizeOptions(options Options) Options {
	if options.SessionGapMinutes <= 0 {
		options.SessionGapMinutes = defaultSessionGapMinutes
	}
	return options
}

func agentModelUsageKey(agent string, model string) string {
	return agent + "\x00" + model
}

func providerModelUsageKey(provider string, model string) string {
	return provider + "\x00" + model
}

func startOfDay(t time.Time) time.Time {
	local := t.Local()
	year, month, day := local.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, local.Location())
}

func startOfMonth(t time.Time) time.Time {
	local := t.Local()
	year, month, _ := local.Date()
	return time.Date(year, month, 1, 0, 0, 0, 0, local.Location())
}

func dayKey(t time.Time) string {
	return startOfDay(t).Format("2006-01-02")
}

func scopedDirectoryClause() string {
	if runtime.GOOS == "windows" {
		return "replace(lower(s.directory), '\\', '/') = replace(lower(?), '\\', '/')"
	}
	return "s.directory = ?"
}

func scopedDirectoryArg(dir string) string {
	return filepath.Clean(dir)
}
