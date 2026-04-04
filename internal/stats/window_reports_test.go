package stats

import (
	"math"
	"path/filepath"
	"testing"
	"time"
)

func TestSlotTokensBucketing(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_a", "ses_work", time.Date(2026, time.March, 29, 9, 15, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_a", "msg_a", "ses_work", time.Date(2026, time.March, 29, 9, 15, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":200,"reasoning":50}}`)

	insertMessage(t, db, "msg_b", "ses_work", time.Date(2026, time.March, 29, 9, 45, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_b", "msg_b", "ses_work", time.Date(2026, time.March, 29, 9, 45, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":300,"output":400,"reasoning":100}}`)

	insertMessage(t, db, "msg_c", "ses_work", time.Date(2026, time.March, 29, 9, 20, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_c", "msg_c", "ses_work", time.Date(2026, time.March, 29, 9, 20, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":50,"output":50,"reasoning":0}}`)

	now := time.Date(2026, time.March, 29, 12, 0, 0, 0, time.Local)
	report, err := loadForDirAtWithOptions(dir, now, Options{SessionGapMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}

	today := report.Days[len(report.Days)-1]
	if today.SlotTokens[18] != 450 {
		t.Errorf("slot 18: got %d, want 450", today.SlotTokens[18])
	}
	if today.SlotTokens[19] != 800 {
		t.Errorf("slot 19: got %d, want 800", today.SlotTokens[19])
	}
	if today.SlotTokens[0] != 0 {
		t.Errorf("slot 0: got %d, want 0", today.SlotTokens[0])
	}
}

func TestRolling24hSlotAssembly(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_y1", "ses_work", time.Date(2026, time.March, 28, 22, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_y1", "msg_y1", "ses_work", time.Date(2026, time.March, 28, 22, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":500,"output":500,"reasoning":0}}`)

	insertMessage(t, db, "msg_y2", "ses_work", time.Date(2026, time.March, 28, 23, 30, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_y2", "msg_y2", "ses_work", time.Date(2026, time.March, 28, 23, 30, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":200,"output":200,"reasoning":0}}`)

	insertMessage(t, db, "msg_t1", "ses_work", time.Date(2026, time.March, 29, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_t1", "msg_t1", "ses_work", time.Date(2026, time.March, 29, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":300,"output":300,"reasoning":0}}`)

	now := time.Date(2026, time.March, 29, 10, 15, 0, 0, time.Local)
	report, err := loadForDirAtWithOptions(dir, now, Options{SessionGapMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}

	if report.Rolling24hSlots[23] != 1000 {
		t.Errorf("rolling slot 23 (yesterday 22:00): got %d, want 1000", report.Rolling24hSlots[23])
	}
	if report.Rolling24hSlots[26] != 400 {
		t.Errorf("rolling slot 26 (yesterday 23:30): got %d, want 400", report.Rolling24hSlots[26])
	}
	if report.Rolling24hSlots[47] != 600 {
		t.Errorf("rolling slot 47 (today 10:00): got %d, want 600", report.Rolling24hSlots[47])
	}
	if report.Rolling24hSlots[0] != 0 {
		t.Errorf("rolling slot 0 (inactive): got %d, want 0", report.Rolling24hSlots[0])
	}
}

func TestHourlyStreakSlotsAcrossHalfHourWindows(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_a", "ses_work", time.Date(2026, time.March, 29, 8, 15, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_a", "msg_a", "ses_work", time.Date(2026, time.March, 29, 8, 15, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":100}}`)
	insertMessage(t, db, "msg_b", "ses_work", time.Date(2026, time.March, 29, 8, 45, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_b", "msg_b", "ses_work", time.Date(2026, time.March, 29, 8, 45, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":100}}`)
	insertMessage(t, db, "msg_c", "ses_work", time.Date(2026, time.March, 29, 9, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_c", "msg_c", "ses_work", time.Date(2026, time.March, 29, 9, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":100}}`)
	insertMessage(t, db, "msg_d", "ses_work", time.Date(2026, time.March, 30, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_d", "msg_d", "ses_work", time.Date(2026, time.March, 30, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":100}}`)
	insertMessage(t, db, "msg_e", "ses_work", time.Date(2026, time.March, 30, 10, 30, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_e", "msg_e", "ses_work", time.Date(2026, time.March, 30, 10, 30, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":100}}`)

	now := time.Date(2026, time.March, 30, 12, 0, 0, 0, time.Local)
	report, err := loadForDirAtWithOptions(dir, now, Options{SessionGapMinutes: 15})
	if err != nil {
		t.Fatal(err)
	}

	if report.CurrentHourlyStreakSlots != 2 {
		t.Fatalf("expected current hourly streak 2 slots, got %d", report.CurrentHourlyStreakSlots)
	}
	if report.BestHourlyStreakSlots != 3 {
		t.Fatalf("expected best hourly streak 3 slots, got %d", report.BestHourlyStreakSlots)
	}
}

func TestBuildWindowReport_PopulatesDailyDetailHourlyAndAllSessions(t *testing.T) {
	db, _ := openStatsTestDBWithSchema(t, `
		CREATE TABLE session (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', directory TEXT NOT NULL, parent_id TEXT, time_updated INTEGER NOT NULL, summary_additions INTEGER NOT NULL DEFAULT 0, summary_deletions INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE message (id TEXT PRIMARY KEY, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
		CREATE TABLE part (id TEXT PRIMARY KEY, message_id TEXT NOT NULL, session_id TEXT NOT NULL, time_created INTEGER NOT NULL, data TEXT NOT NULL);
	`)
	tmp := t.TempDir()

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_a", dir)
	insertSession(t, db, "ses_b", dir)
	start := time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local)
	insertMessage(t, db, "msg_a", "ses_a", start.Add(9*time.Hour), `{"role":"assistant"}`)
	insertPart(t, db, "part_a", "msg_a", "ses_a", start.Add(9*time.Hour), `{"type":"step-finish","tokens":{"input":1000,"output":500}}`)
	insertMessage(t, db, "msg_a2", "ses_a", start.Add(9*time.Hour+10*time.Minute), `{"role":"assistant"}`)
	insertPart(t, db, "part_a2", "msg_a2", "ses_a", start.Add(9*time.Hour+10*time.Minute), `{"type":"step-finish","tokens":{"input":500,"output":500}}`)
	insertMessage(t, db, "msg_b", "ses_b", start.Add(11*time.Hour+30*time.Minute), `{"role":"assistant"}`)
	insertPart(t, db, "part_b", "msg_b", "ses_b", start.Add(11*time.Hour+30*time.Minute), `{"type":"step-finish","tokens":{"input":2000,"output":1000}}`)
	if _, err := db.Exec(`UPDATE session SET summary_additions = 120, summary_deletions = 30, time_updated = ? WHERE id = ?`, start.Add(20*time.Hour).UnixMilli(), "ses_a"); err != nil {
		t.Fatal(err)
	}
	insertPart(t, db, "part_patch", "msg_a", "ses_a", start.Add(9*time.Hour+20*time.Minute), `{"type":"tool","tool":"apply_patch","state":{"status":"completed","input":{"patchText":"*** Begin Patch\n*** Update File: foo.go\n*** Add File: bar.go\n*** End Patch"}}}`)

	report, err := buildWindowReport(db, dir, "Daily", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.AllSessions) != 2 {
		t.Fatalf("expected all sessions populated, got %+v", report.AllSessions)
	}
	if len(report.TopTools) != 1 || len(report.TopSkills) != 0 || len(report.TopAgents) != 0 || len(report.TopProjects) != 1 {
		t.Fatalf("expected day-scoped activity aggregates, got tools=%v skills=%v agents=%v projects=%v", report.TopTools, report.TopSkills, report.TopAgents, report.TopProjects)
	}
	if report.HalfHourSlots[18] != 2500 {
		t.Fatalf("expected 09:00 half-hour slot bucket to include both 09:00 and 09:10 events, got %d", report.HalfHourSlots[18])
	}
	if report.HalfHourSlots[23] != 3000 {
		t.Fatalf("expected 11:30 half-hour slot to be populated, got %d", report.HalfHourSlots[23])
	}
	if report.ActiveMinutes <= 0 {
		t.Fatalf("expected active minutes to be computed, got %d", report.ActiveMinutes)
	}
	if report.InputTokens != 3500 || report.OutputTokens != 2000 {
		t.Fatalf("expected token breakdown totals, got input=%d output=%d", report.InputTokens, report.OutputTokens)
	}
	if report.CodeLines != 150 {
		t.Fatalf("expected window code lines from session summaries, got %d", report.CodeLines)
	}
	if report.ChangedFiles != 2 {
		t.Fatalf("expected changed files deduped from patch payload, got %d", report.ChangedFiles)
	}
}

