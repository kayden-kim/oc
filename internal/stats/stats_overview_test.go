package stats

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"
)

func TestNewEmptyDay_InitializesReportMaps(t *testing.T) {
	date := time.Date(2026, time.March, 27, 0, 0, 0, 0, time.Local)

	day := newEmptyDay(date)

	if !day.Date.Equal(date) {
		t.Fatalf("expected date %v, got %v", date, day.Date)
	}
	if day.ToolCounts == nil || day.SkillCounts == nil || day.AgentCounts == nil || day.AgentModelCounts == nil {
		t.Fatal("expected usage count maps to be initialized")
	}
	if day.ModelCounts == nil || day.ModelCosts == nil {
		t.Fatal("expected model maps to be initialized")
	}
	if day.UniqueTools == nil || day.UniqueSkills == nil || day.UniqueAgents == nil || day.UniqueAgentModels == nil {
		t.Fatal("expected unique tracking maps to be initialized")
	}
	if day.AssistantMessages != 0 || day.ToolCalls != 0 || day.Tokens != 0 || day.Cost != 0 {
		t.Fatalf("expected zero-value metrics, got %+v", day)
	}
}

func TestNewEmptyReport_InitializesHighlightDays(t *testing.T) {
	report := newEmptyReport(nil)

	if report.Days != nil {
		t.Fatalf("expected nil days, got %v", report.Days)
	}
	highlights := []Day{
		report.HighestBurnDay,
		report.HighestCodeDay,
		report.HighestChangedFilesDay,
		report.LongestSessionDay,
		report.MostEfficientDay,
	}
	for i, day := range highlights {
		if day.ToolCounts == nil || day.SkillCounts == nil || day.AgentCounts == nil || day.AgentModelCounts == nil {
			t.Fatalf("expected initialized count maps for highlight %d", i)
		}
		if day.ModelCounts == nil || day.ModelCosts == nil {
			t.Fatalf("expected initialized model maps for highlight %d", i)
		}
		if day.UniqueTools == nil || day.UniqueSkills == nil || day.UniqueAgents == nil || day.UniqueAgentModels == nil {
			t.Fatalf("expected initialized unique maps for highlight %d", i)
		}
	}
}

