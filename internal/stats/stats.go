package stats

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const dayWindow = 30

const defaultSessionGapMinutes = 15

const unlimitedUsageItems = 0

type Day struct {
	Date              time.Time
	AssistantMessages int
	ToolCalls         int
	SkillCalls        int
	StepFinishes      int
	Subtasks          int
	Cost              float64
	Tokens            int64
	ReasoningTokens   int64
	SessionMinutes    int
	CodeLines         int
	ToolCounts        map[string]int
	SkillCounts       map[string]int
	AgentCounts       map[string]int
	ModelCounts       map[string]int64
	eventTimes        []int64
	UniqueTools       map[string]struct{}
	UniqueSkills      map[string]struct{}
	UniqueAgents      map[string]struct{}
	SlotTokens        [48]int64
}

type UsageCount struct {
	Name   string
	Count  int
	Amount int64
}

type Options struct {
	SessionGapMinutes int
}

type Report struct {
	Days                     []Day
	ActiveDays               int
	AgentDays                int
	CurrentStreak            int
	BestStreak               int
	WeeklyActiveDays         int
	WeeklyAgentDays          int
	ThirtyDayCost            float64
	ThirtyDayTokens          int64
	ThirtyDaySessionMinutes  int
	ThirtyDayCodeLines       int
	TotalToolCalls           int
	TotalSkillCalls          int
	TotalSubtasks            int
	UniqueToolCount          int
	UniqueSkillCount         int
	UniqueAgentCount         int
	UniqueModelCount         int
	TodayCost                float64
	YesterdayCost            float64
	TodayTokens              int64
	YesterdayTokens          int64
	TotalModelTokens         int64
	TodaySessionMinutes      int
	YesterdaySessionMinutes  int
	TodayCodeLines           int
	YesterdayCodeLines       int
	TodayReasoningShare      float64
	RecentReasoningShare     float64
	WeekdayActiveCounts      [7]int
	WeekdayAgentCounts       [7]int
	TopTools                 []UsageCount
	TopSkills                []UsageCount
	TopAgents                []UsageCount
	TopModels                []UsageCount
	HighestBurnDay           Day
	HighestCodeDay           Day
	LongestSessionDay        Day
	MostEfficientDay         Day
	Rolling24hSlots          [48]int64
	Rolling24hSessionMinutes int
}

type WindowReport struct {
	Label       string
	Start       time.Time
	End         time.Time
	Messages    int
	Sessions    int
	Tokens      int64
	Cost        float64
	Models      []ModelUsage
	TopSessions []SessionUsage
}

type ModelUsage struct {
	Source           string
	Model            string
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	ReasoningTokens  int64
	TotalTokens      int64
	Cost             float64
}

type SessionUsage struct {
	ID       string
	Title    string
	Cost     float64
	Tokens   int64
	Messages int
}

type windowMessageRow struct {
	MessageID string
	SessionID string
	Title     string
	CreatedAt int64
	Role      string
	Cost      float64
	Summary   bool
	Agent     string
}

type windowPartRow struct {
	MessageID        string
	SessionID        string
	Title            string
	CreatedAt        int64
	Type             string
	ProviderID       string
	ModelID          string
	Cost             float64
	MessageCost      float64
	InputTokens      int64
	OutputTokens     int64
	ReasoningTokens  int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	Summary          bool
	MessageAgent     string
}

type messageEvent struct {
	CreatedAt int64
	Role      string
	Cost      float64
	Summary   bool
	Agent     string
}

type partEvent struct {
	CreatedAt        int64
	Type             string
	Tool             string
	SkillName        string
	Agent            string
	ProviderID       string
	ModelID          string
	Cost             float64
	MessageCost      float64
	InputTokens      int64
	OutputTokens     int64
	ReasoningTokens  int64
	CacheReadTokens  int64
	CacheWriteTokens int64
	Summary          bool
	MessageAgent     string
}

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

func loadAtWithOptions(now time.Time, dir string, options Options) (Report, error) {
	options = normalizeOptions(options)
	dbPath, err := opencodeDBPath()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return buildReport(nil, now, options), nil
		}
		return Report{}, err
	}

	db, err := sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return Report{}, err
	}
	defer db.Close()

	since := startOfDay(now).AddDate(0, 0, -(dayWindow - 1)).UnixMilli()
	dayMap := make(map[string]*Day, dayWindow)
	for _, day := range buildEmptyDays(now) {
		copyDay := day
		dayMap[dayKey(day.Date)] = &copyDay
	}

	if err := mergeMessageStats(db, dir, since, now.Location(), dayMap); err != nil {
		return Report{}, err
	}
	if err := mergePartStats(db, dir, since, now.Location(), dayMap); err != nil {
		return Report{}, err
	}
	if err := mergeSessionCodeStats(db, dir, since, now.Location(), dayMap); err != nil {
		return Report{}, err
	}

	days := make([]Day, 0, len(dayMap))
	for _, day := range dayMap {
		days = append(days, *day)
	}
	sort.Slice(days, func(i, j int) bool {
		return days[i].Date.Before(days[j].Date)
	})

	return buildReport(days, now, options), nil
}