func TestBuildWindowReport_PopulatesDailyDetailActivityTables(t *testing.T) {
	db, _ := openStatsTestDB(t)
	tmp := t.TempDir()

	dirA := filepath.Join(tmp, "work-a")
	dirB := filepath.Join(tmp, "work-b")
	insertSession(t, db, "ses_a", dirA)
	insertSession(t, db, "ses_b", dirB)
	start := time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local)
	insertMessage(t, db, "msg_agent", "ses_a", start.Add(9*time.Hour), `{"role":"assistant","agent":"explore","modelID":"gpt-5.4"}`)
	insertPart(t, db, "part_tool", "msg_agent", "ses_a", start.Add(9*time.Hour+5*time.Minute), `{"type":"tool","tool":"bash"}`)
	insertPart(t, db, "part_skill", "msg_agent", "ses_a", start.Add(9*time.Hour+10*time.Minute), `{"type":"tool","tool":"skill","state":{"input":{"name":"writing-plans"}}}`)
	insertPart(t, db, "part_step_a", "msg_agent", "ses_a", start.Add(9*time.Hour+15*time.Minute), `{"type":"step-finish","tokens":{"input":1000,"output":500},"cost":1.5}`)
	insertMessage(t, db, "msg_b", "ses_b", start.Add(11*time.Hour), `{"role":"assistant","agent":"explore","modelID":"gpt-5.4"}`)
	insertPart(t, db, "part_step_b", "msg_b", "ses_b", start.Add(11*time.Hour), `{"type":"step-finish","tokens":{"input":2000,"output":1000},"cost":2.0}`)

	report, err := buildWindowReport(db, "", "Daily", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.TopProjects) != 2 || report.TotalProjectCost <= 0 {
		t.Fatalf("expected project activity aggregated, got %+v totalCost=%v", report.TopProjects, report.TotalProjectCost)
	}
	if len(report.TopAgents) != 1 || report.TopAgents[0].Name != "explore" || report.TotalSubtasks != 2 {
		t.Fatalf("expected agent activity aggregated, got %+v total=%d", report.TopAgents, report.TotalSubtasks)
	}
	if len(report.TopAgentModels) != 1 || report.TopAgentModels[0].Name != "explore\x00gpt-5.4" || report.TotalAgentModelCalls != 2 {
		t.Fatalf("expected agent-model activity aggregated, got %+v total=%d", report.TopAgentModels, report.TotalAgentModelCalls)
	}
	if len(report.TopSkills) != 1 || report.TopSkills[0].Name != "writing-plans" || report.TotalSkillCalls != 1 {
		t.Fatalf("expected skill activity aggregated, got %+v total=%d", report.TopSkills, report.TotalSkillCalls)
	}
	if len(report.TopTools) != 2 || report.TotalToolCalls != 2 {
		t.Fatalf("expected tool activity aggregated, got %+v total=%d", report.TopTools, report.TotalToolCalls)
	}
}