func TestLoadForDirAt_AggregatesGlobalStatsAndFiltersSynthetic(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	otherDir := filepath.Join(tmp, "other")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertSession(t, db, "ses_other", otherDir)

	insertMessage(t, db, "msg_tool", "ses_work", now.AddDate(0, 0, -2), `{"role":"assistant","cost":0.50}`)
	for i := 0; i < 6; i++ {
		insertPart(t, db, fmt.Sprintf("tool_%d", i), "msg_tool", "ses_work", now.AddDate(0, 0, -2), `{"type":"tool","tool":"bash"}`)
	}

	insertMessage(t, db, "msg_yesterday", "ses_work", now.AddDate(0, 0, -1), `{"role":"assistant","cost":1.50}`)
	insertPart(t, db, "step_yesterday", "msg_yesterday", "ses_work", now.AddDate(0, 0, -1), `{"type":"step-finish","tokens":{"input":40,"output":40,"reasoning":5}}`)

	insertMessage(t, db, "msg_today", "ses_work", now, `{"role":"assistant","cost":1.84}`)
	insertPart(t, db, "step_today", "msg_today", "ses_work", now, `{"type":"step-finish","tokens":{"input":50,"output":50,"reasoning":25}}`)

	insertMessage(t, db, "msg_synthetic", "ses_work", now, `{"role":"assistant","cost":99,"summary":true,"agent":"compaction"}`)
	insertPart(t, db, "part_synthetic", "msg_synthetic", "ses_work", now, `{"type":"compaction"}`)

	insertMessage(t, db, "msg_other", "ses_other", now, `{"role":"assistant","cost":42}`)
	insertPart(t, db, "part_other", "msg_other", "ses_other", now, `{"type":"step-finish","tokens":{"input":100,"output":100,"reasoning":100}}`)

	report, err := loadGlobalAt(now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if report.ActiveDays != 3 {
		t.Fatalf("expected 3 active days, got %d", report.ActiveDays)
	}
	if report.AgentDays != 0 {
		t.Fatalf("expected 0 agent days, got %d", report.AgentDays)
	}
	if report.CurrentStreak != 3 {
		t.Fatalf("expected current streak 3, got %d", report.CurrentStreak)
	}
	if report.CurrentHourlyStreakSlots != 1 {
		t.Fatalf("expected current hourly streak 1 slot, got %d", report.CurrentHourlyStreakSlots)
	}
	if report.BestHourlyStreakSlots != 1 {
		t.Fatalf("expected best hourly streak 1 slot, got %d", report.BestHourlyStreakSlots)
	}
	if report.TotalToolCalls != 6 {
		t.Fatalf("expected 6 tool calls, got %d", report.TotalToolCalls)
	}
	if report.UniqueToolCount != 1 {
		t.Fatalf("expected 1 unique tool, got %d", report.UniqueToolCount)
	}
	if report.TodayCost != 43.84 {
		t.Fatalf("expected today cost 43.84, got %.2f", report.TodayCost)
	}
	if report.ThirtyDayCost != 45.84 {
		t.Fatalf("expected 30-day cost 45.84, got %.2f", report.ThirtyDayCost)
	}
	if report.YesterdayCost != 1.50 {
		t.Fatalf("expected yesterday cost 1.50, got %.2f", report.YesterdayCost)
	}
	if report.TodayTokens != 425 {
		t.Fatalf("expected today tokens 425, got %d", report.TodayTokens)
	}
	if report.ThirtyDayTokens != 510 {
		t.Fatalf("expected 30-day tokens 510, got %d", report.ThirtyDayTokens)
	}
	if report.YesterdayTokens != 85 {
		t.Fatalf("expected yesterday tokens 85, got %d", report.YesterdayTokens)
	}
}

func TestLoadForDirAt_Uses30DaySumsForWeeklyTotals(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)

	insertMessage(t, testSession.db, "msg_old", "ses_work", testSession.now.AddDate(0, 0, -10), `{"role":"assistant","cost":2.00}`)
	insertPart(t, testSession.db, "step_old", "msg_old", "ses_work", testSession.now.AddDate(0, 0, -10), `{"type":"step-finish","tokens":{"input":20,"output":30,"reasoning":10}}`)

	insertMessage(t, testSession.db, "msg_today", "ses_work", testSession.now, `{"role":"assistant","cost":3.00}`)
	insertPart(t, testSession.db, "step_today", "msg_today", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":40,"output":30,"reasoning":20}}`)

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if report.ThirtyDayCost != 5.00 {
		t.Fatalf("expected 30-day cost sum 5.00, got %.2f", report.ThirtyDayCost)
	}
	if report.ThirtyDayTokens != 150 {
		t.Fatalf("expected 30-day token sum 150, got %d", report.ThirtyDayTokens)
	}
}

func TestLoadForDirAt_CountsDelegatedAgentMessages(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)

	insertMessage(t, testSession.db, "msg_main", "ses_work", testSession.now, `{"role":"assistant","cost":1.00}`)
	insertPart(t, testSession.db, "main_step", "msg_main", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":10,"output":10,"reasoning":0}}`)

	insertMessage(t, testSession.db, "msg_explore", "ses_work", testSession.now, `{"role":"assistant","agent":"explore","cost":0.20}`)
	insertPart(t, testSession.db, "explore_start", "msg_explore", "ses_work", testSession.now, `{"type":"step-start"}`)
	insertPart(t, testSession.db, "explore_tool", "msg_explore", "ses_work", testSession.now, `{"type":"tool","tool":"read"}`)
	insertPart(t, testSession.db, "explore_finish", "msg_explore", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":5,"output":5,"reasoning":0}}`)

	insertMessage(t, testSession.db, "msg_librarian", "ses_work", testSession.now, `{"role":"assistant","agent":"librarian","cost":0.30}`)
	insertPart(t, testSession.db, "librarian_finish", "msg_librarian", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":6,"output":4,"reasoning":0}}`)

	insertMessage(t, testSession.db, "msg_user_agent", "ses_work", testSession.now, `{"role":"user","agent":"explore"}`)
	insertPart(t, testSession.db, "user_agent_finish", "msg_user_agent", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":1,"output":1,"reasoning":0}}`)

	insertMessage(t, testSession.db, "msg_compaction", "ses_work", testSession.now, `{"role":"assistant","agent":"compaction","cost":5.00}`)
	insertPart(t, testSession.db, "compaction_part", "msg_compaction", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":50,"output":50,"reasoning":50}}`)

	insertMessage(t, testSession.db, "msg_part_subtask", "ses_work", testSession.now, `{"role":"assistant","cost":0.10}`)
	insertPart(t, testSession.db, "part_subtask", "msg_part_subtask", "ses_work", testSession.now, `{"type":"subtask","agent":"legacy-subtask"}`)

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if report.TotalSubtasks != 2 {
		t.Fatalf("expected 2 delegated agent messages, got %d", report.TotalSubtasks)
	}
	if report.UniqueAgentCount != 2 {
		t.Fatalf("expected 2 unique delegated agents, got %d", report.UniqueAgentCount)
	}
	if report.AgentDays != 1 {
		t.Fatalf("expected 1 delegated agent day, got %d", report.AgentDays)
	}
	if report.TotalToolCalls != 1 {
		t.Fatalf("expected delegated tool call to still count, got %d", report.TotalToolCalls)
	}
}

func TestLoadForDirAt_BuildsTopToolUsage(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)
	insertMessage(t, testSession.db, "msg_tools", "ses_work", testSession.now, `{"role":"assistant","cost":1.00}`)

	toolCounts := map[string]int{"bash": 5, "read": 4, "edit": 3, "grep": 3, "write": 2, "glob": 1}
	partID := 0
	for tool, count := range toolCounts {
		for range count {
			insertPart(t, testSession.db, fmt.Sprintf("tool_%d", partID), "msg_tools", "ses_work", testSession.now, fmt.Sprintf(`{"type":"tool","tool":%q}`, tool))
			partID++
		}
	}

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	topTools := rankedUsageFromReportField(t, report, "TopTools")
	if len(topTools) != 6 {
		t.Fatalf("expected 6 top tools, got %d (%v)", len(topTools), topTools)
	}
	expected := []usageSnapshot{{"bash", 5}, {"read", 4}, {"edit", 3}, {"grep", 3}, {"write", 2}, {"glob", 1}}
	if !reflect.DeepEqual(topTools, expected) {
		t.Fatalf("expected top tools %v, got %v", expected, topTools)
	}
}

func TestLoadForDirAt_BuildsTopSkillUsage(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)
	insertMessage(t, testSession.db, "msg_skills", "ses_work", testSession.now, `{"role":"assistant","cost":1.00}`)

	rows := []string{
		`{"type":"tool","tool":"skill","state":{"input":{"name":"writing-plans"}}}`,
		`{"type":"tool","tool":"skill","state":{"input":{"name":"writing-plans"}}}`,
		`{"type":"tool","tool":"skill","state":{"input":{"name":"test-driven-development"}}}`,
		`{"type":"tool","tool":"skill","state":{"input":{"name":""}}}`,
		`{"type":"tool","tool":"skill","state":{"input":{}}}`,
		`{"type":"tool","tool":"skill","state":{"input":{"name":123}}}`,
		`{"type":"tool","tool":"bash"}`,
	}
	for i, data := range rows {
		insertPart(t, testSession.db, fmt.Sprintf("skill_%d", i), "msg_skills", "ses_work", testSession.now, data)
	}

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if report.TotalSkillCalls != 6 {
		t.Fatalf("expected 6 skill calls, got %d", report.TotalSkillCalls)
	}
	if report.UniqueSkillCount != 2 {
		t.Fatalf("expected 2 unique skills, got %d", report.UniqueSkillCount)
	}
	topSkills := rankedUsageFromReportField(t, report, "TopSkills")
	expected := []usageSnapshot{{"writing-plans", 2}, {"test-driven-development", 1}}
	if !reflect.DeepEqual(topSkills, expected) {
		t.Fatalf("expected top skills %v, got %v", expected, topSkills)
	}
}

func TestLoadForDirAt_BuildsTopAgentUsage(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)
	agents := []string{"explore", "explore", "explore", "oracle", "oracle", "planner", "planner", "review", "debug", "compaction"}
	for i, agent := range agents {
		insertMessage(t, testSession.db, fmt.Sprintf("msg_agent_%d", i), "ses_work", testSession.now, fmt.Sprintf(`{"role":"assistant","agent":%q}`, agent))
	}
	insertMessage(t, testSession.db, "msg_user_agent", "ses_work", testSession.now, `{"role":"user","agent":"explore"}`)
	insertMessage(t, testSession.db, "msg_plain", "ses_work", testSession.now, `{"role":"assistant"}`)
	insertMessage(t, testSession.db, "msg_legacy_subtask", "ses_work", testSession.now, `{"role":"assistant"}`)
	insertPart(t, testSession.db, "part_legacy_subtask", "msg_legacy_subtask", "ses_work", testSession.now, `{"type":"subtask","agent":"legacy-subtask"}`)

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	topAgents := rankedUsageFromReportField(t, report, "TopAgents")
	expected := []usageSnapshot{{"explore", 3}, {"oracle", 2}, {"planner", 2}, {"debug", 1}, {"review", 1}}
	if !reflect.DeepEqual(topAgents, expected) {
		t.Fatalf("expected top agents %v, got %v", expected, topAgents)
	}
}

func TestLoadForDirAt_BuildsTopAgentModelUsage(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)

	entries := []struct {
		messageID string
		agent     string
		provider  string
		model     string
	}{
		{"msg_explore_1", "explore", "openai", "gpt-5.4"},
		{"msg_explore_2", "explore", "openai", "gpt-5.4"},
		{"msg_explore_3", "explore", "anthropic", "claude-sonnet-4.5"},
		{"msg_oracle_1", "oracle", "openai", "gpt-5.4"},
		{"msg_oracle_2", "oracle", "google", "gemini-2.5-pro"},
	}
	for i, entry := range entries {
		insertMessage(t, testSession.db, entry.messageID, "ses_work", testSession.now, fmt.Sprintf(`{"role":"assistant","agent":%q,"providerID":%q,"modelID":%q}`, entry.agent, entry.provider, entry.model))
		insertPart(t, testSession.db, fmt.Sprintf("part_%d", i), entry.messageID, "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":5,"output":5}}`)
	}

	insertMessage(t, testSession.db, "msg_no_agent", "ses_work", testSession.now, `{"role":"assistant","providerID":"openai","modelID":"gpt-5.4"}`)
	insertPart(t, testSession.db, "part_no_agent", "msg_no_agent", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":5,"output":5}}`)

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	topAgentModels := rankedUsageFromReportField(t, report, "TopAgentModels")
	expected := []usageSnapshot{{"explore\x00gpt-5.4", 2}, {"explore\x00claude-sonnet-4.5", 1}, {"oracle\x00gemini-2.5-pro", 1}, {"oracle\x00gpt-5.4", 1}}
	if !reflect.DeepEqual(topAgentModels, expected) {
		t.Fatalf("expected top agent models %v, got %v", expected, topAgentModels)
	}
	if report.UniqueAgentModelCount != 4 {
		t.Fatalf("expected 4 unique agent/model pairs, got %d", report.UniqueAgentModelCount)
	}
	if report.TotalAgentModelCalls != 5 {
		t.Fatalf("expected 5 agent/model calls, got %d", report.TotalAgentModelCalls)
	}
}

