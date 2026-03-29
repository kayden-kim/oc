package stats

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestLoadForDirAt_AggregatesGlobalStatsAndFiltersSynthetic(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

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

	t.Setenv("OPENCODE_DB", dbPath)
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
	if report.CoachingNote != "reasoning elevated, but overall cadence is steady" {
		t.Fatalf("unexpected coaching note: %q", report.CoachingNote)
	}
}

func TestLoadForDirAt_Uses30DaySumsForWeeklyTotals(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_old", "ses_work", now.AddDate(0, 0, -10), `{"role":"assistant","cost":2.00}`)
	insertPart(t, db, "step_old", "msg_old", "ses_work", now.AddDate(0, 0, -10), `{"type":"step-finish","tokens":{"input":20,"output":30,"reasoning":10}}`)

	insertMessage(t, db, "msg_today", "ses_work", now, `{"role":"assistant","cost":3.00}`)
	insertPart(t, db, "step_today", "msg_today", "ses_work", now, `{"type":"step-finish","tokens":{"input":40,"output":30,"reasoning":20}}`)

	t.Setenv("OPENCODE_DB", dbPath)
	report, err := loadForDirAt(dir, now)
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
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_main", "ses_work", now, `{"role":"assistant","cost":1.00}`)
	insertPart(t, db, "main_step", "msg_main", "ses_work", now, `{"type":"step-finish","tokens":{"input":10,"output":10,"reasoning":0}}`)

	insertMessage(t, db, "msg_explore", "ses_work", now, `{"role":"assistant","agent":"explore","cost":0.20}`)
	insertPart(t, db, "explore_start", "msg_explore", "ses_work", now, `{"type":"step-start"}`)
	insertPart(t, db, "explore_tool", "msg_explore", "ses_work", now, `{"type":"tool","tool":"read"}`)
	insertPart(t, db, "explore_finish", "msg_explore", "ses_work", now, `{"type":"step-finish","tokens":{"input":5,"output":5,"reasoning":0}}`)

	insertMessage(t, db, "msg_librarian", "ses_work", now, `{"role":"assistant","agent":"librarian","cost":0.30}`)
	insertPart(t, db, "librarian_finish", "msg_librarian", "ses_work", now, `{"type":"step-finish","tokens":{"input":6,"output":4,"reasoning":0}}`)

	insertMessage(t, db, "msg_user_agent", "ses_work", now, `{"role":"user","agent":"explore"}`)
	insertPart(t, db, "user_agent_finish", "msg_user_agent", "ses_work", now, `{"type":"step-finish","tokens":{"input":1,"output":1,"reasoning":0}}`)

	insertMessage(t, db, "msg_compaction", "ses_work", now, `{"role":"assistant","agent":"compaction","cost":5.00}`)
	insertPart(t, db, "compaction_part", "msg_compaction", "ses_work", now, `{"type":"step-finish","tokens":{"input":50,"output":50,"reasoning":50}}`)

	insertMessage(t, db, "msg_part_subtask", "ses_work", now, `{"role":"assistant","cost":0.10}`)
	insertPart(t, db, "part_subtask", "msg_part_subtask", "ses_work", now, `{"type":"subtask","agent":"legacy-subtask"}`)

	t.Setenv("OPENCODE_DB", dbPath)
	report, err := loadForDirAt(dir, now)
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
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertMessage(t, db, "msg_tools", "ses_work", now, `{"role":"assistant","cost":1.00}`)

	toolCounts := map[string]int{"bash": 5, "read": 4, "edit": 3, "grep": 3, "write": 2, "glob": 1}
	partID := 0
	for tool, count := range toolCounts {
		for range count {
			insertPart(t, db, fmt.Sprintf("tool_%d", partID), "msg_tools", "ses_work", now, fmt.Sprintf(`{"type":"tool","tool":%q}`, tool))
			partID++
		}
	}

	t.Setenv("OPENCODE_DB", dbPath)
	report, err := loadForDirAt(dir, now)
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
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertMessage(t, db, "msg_skills", "ses_work", now, `{"role":"assistant","cost":1.00}`)

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
		insertPart(t, db, fmt.Sprintf("skill_%d", i), "msg_skills", "ses_work", now, data)
	}

	t.Setenv("OPENCODE_DB", dbPath)
	report, err := loadForDirAt(dir, now)
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
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	agents := []string{"explore", "explore", "explore", "oracle", "oracle", "planner", "planner", "review", "debug", "compaction"}
	for i, agent := range agents {
		insertMessage(t, db, fmt.Sprintf("msg_agent_%d", i), "ses_work", now, fmt.Sprintf(`{"role":"assistant","agent":%q}`, agent))
	}
	insertMessage(t, db, "msg_user_agent", "ses_work", now, `{"role":"user","agent":"explore"}`)
	insertMessage(t, db, "msg_plain", "ses_work", now, `{"role":"assistant"}`)
	insertMessage(t, db, "msg_legacy_subtask", "ses_work", now, `{"role":"assistant"}`)
	insertPart(t, db, "part_legacy_subtask", "msg_legacy_subtask", "ses_work", now, `{"type":"subtask","agent":"legacy-subtask"}`)

	t.Setenv("OPENCODE_DB", dbPath)
	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	topAgents := rankedUsageFromReportField(t, report, "TopAgents")
	expected := []usageSnapshot{{"explore", 3}, {"oracle", 2}, {"planner", 2}, {"debug", 1}, {"review", 1}}
	if !reflect.DeepEqual(topAgents, expected) {
		t.Fatalf("expected top agents %v, got %v", expected, topAgents)
	}
}