func TestBuildWindowReport_PreservesProviderModelKeyInModels(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_a", dir)
	start := time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local)
	insertMessage(t, db, "msg_model", "ses_a", start.Add(9*time.Hour), `{"role":"assistant","providerID":"openai","modelID":"gpt-5.4"}`)
	insertPart(t, db, "part_model", "msg_model", "ses_a", start.Add(9*time.Hour), `{"type":"step-finish","tokens":{"input":1000,"output":500},"cost":1.5}`)

	report, err := buildWindowReport(db, dir, "Daily", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Models) != 1 {
		t.Fatalf("expected one model usage row, got %+v", report.Models)
	}
	if report.Models[0].Model != "openai\x00gpt-5.4" {
		t.Fatalf("expected provider/model key preserved, got %q", report.Models[0].Model)
	}
}

func TestLoadWindowReport_FiltersScopedDirectoryFromConfiguredDB(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	targetDir := filepath.Join(tmp, "work")
	otherDir := filepath.Join(tmp, "other")
	start := time.Now().In(time.Local).Add(-2 * time.Hour).Truncate(time.Hour)
	insertSession(t, db, "ses_target", targetDir)
	insertSession(t, db, "ses_other", otherDir)

	insertMessage(t, db, "msg_target", "ses_target", start.Add(30*time.Minute), `{"role":"assistant","cost":1.25}`)
	insertPart(t, db, "part_target", "msg_target", "ses_target", start.Add(30*time.Minute), `{"type":"step-finish","tokens":{"input":100,"output":50,"reasoning":0}}`)
	insertMessage(t, db, "msg_other", "ses_other", start.Add(45*time.Minute), `{"role":"assistant","cost":5.00}`)
	insertPart(t, db, "part_other", "msg_other", "ses_other", start.Add(45*time.Minute), `{"type":"step-finish","tokens":{"input":500,"output":100,"reasoning":0}}`)

	report, err := LoadWindowReport(targetDir, "Daily", start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("LoadWindowReport returned error: %v", err)
	}

	if report.Tokens != 150 {
		t.Fatalf("expected scoped window tokens 150, got %d", report.Tokens)
	}
	if math.Abs(report.Cost-1.25) > 1e-9 {
		t.Fatalf("expected scoped window cost 1.25, got %.4f", report.Cost)
	}
	if report.Sessions != 1 {
		t.Fatalf("expected one scoped session, got %d", report.Sessions)
	}
}