func TestLoadForDirAt_BuildsTopModelUsage(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)

	usage := []struct {
		provider string
		model    string
		input    int64
		output   int64
		reason   int64
		cacheR   int64
		cacheW   int64
	}{
		{"openai", "gpt-5.4", 90, 20, 10, 500, 500},
		{"anthropic", "claude-sonnet-4.5", 80, 15, 5, 400, 400},
		{"google", "gemini-2.5-pro", 70, 15, 5, 300, 300},
		{"openrouter", "qwen/qwen3-coder", 60, 10, 5, 200, 200},
		{"azure", "gpt-4.1", 50, 10, 5, 100, 100},
		{"bedrock", "claude-3.7-sonnet", 40, 10, 5, 90, 90},
		{"vertex_ai", "gemini-2.0-flash", 35, 10, 5, 80, 80},
		{"copilot", "gpt-4o", 30, 10, 5, 70, 70},
		{"github_models", "mistral-large", 25, 10, 5, 60, 60},
		{"openai", "o4-mini", 20, 10, 5, 50, 50},
		{"anthropic", "claude-haiku-4.5", 15, 10, 5, 40, 40},
		{"google", "gemini-2.0-flash-lite", 10, 10, 5, 30, 30},
	}

	for i, item := range usage {
		msgID := fmt.Sprintf("msg_model_%d", i)
		partID := fmt.Sprintf("part_model_%d", i)
		insertMessage(t, testSession.db, msgID, "ses_work", testSession.now, fmt.Sprintf(`{"role":"assistant","providerID":%q,"modelID":%q,"cost":1.00}`, item.provider, item.model))
		insertPart(t, testSession.db, partID, msgID, "ses_work", testSession.now, fmt.Sprintf(`{"type":"step-finish","tokens":{"input":%d,"output":%d,"reasoning":%d,"cache":{"read":%d,"write":%d}}}`,
			item.input,
			item.output,
			item.reason,
			item.cacheR,
			item.cacheW,
		))
	}

	insertMessage(t, testSession.db, "msg_missing_provider", "ses_work", testSession.now, `{"role":"assistant","modelID":"skip-me"}`)
	insertPart(t, testSession.db, "part_missing_provider", "msg_missing_provider", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":999,"output":999,"reasoning":999}}`)
	insertMessage(t, testSession.db, "msg_missing_model", "ses_work", testSession.now, `{"role":"assistant","providerID":"openai"}`)
	insertPart(t, testSession.db, "part_missing_model", "msg_missing_model", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":999,"output":999,"reasoning":999}}`)

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if got := intFieldFromReport(t, report, "TotalModelTokens"); got != 4570 {
		t.Fatalf("expected 4570 total model tokens, got %d", got)
	}
	if math.Abs(report.TotalModelCost-12.0) > 1e-9 {
		t.Fatalf("expected 12.0 total model cost, got %.4f", report.TotalModelCost)
	}
	if got := intFieldFromReport(t, report, "UniqueModelCount"); got != 12 {
		t.Fatalf("expected 12 unique models, got %d", got)
	}

	topModels := rankedUsageMetricsFromReportField(t, report, "TopModels")
	if len(topModels) != 12 {
		t.Fatalf("expected all 12 ranked models, got %d (%v)", len(topModels), topModels)
	}
	expected := []usageMetric{
		{Name: "openai\x00gpt-5.4", Amount: 1120},
		{Name: "anthropic\x00claude-sonnet-4.5", Amount: 900},
		{Name: "google\x00gemini-2.5-pro", Amount: 690},
		{Name: "openrouter\x00qwen/qwen3-coder", Amount: 475},
		{Name: "azure\x00gpt-4.1", Amount: 265},
		{Name: "bedrock\x00claude-3.7-sonnet", Amount: 235},
		{Name: "vertex_ai\x00gemini-2.0-flash", Amount: 210},
		{Name: "copilot\x00gpt-4o", Amount: 185},
		{Name: "github_models\x00mistral-large", Amount: 160},
		{Name: "openai\x00o4-mini", Amount: 135},
		{Name: "anthropic\x00claude-haiku-4.5", Amount: 110},
		{Name: "google\x00gemini-2.0-flash-lite", Amount: 85},
	}
	if !reflect.DeepEqual(topModels, expected) {
		t.Fatalf("expected top models %v, got %v", expected, topModels)
	}
	if math.Abs(report.TopModels[0].Cost-1.0) > 1e-9 {
		t.Fatalf("expected top model cost 1.0, got %.4f", report.TopModels[0].Cost)
	}
	if math.Abs(report.TopModels[len(report.TopModels)-1].Cost-1.0) > 1e-9 {
		t.Fatalf("expected trailing model cost 1.0, got %.4f", report.TopModels[len(report.TopModels)-1].Cost)
	}
}

func TestLoadForDirAt_DoesNotDoubleCountModelCostWhenMessageCostExistsAcrossMultipleSteps(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)
	insertMessage(t, testSession.db, "msg_work", "ses_work", testSession.now, `{"role":"assistant","providerID":"openai","modelID":"gpt-5.4","cost":1.84}`)
	insertPart(t, testSession.db, "part_work_1", "msg_work", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":50,"output":25,"reasoning":5}}`)
	insertPart(t, testSession.db, "part_work_2", "msg_work", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":20,"output":10,"reasoning":0}}`)

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if math.Abs(report.TotalModelCost-1.84) > 1e-9 {
		t.Fatalf("expected total model cost 1.84, got %.4f", report.TotalModelCost)
	}
	if len(report.TopModels) != 1 {
		t.Fatalf("expected one top model, got %v", report.TopModels)
	}
	if math.Abs(report.TopModels[0].Cost-1.84) > 1e-9 {
		t.Fatalf("expected model cost 1.84, got %.4f", report.TopModels[0].Cost)
	}
	if report.TopModels[0].Amount != 110 {
		t.Fatalf("expected model tokens 110, got %d", report.TopModels[0].Amount)
	}
}

func TestLoadForDirAt_CountsMessageCostInDailyTotalsOnceAcrossMultipleSteps(t *testing.T) {
	testSession := setupStatsTestWorkSession(t)
	insertMessage(t, testSession.db, "msg_work", "ses_work", testSession.now, `{"role":"assistant","providerID":"openai","modelID":"gpt-5.4","cost":1.84}`)
	insertPart(t, testSession.db, "part_work_1", "msg_work", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":50,"output":25,"reasoning":5}}`)
	insertPart(t, testSession.db, "part_work_2", "msg_work", "ses_work", testSession.now, `{"type":"step-finish","tokens":{"input":20,"output":10,"reasoning":0}}`)

	report, err := loadForDirAt(testSession.dir, testSession.now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if math.Abs(report.TodayCost-1.84) > 1e-9 {
		t.Fatalf("expected today cost 1.84, got %.4f", report.TodayCost)
	}
	if math.Abs(report.ThirtyDayCost-1.84) > 1e-9 {
		t.Fatalf("expected 30-day cost 1.84, got %.4f", report.ThirtyDayCost)
	}
}

func TestLoadGlobalAt_BuildsTopProjectUsage(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	projectA := filepath.Join(tmp, "work-a")
	projectB := filepath.Join(tmp, "work-b")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_a1", projectA)
	insertSession(t, db, "ses_a2", projectA)
	insertSession(t, db, "ses_b1", projectB)

	insertMessage(t, db, "msg_a1", "ses_a1", now, `{"role":"assistant","providerID":"openai","modelID":"gpt-5.4"}`)
	insertPart(t, db, "part_a1", "msg_a1", "ses_a1", now, `{"type":"step-finish","cost":1.25,"tokens":{"input":100,"output":50,"reasoning":25}}`)
	insertMessage(t, db, "msg_a2", "ses_a2", now, `{"role":"assistant","providerID":"anthropic","modelID":"claude-sonnet-4.5"}`)
	insertPart(t, db, "part_a2", "msg_a2", "ses_a2", now, `{"type":"step-finish","cost":0.75,"tokens":{"input":80,"output":20,"reasoning":0}}`)
	insertMessage(t, db, "msg_b1", "ses_b1", now, `{"role":"assistant","providerID":"google","modelID":"gemini-2.5-pro"}`)
	insertPart(t, db, "part_b1", "msg_b1", "ses_b1", now, `{"type":"step-finish","cost":0.50,"tokens":{"input":70,"output":20,"reasoning":10}}`)

	insertMessage(t, db, "msg_compaction", "ses_b1", now, `{"role":"assistant","summary":true,"agent":"compaction"}`)
	insertPart(t, db, "part_compaction", "msg_compaction", "ses_b1", now, `{"type":"step-finish","tokens":{"input":999,"output":999,"reasoning":999}}`)

	report, err := loadGlobalAt(now)
	if err != nil {
		t.Fatalf("loadGlobalAt returned error: %v", err)
	}

	if report.UniqueProjectCount != 2 {
		t.Fatalf("expected 2 unique projects, got %d", report.UniqueProjectCount)
	}
	topProjects := rankedUsageMetricsFromReportField(t, report, "TopProjects")
	expected := []usageMetric{{Name: normalizeProjectUsageKey(projectA), Amount: 275}, {Name: normalizeProjectUsageKey(projectB), Amount: 100}}
	if !reflect.DeepEqual(topProjects, expected) {
		t.Fatalf("expected top projects %v, got %v", expected, topProjects)
	}
	if math.Abs(report.TotalProjectCost-2.50) > 1e-9 {
		t.Fatalf("expected 2.50 total project cost, got %.4f", report.TotalProjectCost)
	}
	if math.Abs(report.TopProjects[0].Cost-2.0) > 1e-9 {
		t.Fatalf("expected project A cost 2.0, got %.4f", report.TopProjects[0].Cost)
	}
	if math.Abs(report.TopProjects[1].Cost-0.5) > 1e-9 {
		t.Fatalf("expected project B cost 0.5, got %.4f", report.TopProjects[1].Cost)
	}
}

func TestTopUsageAmountsWithCostsFromMaps_IncludesCostOnlyEntries(t *testing.T) {
	items := topUsageAmountsWithCostsFromMaps(
		map[string]int64{},
		map[string]float64{"openai\x00gpt-5.4": 1.25},
		0,
	)

	if len(items) != 1 {
		t.Fatalf("expected one item, got %v", items)
	}
	if items[0].Name != "openai\x00gpt-5.4" {
		t.Fatalf("expected cost-only entry name, got %q", items[0].Name)
	}
	if items[0].Amount != 0 {
		t.Fatalf("expected zero tokens for cost-only entry, got %d", items[0].Amount)
	}
	if math.Abs(items[0].Cost-1.25) > 1e-9 {
		t.Fatalf("expected cost 1.25, got %.4f", items[0].Cost)
	}
}

func TestLoadGlobalAt_EstimatesProjectCostWhenStoredCostMissing(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	projectDir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", projectDir)
	insertMessage(t, db, "msg_work", "ses_work", now, `{"role":"assistant","providerID":"openai","modelID":"gpt-4o-mini"}`)
	insertPart(t, db, "step_work", "msg_work", "ses_work", now, `{"type":"step-finish","tokens":{"input":1000,"output":500,"reasoning":0,"cache":{"read":200,"write":100}}}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"gpt-4o-mini":{"input_cost_per_token":0.000001,"output_cost_per_token":0.000002,"cache_creation_input_token_cost":0.000003,"cache_read_input_token_cost":0.0000005}}`)
	}))
	defer server.Close()

	previousResolver := defaultPricingResolver
	defaultPricingResolver = newLiteLLMPricingResolver(server.URL, server.Client())
	t.Cleanup(func() {
		defaultPricingResolver = previousResolver
	})

	report, err := loadGlobalAt(now)
	if err != nil {
		t.Fatalf("loadGlobalAt returned error: %v", err)
	}

	const expected = 0.0024
	if math.Abs(report.TotalProjectCost-expected) > 1e-9 {
		t.Fatalf("expected total project cost %.4f, got %.4f", expected, report.TotalProjectCost)
	}
	if len(report.TopProjects) != 1 {
		t.Fatalf("expected one top project, got %v", report.TopProjects)
	}
	if math.Abs(report.TopProjects[0].Cost-expected) > 1e-9 {
		t.Fatalf("expected top project cost %.4f, got %.4f", expected, report.TopProjects[0].Cost)
	}
}

func TestLoadForDirAt_DoesNotAggregateTopProjectsInProjectScope(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertMessage(t, db, "msg_work", "ses_work", now, `{"role":"assistant"}`)
	insertPart(t, db, "part_work", "msg_work", "ses_work", now, `{"type":"step-finish","tokens":{"input":10,"output":10,"reasoning":5}}`)

	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if report.UniqueProjectCount != 0 {
		t.Fatalf("expected no project aggregation in project scope, got %d", report.UniqueProjectCount)
	}
	if len(report.TopProjects) != 0 {
		t.Fatalf("expected no top projects in project scope, got %v", report.TopProjects)
	}
}

func TestNormalizeProjectUsageKey(t *testing.T) {
	if got := normalizeProjectUsageKey("   "); got != "(unknown project)" {
		t.Fatalf("expected unknown project label, got %q", got)
	}

	if runtime.GOOS == "windows" {
		if got := normalizeProjectUsageKey(`C:/Work/App`); got != "c:/work/app" {
			t.Fatalf("expected normalized windows project key, got %q", got)
		}
		if got := normalizeProjectUsageKey(`C:\\Work\\App`); got != "c:/work/app" {
			t.Fatalf("expected slash-normalized windows project key, got %q", got)
		}
		return
	}

	if got := normalizeProjectUsageKey("/tmp/work/app/"); got != filepath.Clean("/tmp/work/app/") {
		t.Fatalf("expected cleaned project key, got %q", got)
	}
}

func TestTopUsageCounts_AllowsUnlimitedWhenLimitIsZero(t *testing.T) {
	items := topUsageCounts(map[string]int{"c": 1, "a": 3, "b": 2}, 0)
	expected := []UsageCount{{Name: "a", Count: 3}, {Name: "b", Count: 2}, {Name: "c", Count: 1}}
	if !reflect.DeepEqual(items, expected) {
		t.Fatalf("expected unlimited ranked usage %v, got %v", expected, items)
	}
}

func TestTopUsageAmounts_AllowsUnlimitedWhenLimitIsZero(t *testing.T) {
	items := topUsageAmounts(map[string]int64{"c": 10, "a": 30, "b": 20}, 0)
	expected := []UsageCount{{Name: "a", Amount: 30}, {Name: "b", Amount: 20}, {Name: "c", Amount: 10}}
	if !reflect.DeepEqual(items, expected) {
		t.Fatalf("expected unlimited ranked usage amounts %v, got %v", expected, items)
	}
}

func TestLoadForDirAt_ComputesSessionizedHoursFromEventGaps(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 18, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_day1_a", "ses_work", time.Date(2026, time.March, 26, 9, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "day1_part_a", "msg_day1_a", "ses_work", time.Date(2026, time.March, 26, 9, 10, 0, 0, time.Local), `{"type":"tool","tool":"read"}`)
	insertPart(t, db, "day1_part_b", "msg_day1_a", "ses_work", time.Date(2026, time.March, 26, 9, 20, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":1,"output":1,"reasoning":0}}`)
	insertMessage(t, db, "msg_day1_b", "ses_work", time.Date(2026, time.March, 26, 9, 45, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "day1_part_c", "msg_day1_b", "ses_work", time.Date(2026, time.March, 26, 9, 55, 0, 0, time.Local), `{"type":"tool","tool":"bash"}`)

	insertMessage(t, db, "msg_today_a", "ses_work", time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "today_part_a", "msg_today_a", "ses_work", time.Date(2026, time.March, 27, 10, 5, 0, 0, time.Local), `{"type":"tool","tool":"read"}`)
	insertPart(t, db, "today_part_b", "msg_today_a", "ses_work", time.Date(2026, time.March, 27, 10, 10, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":1,"output":1,"reasoning":0}}`)
	insertMessage(t, db, "msg_today_b", "ses_work", time.Date(2026, time.March, 27, 10, 40, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "today_part_c", "msg_today_b", "ses_work", time.Date(2026, time.March, 27, 10, 50, 0, 0, time.Local), `{"type":"tool","tool":"bash"}`)

	report, err := loadForDirAtWithOptions(dir, now, Options{SessionGapMinutes: 15})
	if err != nil {
		t.Fatalf("loadForDirAtWithOptions returned error: %v", err)
	}

	if report.YesterdaySessionMinutes != 30 {
		t.Fatalf("expected yesterday session minutes 30, got %d", report.YesterdaySessionMinutes)
	}
	if report.TodaySessionMinutes != 20 {
		t.Fatalf("expected today session minutes 20, got %d", report.TodaySessionMinutes)
	}
	if report.ThirtyDaySessionMinutes != 50 {
		t.Fatalf("expected 30-day session minutes 50, got %d", report.ThirtyDaySessionMinutes)
	}
	if report.LongestSessionDay.Date.Format("2006-01-02") != "2026-03-26" {
		t.Fatalf("expected longest session day on 2026-03-26, got %s", report.LongestSessionDay.Date.Format("2006-01-02"))
	}
	if report.LongestSessionDay.SessionMinutes != 30 {
		t.Fatalf("expected longest session minutes 30, got %d", report.LongestSessionDay.SessionMinutes)
	}
}

func TestLoadGlobalWithOptions_UsesConfiguredSessionGapForSummaryRollups(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	projectA := filepath.Join(tmp, "work-a")
	projectB := filepath.Join(tmp, "work-b")
	todayAnchor := startOfDay(time.Now().In(time.Local)).Add(12 * time.Hour)
	insertSession(t, db, "ses_a", projectA)
	insertSession(t, db, "ses_b", projectB)

	insertMessage(t, db, "msg_a1", "ses_a", todayAnchor.Add(-10*time.Minute), `{"role":"assistant","cost":1.25}`)
	insertPart(t, db, "part_a1", "msg_a1", "ses_a", todayAnchor.Add(-10*time.Minute), `{"type":"step-finish","tokens":{"input":100,"output":50,"reasoning":0}}`)
	insertMessage(t, db, "msg_a2", "ses_a", todayAnchor.Add(-7*time.Minute), `{"role":"assistant","cost":0.75}`)
	insertPart(t, db, "part_a2", "msg_a2", "ses_a", todayAnchor.Add(-7*time.Minute), `{"type":"step-finish","tokens":{"input":40,"output":10,"reasoning":0}}`)
	insertMessage(t, db, "msg_a3", "ses_a", todayAnchor, `{"role":"assistant","cost":0.50}`)
	insertPart(t, db, "part_a3", "msg_a3", "ses_a", todayAnchor, `{"type":"step-finish","tokens":{"input":20,"output":10,"reasoning":0}}`)

	insertMessage(t, db, "msg_b1", "ses_b", todayAnchor.Add(-24*time.Hour), `{"role":"assistant","cost":0.25}`)
	insertPart(t, db, "part_b1", "msg_b1", "ses_b", todayAnchor.Add(-24*time.Hour), `{"type":"step-finish","tokens":{"input":30,"output":20,"reasoning":0}}`)

	report, err := LoadGlobalWithOptions(Options{SessionGapMinutes: 5})
	if err != nil {
		t.Fatalf("LoadGlobalWithOptions returned error: %v", err)
	}

	if report.TodaySessionMinutes != 3 {
		t.Fatalf("expected 3 today session minutes with 5-minute gap, got %d", report.TodaySessionMinutes)
	}
	if report.ThirtyDaySessionMinutes != 3 {
		t.Fatalf("expected 3 30-day session minutes with 5-minute gap, got %d", report.ThirtyDaySessionMinutes)
	}
	if report.TodayTokens != 230 {
		t.Fatalf("expected global today tokens 230, got %d", report.TodayTokens)
	}
	if math.Abs(report.TodayCost-2.50) > 1e-9 {
		t.Fatalf("expected global today cost 2.50, got %.4f", report.TodayCost)
	}
	if report.ThirtyDayTokens != 280 {
		t.Fatalf("expected global 30-day tokens 280, got %d", report.ThirtyDayTokens)
	}
	if math.Abs(report.ThirtyDayCost-2.75) > 1e-9 {
		t.Fatalf("expected global 30-day cost 2.75, got %.4f", report.ThirtyDayCost)
	}
}

func TestLoadForDirWithOptions_FiltersScopedDirectoryAndUsesConfiguredSessionGap(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	targetDir := filepath.Join(tmp, "work")
	otherDir := filepath.Join(tmp, "other")
	todayAnchor := startOfDay(time.Now().In(time.Local)).Add(12 * time.Hour)
	insertSession(t, db, "ses_target", targetDir)
	insertSession(t, db, "ses_other", otherDir)

	insertMessage(t, db, "msg_target_1", "ses_target", todayAnchor.Add(-10*time.Minute), `{"role":"assistant","cost":1.50}`)
	insertPart(t, db, "part_target_1", "msg_target_1", "ses_target", todayAnchor.Add(-10*time.Minute), `{"type":"step-finish","tokens":{"input":90,"output":10,"reasoning":0}}`)
	insertMessage(t, db, "msg_target_2", "ses_target", todayAnchor.Add(-7*time.Minute), `{"role":"assistant","cost":0.50}`)
	insertPart(t, db, "part_target_2", "msg_target_2", "ses_target", todayAnchor.Add(-7*time.Minute), `{"type":"step-finish","tokens":{"input":20,"output":10,"reasoning":0}}`)
	insertMessage(t, db, "msg_target_3", "ses_target", todayAnchor, `{"role":"assistant","cost":0.25}`)
	insertPart(t, db, "part_target_3", "msg_target_3", "ses_target", todayAnchor, `{"type":"step-finish","tokens":{"input":5,"output":5,"reasoning":0}}`)

	insertMessage(t, db, "msg_other", "ses_other", todayAnchor.Add(-2*time.Minute), `{"role":"assistant","cost":9.00}`)
	insertPart(t, db, "part_other", "msg_other", "ses_other", todayAnchor.Add(-2*time.Minute), `{"type":"step-finish","tokens":{"input":400,"output":100,"reasoning":0}}`)

	report, err := LoadForDirWithOptions(targetDir, Options{SessionGapMinutes: 5})
	if err != nil {
		t.Fatalf("LoadForDirWithOptions returned error: %v", err)
	}

	if report.TodaySessionMinutes != 3 {
		t.Fatalf("expected 3 today session minutes for scoped project, got %d", report.TodaySessionMinutes)
	}
	if report.ThirtyDaySessionMinutes != 3 {
		t.Fatalf("expected 3 30-day session minutes for scoped project, got %d", report.ThirtyDaySessionMinutes)
	}
	if report.TodayTokens != 140 {
		t.Fatalf("expected scoped today tokens 140, got %d", report.TodayTokens)
	}
	if math.Abs(report.TodayCost-2.25) > 1e-9 {
		t.Fatalf("expected scoped today cost 2.25, got %.4f", report.TodayCost)
	}
}

func TestLoadForDirAt_AggregatesCodeLinesFromSessionSummaries(t *testing.T) {
	db, ntmp := openStatsTestDBWithSchema(t, `
		CREATE TABLE session (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			directory TEXT NOT NULL,
			parent_id TEXT,
			time_updated INTEGER NOT NULL,
			summary_additions INTEGER,
			summary_deletions INTEGER
		);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)

	dir := filepath.Join(ntmp, "work")
	otherDir := filepath.Join(ntmp, "other")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	dayBefore := now.AddDate(0, 0, -1)
	oldDay := now.AddDate(0, 0, -10)

	if _, err := db.Exec(`INSERT INTO session (id, title, directory, parent_id, time_updated, summary_additions, summary_deletions) VALUES (?, ?, ?, NULL, ?, ?, ?)`, "ses_today", "today", dir, now.UnixMilli(), 120, 30); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO session (id, title, directory, parent_id, time_updated, summary_additions, summary_deletions) VALUES (?, ?, ?, NULL, ?, ?, ?)`, "ses_yesterday", "yesterday", dir, dayBefore.UnixMilli(), 40, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO session (id, title, directory, parent_id, time_updated, summary_additions, summary_deletions) VALUES (?, ?, ?, NULL, ?, ?, ?)`, "ses_old", "old", dir, oldDay.UnixMilli(), 20, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO session (id, title, directory, parent_id, time_updated, summary_additions, summary_deletions) VALUES (?, ?, ?, NULL, ?, ?, ?)`, "ses_other", "other", otherDir, now.UnixMilli(), 999, 1); err != nil {
		t.Fatal(err)
	}

	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if report.TodayCodeLines != 150 {
		t.Fatalf("expected today code lines 150, got %d", report.TodayCodeLines)
	}
	if report.YesterdayCodeLines != 50 {
		t.Fatalf("expected yesterday code lines 50, got %d", report.YesterdayCodeLines)
	}
	if report.ThirtyDayCodeLines != 225 {
		t.Fatalf("expected 30-day code lines 225, got %d", report.ThirtyDayCodeLines)
	}
	if report.Days[len(report.Days)-1].CodeLines != 150 {
		t.Fatalf("expected today day bucket code lines 150, got %d", report.Days[len(report.Days)-1].CodeLines)
	}
	if report.Days[len(report.Days)-2].CodeLines != 50 {
		t.Fatalf("expected yesterday day bucket code lines 50, got %d", report.Days[len(report.Days)-2].CodeLines)
	}
	if report.HighestCodeDay.Date.Format("2006-01-02") != now.Format("2006-01-02") {
		t.Fatalf("expected highest code day on %s, got %s", now.Format("2006-01-02"), report.HighestCodeDay.Date.Format("2006-01-02"))
	}
	if report.HighestCodeDay.CodeLines != 150 {
		t.Fatalf("expected highest code day code lines 150, got %d", report.HighestCodeDay.CodeLines)
	}
}

func TestLoadForDirAt_AggregatesChangedFilesFromPartSignals(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	otherDir := filepath.Join(tmp, "other")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	yesterday := now.AddDate(0, 0, -1)

	insertSession(t, db, "ses_work", dir)
	insertSession(t, db, "ses_other", otherDir)

	insertMessage(t, db, "msg_today", "ses_work", now, `{"role":"assistant"}`)
	insertPart(t, db, "patch_today", "msg_today", "ses_work", now, `{"type":"patch","files":["internal/app/app.go","README.md"]}`)
	insertPart(t, db, "write_today", "msg_today", "ses_work", now, `{"type":"tool","tool":"write","state":{"status":"completed","input":{"filePath":"docs/notes.md"}}}`)
	insertPart(t, db, "edit_today", "msg_today", "ses_work", now, `{"type":"tool","tool":"edit","state":{"status":"completed","input":{"filePath":"docs/notes.md"}}}`)
	insertPart(t, db, "apply_patch_today", "msg_today", "ses_work", now, `{"type":"tool","tool":"apply_patch","state":{"status":"completed","input":{"patchText":"*** Begin Patch\n*** Add File: internal/new/file.go\n*** Update File: README.md\n*** Delete File: docs/old.md\n*** End Patch"}}}`)
	insertPart(t, db, "apply_patch_pending", "msg_today", "ses_work", now, `{"type":"tool","tool":"apply_patch","state":{"status":"pending","input":{"patchText":"*** Begin Patch\n*** Add File: should/not-count.go\n*** End Patch"}}}`)

	insertMessage(t, db, "msg_yesterday", "ses_work", yesterday, `{"role":"assistant"}`)
	insertPart(t, db, "patch_yesterday", "msg_yesterday", "ses_work", yesterday, `{"type":"patch","files":["cmd/oc/main.go","internal/tui/model.go"]}`)
	insertPart(t, db, "write_yesterday_dup", "msg_yesterday", "ses_work", yesterday, `{"type":"tool","tool":"write","state":{"status":"completed","input":{"filePath":"cmd/oc/main.go"}}}`)

	insertMessage(t, db, "msg_summary", "ses_work", now, `{"role":"assistant","summary":true,"agent":"compaction"}`)
	insertPart(t, db, "patch_summary", "msg_summary", "ses_work", now, `{"type":"patch","files":["ignored/summary.go"]}`)

	insertMessage(t, db, "msg_other", "ses_other", now, `{"role":"assistant"}`)
	insertPart(t, db, "patch_other", "msg_other", "ses_other", now, `{"type":"patch","files":["other/project.go"]}`)

	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if report.TodayChangedFiles != 5 {
		t.Fatalf("expected today changed files 5, got %d", report.TodayChangedFiles)
	}
	if report.YesterdayChangedFiles != 2 {
		t.Fatalf("expected yesterday changed files 2, got %d", report.YesterdayChangedFiles)
	}
	if report.ThirtyDayChangedFiles != 7 {
		t.Fatalf("expected 30-day changed files 7, got %d", report.ThirtyDayChangedFiles)
	}
	if report.Days[len(report.Days)-1].ChangedFiles != 5 {
		t.Fatalf("expected today day bucket changed files 5, got %d", report.Days[len(report.Days)-1].ChangedFiles)
	}
	if report.Days[len(report.Days)-2].ChangedFiles != 2 {
		t.Fatalf("expected yesterday day bucket changed files 2, got %d", report.Days[len(report.Days)-2].ChangedFiles)
	}
	if report.HighestChangedFilesDay.Date.Format("2006-01-02") != now.Format("2006-01-02") {
		t.Fatalf("expected highest changed-files day on %s, got %s", now.Format("2006-01-02"), report.HighestChangedFilesDay.Date.Format("2006-01-02"))
	}
	if report.HighestChangedFilesDay.ChangedFiles != 5 {
		t.Fatalf("expected highest changed-files day 5, got %d", report.HighestChangedFilesDay.ChangedFiles)
	}
}

func TestExtractChangedFilesFromPart_PatchAndToolInputs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{
			name: "patch payload uses files list",
			raw:  `{"type":"patch","files":["internal/app/app.go","README.md","README.md"]}`,
			want: []string{"README.md", "internal/app/app.go"},
		},
		{
			name: "tool payload uses completed apply_patch input",
			raw:  `{"type":"tool","tool":"apply_patch","state":{"status":"completed","input":{"patchText":"*** Begin Patch\n*** Add File: docs/new.md\n*** Update File: internal/stats/stats.go\n*** End Patch"}}}`,
			want: []string{"docs/new.md", "internal/stats/stats.go"},
		},
		{
			name: "completed write payload uses file path",
			raw:  `{"type":"tool","tool":"write","state":{"status":"completed","input":{"filePath":"docs/notes.md"}}}`,
			want: []string{"docs/notes.md"},
		},
		{
			name: "incomplete tool payload is ignored",
			raw:  `{"type":"tool","tool":"edit","state":{"status":"pending","input":{"filePath":"docs/notes.md"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractChangedFilesFromPart(tt.raw)
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("expected files %v, got %v", want, got)
			}
		})
	}
}

func TestExtractFilesFromPatchText_ReturnsTouchedFiles(t *testing.T) {
	got := extractFilesFromPatchText("*** Begin Patch\n*** Add File: internal/new/file.go\n*** Update File: README.md\n*** Delete File: docs/old.md\n*** End Patch")
	sort.Strings(got)
	want := []string{"README.md", "docs/old.md", "internal/new/file.go"}
	sort.Strings(want)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected files %v, got %v", want, got)
	}
}

func TestNormalizeChangedFilePath_NormalizesPlatformSpecificInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantPOSIX   string
		wantWindows string
	}{
		{
			name:        "trims and cleans relative path",
			input:       "  ./docs/../internal/stats.go  ",
			wantPOSIX:   "internal/stats.go",
			wantWindows: "internal/stats.go",
		},
		{
			name:        "empty path stays empty",
			input:       "   ",
			wantPOSIX:   "",
			wantWindows: "",
		},
		{
			name:        "windows normalizes slashes and casing",
			input:       `  .\Internal\Stats.GO  `,
			wantPOSIX:   `.\Internal\Stats.GO`,
			wantWindows: "internal/stats.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := tt.wantPOSIX
			if runtime.GOOS == "windows" {
				want = tt.wantWindows
			}

			if got := normalizeChangedFilePath(tt.input); got != want {
				t.Fatalf("expected normalized path %q, got %q", want, got)
			}
		})
	}
}

func TestMergeSessionCodeStats_UsesSummaryColumnsWhenPresent(t *testing.T) {
	db, tmp := openStatsTestDBWithSchema(t, `
		CREATE TABLE session (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL DEFAULT '',
			directory TEXT NOT NULL,
			parent_id TEXT,
			time_updated INTEGER NOT NULL,
			summary_additions INTEGER,
			summary_deletions INTEGER
		);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)

	day := newEmptyDay(time.Date(2026, time.March, 27, 0, 0, 0, 0, time.Local))
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	dayMap := map[string]*Day{dayKey(now): &day}
	dir := filepath.Join(tmp, "work")

	if _, err := db.Exec(`INSERT INTO session (id, title, directory, parent_id, time_updated, summary_additions, summary_deletions) VALUES (?, ?, ?, NULL, ?, ?, ?)`, "ses_today", "today", dir, now.UnixMilli(), 120, 30); err != nil {
		t.Fatal(err)
	}

	if err := mergeSessionCodeStats(db, dir, now.Add(-time.Hour).UnixMilli(), time.Local, dayMap); err != nil {
		t.Fatalf("mergeSessionCodeStats returned error: %v", err)
	}

	if dayMap[dayKey(now)].CodeLines != 150 {
		t.Fatalf("expected today code lines 150, got %d", dayMap[dayKey(now)].CodeLines)
	}
}

func TestLoadForDirAt_UsesStepFinishCostWhenMessageCostMissing(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertMessage(t, db, "msg_today", "ses_work", now, `{"role":"assistant"}`)
	insertPart(t, db, "step_today", "msg_today", "ses_work", now, `{"type":"step-finish","cost":2.25,"tokens":{"input":20,"output":10,"reasoning":5}}`)

	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}
	if report.TodayCost != 2.25 {
		t.Fatalf("expected step-finish fallback cost 2.25, got %.2f", report.TodayCost)
	}
}

func TestLoadForDirAt_DoesNotDoubleCountStepFinishCostWhenMessageCostExists(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertMessage(t, db, "msg_today", "ses_work", now, `{"role":"assistant","cost":1.84}`)
	insertPart(t, db, "step_today", "msg_today", "ses_work", now, `{"type":"step-finish","cost":2.25,"tokens":{"input":20,"output":10,"reasoning":5}}`)

	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}
	if report.TodayCost != 1.84 {
		t.Fatalf("expected message cost 1.84 without double count, got %.2f", report.TodayCost)
	}
}

func TestLoadForDirAt_ComputesLiteLLMCostWhenStoredCostsMissing(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertMessage(t, db, "msg_today", "ses_work", now, `{"role":"assistant","providerID":"openai","modelID":"gpt-4o-mini"}`)
	insertPart(t, db, "step_today", "msg_today", "ses_work", now, `{"type":"step-finish","tokens":{"input":1000,"output":500,"reasoning":0,"cache":{"read":200,"write":100}}}`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"gpt-4o-mini":{"input_cost_per_token":0.000001,"output_cost_per_token":0.000002,"cache_creation_input_token_cost":0.000003,"cache_read_input_token_cost":0.0000005}}`)
	}))
	defer server.Close()

	previousResolver := defaultPricingResolver
	defaultPricingResolver = newLiteLLMPricingResolver(server.URL, server.Client())
	t.Cleanup(func() {
		defaultPricingResolver = previousResolver
	})

	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	const expected = 0.0024
	if report.TodayCost != expected {
		t.Fatalf("expected LiteLLM fallback cost %.4f, got %.4f", expected, report.TodayCost)
	}
	if report.TodayTokens != 1800 {
		t.Fatalf("expected visible tokens 1800, got %d", report.TodayTokens)
	}
}