func TestLoadForDirAt_BuildsTopModelUsage(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)

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
		insertMessage(t, db, msgID, "ses_work", now, fmt.Sprintf(`{"role":"assistant","providerID":%q,"modelID":%q,"cost":1.00}`, item.provider, item.model))
		insertPart(t, db, partID, msgID, "ses_work", now, fmt.Sprintf(`{"type":"step-finish","tokens":{"input":%d,"output":%d,"reasoning":%d,"cache":{"read":%d,"write":%d}}}`,
			item.input,
			item.output,
			item.reason,
			item.cacheR,
			item.cacheW,
		))
	}

	insertMessage(t, db, "msg_missing_provider", "ses_work", now, `{"role":"assistant","modelID":"skip-me"}`)
	insertPart(t, db, "part_missing_provider", "msg_missing_provider", "ses_work", now, `{"type":"step-finish","tokens":{"input":999,"output":999,"reasoning":999}}`)
	insertMessage(t, db, "msg_missing_model", "ses_work", now, `{"role":"assistant","providerID":"openai"}`)
	insertPart(t, db, "part_missing_model", "msg_missing_model", "ses_work", now, `{"type":"step-finish","tokens":{"input":999,"output":999,"reasoning":999}}`)

	t.Setenv("OPENCODE_DB", dbPath)
	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}

	if got := intFieldFromReport(t, report, "TotalModelTokens"); got != 4570 {
		t.Fatalf("expected 4570 total model tokens, got %d", got)
	}
	if got := intFieldFromReport(t, report, "UniqueModelCount"); got != 12 {
		t.Fatalf("expected 12 unique models, got %d", got)
	}

	topModels := rankedUsageMetricsFromReportField(t, report, "TopModels")
	if len(topModels) != 12 {
		t.Fatalf("expected all 12 ranked models, got %d (%v)", len(topModels), topModels)
	}
	expected := []usageMetric{
		{Name: "[OpenAI] gpt-5.4", Amount: 1120},
		{Name: "[Anthropic] claude-sonnet-4.5", Amount: 900},
		{Name: "[Google] gemini-2.5-pro", Amount: 690},
		{Name: "[OpenRouter] qwen/qwen3-coder", Amount: 475},
		{Name: "[Azure] gpt-4.1", Amount: 265},
		{Name: "[Bedrock] claude-3.7-sonnet", Amount: 235},
		{Name: "[Vertex] gemini-2.0-flash", Amount: 210},
		{Name: "[Copilot] gpt-4o", Amount: 185},
		{Name: "[Copilot] mistral-large", Amount: 160},
		{Name: "[OpenAI] o4-mini", Amount: 135},
		{Name: "[Anthropic] claude-haiku-4.5", Amount: 110},
		{Name: "[Google] gemini-2.0-flash-lite", Amount: 85},
	}
	if !reflect.DeepEqual(topModels, expected) {
		t.Fatalf("expected top models %v, got %v", expected, topModels)
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
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

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

	t.Setenv("OPENCODE_DB", dbPath)
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

func TestLoadForDirAt_AggregatesCodeLinesFromSessionSummaries(t *testing.T) {
	ntmp := t.TempDir()
	dbPath := filepath.Join(ntmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
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
	if err != nil {
		t.Fatal(err)
	}

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

	t.Setenv("OPENCODE_DB", dbPath)
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

func TestLoadForDirAt_UsesStepFinishCostWhenMessageCostMissing(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertMessage(t, db, "msg_today", "ses_work", now, `{"role":"assistant"}`)
	insertPart(t, db, "step_today", "msg_today", "ses_work", now, `{"type":"step-finish","cost":2.25,"tokens":{"input":20,"output":10,"reasoning":5}}`)

	t.Setenv("OPENCODE_DB", dbPath)
	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}
	if report.TodayCost != 2.25 {
		t.Fatalf("expected step-finish fallback cost 2.25, got %.2f", report.TodayCost)
	}
}

func TestLoadForDirAt_DoesNotDoubleCountStepFinishCostWhenMessageCostExists(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(tmp, "work")
	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.Local)
	insertSession(t, db, "ses_work", dir)
	insertMessage(t, db, "msg_today", "ses_work", now, `{"role":"assistant","cost":1.84}`)
	insertPart(t, db, "step_today", "msg_today", "ses_work", now, `{"type":"step-finish","cost":2.25,"tokens":{"input":20,"output":10,"reasoning":5}}`)

	t.Setenv("OPENCODE_DB", dbPath)
	report, err := loadForDirAt(dir, now)
	if err != nil {
		t.Fatalf("loadForDirAt returned error: %v", err)
	}
	if report.TodayCost != 1.84 {
		t.Fatalf("expected message cost 1.84 without double count, got %.2f", report.TodayCost)
	}
}

func TestLoadForDirAt_ComputesLiteLLMCostWhenStoredCostsMissing(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec(`
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	if err != nil {
		t.Fatal(err)
	}

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

	t.Setenv("OPENCODE_DB", dbPath)
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

func TestLiteLLMPricingResolver_UsesThresholdRates(t *testing.T) {
	entry := liteLLMPricingEntry{
		InputCostPerToken: ptrFloat(0.000001),
		ThresholdPricing: []thresholdPricing{{
			Threshold:         200000,
			InputCostPerToken: ptrFloat(0.000002),
		}},
	}

	cost := entry.estimateCost(pricedUsage{InputTokens: 300000})
	if cost != 0.6 {
		t.Fatalf("expected threshold-based cost 0.6, got %.4f", cost)
	}
}

func TestLiteLLMPricingResolver_UsesAliasMapping(t *testing.T) {
	resolver := &liteLLMPricingResolver{
		entries: map[string]liteLLMPricingEntry{
			"gemini-3-pro-preview": {InputCostPerToken: ptrFloat(0.000001)},
		},
	}
	resolver.initOnce.Do(func() {})

	cost, err := resolver.EstimateCost(pricedUsage{ModelID: "gemini-3-pro-high", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if math.Abs(cost-0.00001) > 1e-12 {
		t.Fatalf("expected alias-based cost 0.00001, got %.8f", cost)
	}
}

func TestLiteLLMPricingResolver_UsesLocalCacheImmediately(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	cacheDir := filepath.Join(tmp, "oc")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "litellm-pricing-cache.json")
	metaPath := filepath.Join(cacheDir, "litellm-pricing-cache-meta.json")
	if err := os.WriteFile(cachePath, []byte(`{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 26, 9, 0, 0, 0, time.Local)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, meta, 0o644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		fmt.Fprint(w, `{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`)
	}))
	defer server.Close()

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 27, 10, 0, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, server.Client())
	start := time.Now()
	cost, err := resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("expected cached pricing to return immediately, got %v", elapsed)
	}
	if math.Abs(cost-0.00001) > 1e-12 {
		t.Fatalf("expected cached price 0.00001, got %.8f", cost)
	}
}