func buildEmptyDays(now time.Time) []Day {
	start := startOfDay(now).AddDate(0, 0, -(dayWindow - 1))
	days := make([]Day, 0, dayWindow)
	for i := 0; i < dayWindow; i++ {
		days = append(days, Day{
			Date:         start.AddDate(0, 0, i),
			ToolCounts:   map[string]int{},
			SkillCounts:  map[string]int{},
			AgentCounts:  map[string]int{},
			ModelCounts:  map[string]int64{},
			eventTimes:   nil,
			UniqueTools:  map[string]struct{}{},
			UniqueSkills: map[string]struct{}{},
			UniqueAgents: map[string]struct{}{},
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
		day := dayMap[dayKey(unixTimestampToTime(event.CreatedAt).In(loc))]
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

	for rows.Next() {
		var event partEvent
		var summary int64
		var skillNameType string
		if err := rows.Scan(
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
		day := dayMap[dayKey(unixTimestampToTime(event.CreatedAt).In(loc))]
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
			if event.MessageCost <= 0 && event.Cost > 0 {
				day.Cost += event.Cost
			} else if event.MessageCost <= 0 {
				estimatedCost, err := estimatePartCost(event)
				if err != nil {
					return fmt.Errorf("estimate step-finish cost: %w", err)
				}
				day.Cost += estimatedCost
			}
			stepTokens := event.InputTokens + event.OutputTokens + event.ReasoningTokens + event.CacheReadTokens + event.CacheWriteTokens
			day.Tokens += stepTokens
			day.ReasoningTokens += event.ReasoningTokens
			st := unixTimestampToTime(event.CreatedAt).In(loc)
			day.SlotTokens[st.Hour()*2+st.Minute()/30] += stepTokens
			name := modelLabel(event.ProviderID, event.ModelID)
			if name != "" {
				day.ModelCounts[name] += stepTokens
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
		day := dayMap[dayKey(unixTimestampToTime(updatedAt).In(loc))]
		if day == nil {
			continue
		}
		day.CodeLines += additions + deletions
	}

	return rows.Err()
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

	report := Report{Days: days, HighestBurnDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, ModelCounts: map[string]int64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}}, HighestCodeDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, ModelCounts: map[string]int64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}}, LongestSessionDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, ModelCounts: map[string]int64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}}, MostEfficientDay: Day{ToolCounts: map[string]int{}, SkillCounts: map[string]int{}, AgentCounts: map[string]int{}, ModelCounts: map[string]int64{}, UniqueTools: map[string]struct{}{}, UniqueSkills: map[string]struct{}{}, UniqueAgents: map[string]struct{}{}}}
	allTools := map[string]struct{}{}
	allSkills := map[string]struct{}{}
	allAgents := map[string]struct{}{}
	toolTotals := map[string]int{}
	skillTotals := map[string]int{}
	agentTotals := map[string]int{}
	modelTotals := map[string]int64{}
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
		for name, amount := range day.ModelCounts {
			modelTotals[name] += amount
			report.TotalModelTokens += amount
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
	report.UniqueModelCount = len(modelTotals)
	report.TopTools = topUsageCounts(toolTotals, unlimitedUsageItems)
	report.TopSkills = topUsageCounts(skillTotals, unlimitedUsageItems)
	report.TopAgents = topUsageCounts(agentTotals, unlimitedUsageItems)
	report.TopModels = topUsageAmounts(modelTotals, unlimitedUsageItems)
	report.CurrentStreak = currentStreak(days)
	report.BestStreak = bestStreak(days)
	if len(days) > 0 {
		today := days[len(days)-1]
		report.TodayCost = today.Cost
		report.TodayTokens = today.Tokens
		report.TodaySessionMinutes = today.SessionMinutes
		report.TodayCodeLines = today.CodeLines
		report.TodayReasoningShare = reasoningShare(today)
	}
	if len(days) > 1 {
		yesterday := days[len(days)-2]
		report.YesterdayCost = yesterday.Cost
		report.YesterdayTokens = yesterday.Tokens
		report.YesterdaySessionMinutes = yesterday.SessionMinutes
		report.YesterdayCodeLines = yesterday.CodeLines
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

func computeSessionMinutes(eventTimes []int64, gapMinutes int) int {
	if len(eventTimes) < 2 {
		return 0
	}
	sorted := append([]int64(nil), eventTimes...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	gapMillis := int64(gapMinutes) * int64(time.Minute/time.Millisecond)
	start := sorted[0]
	prev := sorted[0]
	totalMillis := int64(0)
	for _, current := range sorted[1:] {
		if current-prev > gapMillis {
			totalMillis += prev - start
			start = current
		}
		prev = current
	}
	totalMillis += prev - start
	return int(totalMillis / int64(time.Minute/time.Millisecond))
}

func isActiveDay(day Day) bool {
	return day.AssistantMessages > 0 || day.ToolCalls > 0 || day.StepFinishes > 0
}

func isAgentDay(day Day) bool {
	return day.Subtasks >= 1
}

func isToolRichDay(day Day) bool {
	return day.ToolCalls >= 5
}

func isHighActivityDay(day Day) bool {
	return day.StepFinishes >= 3
}

func isAgentHeavyDay(day Day) bool {
	return day.Subtasks >= 2
}

func currentStreak(days []Day) int {
	end := len(days) - 1
	for end >= 0 && !isActiveDay(days[end]) {
		end--
	}
	if end < 0 {
		return 0
	}
	streak := 0
	for i := end; i >= 0 && isActiveDay(days[i]); i-- {
		streak++
	}
	return streak
}

func bestStreak(days []Day) int {
	best, current := 0, 0
	for _, day := range days {
		if isActiveDay(day) {
			current++
			if current > best {
				best = current
			}
			continue
		}
		current = 0
	}
	return best
}

func recentReasoningShare(days []Day) float64 {
	if len(days) <= 1 {
		return 0
	}
	start := len(days) - 8
	if start < 0 {
		start = 0
	}
	window := days[start : len(days)-1]
	var totalReasoning int64
	var totalTokens int64
	for _, day := range window {
		totalReasoning += day.ReasoningTokens
		totalTokens += day.Tokens
	}
	if totalTokens > 0 {
		return float64(totalReasoning) / float64(totalTokens)
	}
	return 0
}

func reasoningShare(day Day) float64 {
	if day.Tokens <= 0 {
		return 0
	}
	return float64(day.ReasoningTokens) / float64(day.Tokens)
}

func efficiencyScore(day Day) float64 {
	if day.Tokens <= 0 {
		return day.Cost
	}
	return day.Cost / float64(day.Tokens)
}

func topUsageCounts(counts map[string]int, limit int) []UsageCount {
	if len(counts) == 0 {
		return nil
	}
	items := make([]UsageCount, 0, len(counts))
	for name, count := range counts {
		if count <= 0 {
			continue
		}
		items = append(items, UsageCount{Name: name, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func topUsageAmounts(counts map[string]int64, limit int) []UsageCount {
	if len(counts) == 0 {
		return nil
	}
	items := make([]UsageCount, 0, len(counts))
	for name, amount := range counts {
		if amount <= 0 {
			continue
		}
		items = append(items, UsageCount{Name: name, Amount: amount})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Amount == items[j].Amount {
			return items[i].Name < items[j].Name
		}
		return items[i].Amount > items[j].Amount
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func providerLabel(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai":
		return "OpenAI"
	case "github", "copilot", "github_models":
		return "Copilot"
	case "anthropic":
		return "Anthropic"
	case "google", "gemini":
		return "Google"
	case "openrouter":
		return "OpenRouter"
	case "azure":
		return "Azure"
	case "vertex_ai":
		return "Vertex"
	case "bedrock":
		return "Bedrock"
	default:
		if provider == "" {
			return ""
		}
		return strings.ToUpper(provider[:1]) + provider[1:]
	}
}

func modelLabel(provider string, model string) string {
	provider = providerLabel(provider)
	model = strings.TrimSpace(model)
	if provider == "" || model == "" {
		return ""
	}
	return fmt.Sprintf("[%s] %s", provider, model)
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

func unixTimestampToTime(value int64) time.Time {
	switch {
	case value >= 1_000_000_000_000_000_000 || value <= -1_000_000_000_000_000_000:
		return time.Unix(0, value).Local()
	case value >= 1_000_000_000_000_000 || value <= -1_000_000_000_000_000:
		return time.UnixMicro(value).Local()
	case value >= 1_000_000_000_000 || value <= -1_000_000_000_000:
		return time.UnixMilli(value).Local()
	default:
		return time.Unix(value, 0).Local()
	}
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

func sqliteDSN(path string) string {
	path = filepath.ToSlash(path)
	if runtime.GOOS == "windows" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "file:" + path + "?mode=ro&_pragma=busy_timeout(5000)"
}

func opencodeDBPath() (string, error) {
	if name := os.Getenv("OPENCODE_DB"); name != "" {
		if filepath.IsAbs(name) {
			if _, err := os.Stat(name); err == nil {
				return name, nil
			} else {
				return "", err
			}
		}

		root, err := opencodeDataDir()
		if err != nil {
			return "", err
		}
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else {
			return "", err
		}
	}

	root, err := opencodeDataDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, "opencode.db")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	paths, err := filepath.Glob(filepath.Join(root, "opencode-*.db"))
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", os.ErrNotExist
	}
	sort.SliceStable(paths, func(i, j int) bool {
		left, leftErr := os.Stat(paths[i])
		right, rightErr := os.Stat(paths[j])
		if leftErr != nil || rightErr != nil {
			return paths[i] < paths[j]
		}
		return left.ModTime().After(right.ModTime())
	})
	return paths[0], nil
}

func opencodeDataDir() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "opencode"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "opencode"), nil
}