func TestLiteLLMPricingResolver_RefreshesAtMostOncePerRollingWindow(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	cacheDir := filepath.Join(tmp, "oc")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cachePath := filepath.Join(cacheDir, "litellm-pricing-cache.json")
	metaPath := filepath.Join(cacheDir, "litellm-pricing-cache-meta.json")
	if err := os.WriteFile(cachePath, []byte(`{"gpt-4o-mini":{"input_cost_per_token":0.000001}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	meta, err := json.Marshal(pricingCacheMetadata{LastAttempt: time.Date(2026, time.March, 27, 1, 0, 0, 0, time.Local)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metaPath, meta, 0o644); err != nil {
		t.Fatal(err)
	}

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		fmt.Fprint(w, `{"gpt-4o-mini":{"input_cost_per_token":0.000002}}`)
	}))
	defer server.Close()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	transport := client.Transport
	client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		clone := req.Clone(req.Context())
		clone.URL.Scheme = serverURL.Scheme
		clone.URL.Host = serverURL.Host
		return transport.RoundTrip(clone)
	})

	previousNow := liteLLMNow
	liteLLMNow = func() time.Time { return time.Date(2026, time.March, 28, 1, 1, 0, 0, time.Local) }
	defer func() { liteLLMNow = previousNow }()

	resolver := newLiteLLMPricingResolver(liteLLMDefaultPricingURL, client)
	_, err = resolver.EstimateCost(pricedUsage{ModelID: "gpt-4o-mini", InputTokens: 10})
	if err != nil {
		t.Fatalf("EstimateCost returned error: %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && hits.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected one refresh after rolling window expired, got %d fetches", hits.Load())
	}
}

func ptrFloat(value float64) *float64 {
	return &value
}

type roundTripFunc func(*http.Request) (*http.Response, error)

type usageSnapshot struct {
	Name  string
	Count int
}

type usageMetric struct {
	Name   string
	Count  int
	Amount int64
}

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func rankedUsageFromReportField(t *testing.T, report Report, fieldName string) []usageSnapshot {
	t.Helper()
	value := reflect.ValueOf(report)
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	items := make([]usageSnapshot, 0, field.Len())
	for i := range field.Len() {
		entry := field.Index(i)
		items = append(items, usageSnapshot{
			Name:  entry.FieldByName("Name").String(),
			Count: int(entry.FieldByName("Count").Int()),
		})
	}
	return items
}

func rankedUsageMetricsFromReportField(t *testing.T, report Report, fieldName string) []usageMetric {
	t.Helper()
	value := reflect.ValueOf(report)
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		return nil
	}
	items := make([]usageMetric, 0, field.Len())
	for i := range field.Len() {
		entry := field.Index(i)
		item := usageMetric{Name: entry.FieldByName("Name").String()}
		if v := entry.FieldByName("Count"); v.IsValid() {
			item.Count = int(v.Int())
		}
		if v := entry.FieldByName("Amount"); v.IsValid() {
			item.Amount = v.Int()
		}
		items = append(items, item)
	}
	return items
}

func intFieldFromReport(t *testing.T, report Report, fieldName string) int64 {
	t.Helper()
	field := reflect.ValueOf(report).FieldByName(fieldName)
	if !field.IsValid() {
		return 0
	}
	return field.Int()
}

func insertSession(t *testing.T, db *sql.DB, id string, dir string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO session (id, title, directory, parent_id, time_updated) VALUES (?, ?, ?, NULL, ?)`, id, id, dir, time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}
}

func insertMessage(t *testing.T, db *sql.DB, id string, sessionID string, created time.Time, data string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO message (id, session_id, time_created, data) VALUES (?, ?, ?, ?)`, id, sessionID, created.UnixMilli(), data); err != nil {
		t.Fatal(err)
	}
}

func insertPart(t *testing.T, db *sql.DB, id string, messageID string, sessionID string, created time.Time, data string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO part (id, message_id, session_id, time_created, data) VALUES (?, ?, ?, ?, ?)`, id, messageID, sessionID, created.UnixMilli(), data); err != nil {
		t.Fatal(err)
	}
}
