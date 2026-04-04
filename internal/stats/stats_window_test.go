package stats

import (
	"fmt"
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

func TestDeriveFocusTag_Spike_HighestAndGreaterThan125Percent(t *testing.T) {
	tokens := int64(1000)
	cost := 10.0
	allTokens := []int64{1000, 600, 400, 200}
	allCosts := []float64{10.0, 8.0, 5.0, 2.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "spike" {
		t.Errorf("expected spike, got %s", tag)
	}
}

func TestDeriveFocusTag_Spike_HighestButNotEnough125Percent(t *testing.T) {
	tokens := int64(1000)
	cost := 10.0
	allTokens := []int64{1000, 850, 400, 200}
	allCosts := []float64{10.0, 8.0, 5.0, 2.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "--" {
		t.Errorf("expected --, got %s", tag)
	}
}

func TestDeriveFocusTag_Spike_NotHighest(t *testing.T) {
	tokens := int64(600)
	cost := 8.0
	allTokens := []int64{1000, 600, 400, 200}
	allCosts := []float64{10.0, 8.0, 5.0, 2.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag == "spike" {
		t.Errorf("expected not spike, got %s", tag)
	}
}

func TestDeriveFocusTag_Heavy_TokensAboveMedian(t *testing.T) {
	tokens := int64(2000)
	cost := 1.0
	allTokens := []int64{500, 600, 700, 800, 900}
	allCosts := []float64{1.0, 1.0, 1.0, 1.0, 1.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "heavy" {
		t.Errorf("expected heavy, got %s", tag)
	}
}

func TestDeriveFocusTag_Heavy_CostAboveMedian(t *testing.T) {
	tokens := int64(500)
	cost := 20.0
	allTokens := []int64{500, 500, 500, 500, 500}
	allCosts := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "heavy" {
		t.Errorf("expected heavy, got %s", tag)
	}
}

func TestDeriveFocusTag_Quiet_BothTokensAndCostBelowMedian(t *testing.T) {
	tokens := int64(100)
	cost := 0.5
	allTokens := []int64{500, 600, 700, 800, 900}
	allCosts := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "quiet" {
		t.Errorf("expected quiet, got %s", tag)
	}
}

func TestDeriveFocusTag_Quiet_OnlyTokensBelow(t *testing.T) {
	tokens := int64(100)
	cost := 5.0
	allTokens := []int64{500, 600, 700, 800, 900}
	allCosts := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag == "quiet" {
		t.Errorf("expected not quiet, got %s", tag)
	}
}

func TestDeriveFocusTag_NoTag_ZeroActivity(t *testing.T) {
	tokens := int64(0)
	cost := 0.0
	allTokens := []int64{500, 600, 700, 800, 900}
	allCosts := []float64{2.0, 4.0, 6.0, 8.0, 10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "--" {
		t.Errorf("expected --, got %s", tag)
	}
}

func TestDeriveFocusTag_SingleActiveDay(t *testing.T) {
	tokens := int64(1000)
	cost := 10.0
	allTokens := []int64{1000}
	allCosts := []float64{10.0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag == "spike" {
		t.Errorf("expected not spike for single day, got %s", tag)
	}
}

func TestDeriveFocusTag_AllZeroExceptOne(t *testing.T) {
	tokens := int64(500)
	cost := 5.0
	allTokens := []int64{500, 0, 0, 0, 0}
	allCosts := []float64{5.0, 0, 0, 0, 0}

	tag := deriveFocusTag(tokens, cost, allTokens, allCosts)
	if tag != "--" {
		t.Errorf("expected --, got %s", tag)
	}
}

func TestCalculateMedian_OddLength(t *testing.T) {
	values := []int64{1, 2, 3, 4, 5}
	median := calculateMedian(values)
	if median != 3.0 {
		t.Errorf("expected 3.0, got %v", median)
	}
}

func TestCalculateMedian_EvenLength(t *testing.T) {
	values := []int64{1, 2, 3, 4}
	median := calculateMedian(values)
	if median != 2.5 {
		t.Errorf("expected 2.5, got %v", median)
	}
}

func TestCalculateMedian_Empty(t *testing.T) {
	values := []int64{}
	median := calculateMedian(values)
	if median != 0 {
		t.Errorf("expected 0, got %v", median)
	}
}

func TestCalculateMedianFloat_OddLength(t *testing.T) {
	values := []float64{1.0, 2.0, 3.0, 4.0, 5.0}
	median := calculateMedianFloat(values)
	if median != 3.0 {
		t.Errorf("expected 3.0, got %v", median)
	}
}

func TestCalculateMedianFloat_EvenLength(t *testing.T) {
	values := []float64{1.0, 2.0, 3.0, 4.0}
	median := calculateMedianFloat(values)
	if median != 2.5 {
		t.Errorf("expected 2.5, got %v", median)
	}
}

func TestBuildMonthDailyReport_AggregatesDailyStats(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_a", "ses_work", time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_a", "msg_a", "ses_work", time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":500,"output":500}}`)
	insertMessage(t, db, "msg_b", "ses_work", time.Date(2026, time.March, 1, 11, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_b", "msg_b", "ses_work", time.Date(2026, time.March, 1, 11, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":500,"output":500},"cost":10.0}`)
	insertMessage(t, db, "msg_c", "ses_work", time.Date(2026, time.March, 15, 14, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_c", "msg_c", "ses_work", time.Date(2026, time.March, 15, 14, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":1000,"output":1000}}`)

	monthStart := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	report, err := buildMonthDailyReport(db, dir, monthStart)
	if err != nil {
		t.Fatal(err)
	}

	if report.MonthStart.Month() != time.March {
		t.Errorf("expected March, got %v", report.MonthStart.Month())
	}
	if report.ActiveDays != 2 {
		t.Errorf("expected 2 active days, got %d", report.ActiveDays)
	}
	if report.TotalMessages != 3 {
		t.Errorf("expected 3 total messages, got %d", report.TotalMessages)
	}
	if report.TotalTokens != 4000 {
		t.Errorf("expected 4000 total tokens, got %d", report.TotalTokens)
	}
	if report.TotalSessions != 1 {
		t.Errorf("expected 1 session, got %d", report.TotalSessions)
	}
	if len(report.Days) != 31 {
		t.Errorf("expected 31 days in report, got %d", len(report.Days))
	}
	if report.Days[0].Date.Day() != 31 || report.Days[len(report.Days)-1].Date.Day() != 1 {
		t.Errorf("expected newest-first day ordering, got first=%v last=%v", report.Days[0].Date, report.Days[len(report.Days)-1].Date)
	}

	var march1, march15 *DailySummary
	for i := range report.Days {
		day := &report.Days[i]
		switch day.Date.Day() {
		case 1:
			march1 = day
		case 15:
			march15 = day
		}
	}

	if march1 == nil || march15 == nil {
		t.Fatalf("expected populated summaries for March 1 and March 15, got %v", report.Days)
	}
	if march1.Messages != 2 || march1.Tokens != 2000 || march1.Sessions != 1 {
		t.Errorf("expected March 1 summary {messages:2 tokens:2000 sessions:1}, got %+v", *march1)
	}
	if march15.Messages != 1 || march15.Tokens != 2000 || march15.Sessions != 1 {
		t.Errorf("expected March 15 summary {messages:1 tokens:2000 sessions:1}, got %+v", *march15)
	}
}

func TestBuildMonthDailyReport_DeriveFocusTags(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_spike", "ses_work", time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_spike", "msg_spike", "ses_work", time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":5000,"output":5000}}`)
	insertMessage(t, db, "msg_med", "ses_work", time.Date(2026, time.March, 2, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_med", "msg_med", "ses_work", time.Date(2026, time.March, 2, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":1000,"output":1000}}`)
	insertMessage(t, db, "msg_low", "ses_work", time.Date(2026, time.March, 3, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_low", "msg_low", "ses_work", time.Date(2026, time.March, 3, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":100}}`)

	monthStart := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	report, err := buildMonthDailyReport(db, dir, monthStart)
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Days) != 31 {
		t.Errorf("expected 31 days, got %d", len(report.Days))
	}

	var march1 *DailySummary
	for i := range report.Days {
		if report.Days[i].Date.Day() == 1 {
			march1 = &report.Days[i]
			break
		}
	}
	if march1 == nil {
		t.Fatalf("expected March 1 summary in month report")
	}
	if march1.FocusTag != "spike" {
		t.Errorf("expected spike on March 1, got %s", march1.FocusTag)
	}
	if !report.Days[0].Date.After(report.Days[len(report.Days)-1].Date) {
		t.Errorf("days not sorted in reverse chronological order")
	}
}

func TestBuildMonthDailyReport_MonthBoundaries(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)

	insertMessage(t, db, "msg_march", "ses_work", time.Date(2026, time.March, 31, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_march", "msg_march", "ses_work", time.Date(2026, time.March, 31, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":100}}`)
	insertMessage(t, db, "msg_april", "ses_work", time.Date(2026, time.April, 1, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_april", "msg_april", "ses_work", time.Date(2026, time.April, 1, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":100,"output":100}}`)

	monthStart := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.Local)
	report, err := buildMonthDailyReport(db, dir, monthStart)
	if err != nil {
		t.Fatal(err)
	}

	if report.MonthStart.Day() != 1 || report.MonthStart.Month() != time.March {
		t.Errorf("expected normalized month start (March 1), got %v", report.MonthStart)
	}
	if report.TotalTokens != 200 {
		t.Errorf("expected only March tokens (200), got %d", report.TotalTokens)
	}
	if len(report.Days) != 31 {
		t.Errorf("expected all 31 days in March report, got %d", len(report.Days))
	}
	activeDays := 0
	for _, day := range report.Days {
		if day.Messages > 0 || day.Tokens > 0 || day.Cost > 0 {
			activeDays++
		}
	}
	if activeDays != 1 {
		t.Errorf("expected only 1 active day in March report, got %d", activeDays)
	}
}

func TestBuildMonthDailyReport_ProjectScopeFiltering(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir1 := filepath.Join(tmp, "work1")
	dir2 := filepath.Join(tmp, "work2")

	insertSession(t, db, "ses_work1", dir1)
	insertSession(t, db, "ses_work2", dir2)
	insertMessage(t, db, "msg_1", "ses_work1", time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_1", "msg_1", "ses_work1", time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":1000,"output":1000}}`)
	insertMessage(t, db, "msg_2", "ses_work2", time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local), `{"role":"assistant"}`)
	insertPart(t, db, "step_2", "msg_2", "ses_work2", time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local), `{"type":"step-finish","tokens":{"input":500,"output":500}}`)

	monthStart := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	report, err := buildMonthDailyReport(db, dir1, monthStart)
	if err != nil {
		t.Fatal(err)
	}
	if report.TotalTokens != 2000 {
		t.Errorf("expected 2000 tokens for dir1, got %d", report.TotalTokens)
	}
	if report.TotalSessions != 1 {
		t.Errorf("expected 1 session for dir1, got %d", report.TotalSessions)
	}

	report2, err := buildMonthDailyReport(db, dir2, monthStart)
	if err != nil {
		t.Fatal(err)
	}
	if report2.TotalTokens != 1000 {
		t.Errorf("expected 1000 tokens for dir2, got %d", report2.TotalTokens)
	}
}

func TestBuildMonthDailyReport_CurrentMonthExcludesFutureDates(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)
	today := time.Now().In(time.Local)
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	insertMessage(t, db, "msg_today", "ses_work", todayStart.Add(10*time.Hour), `{"role":"assistant"}`)
	insertPart(t, db, "step_today", "msg_today", "ses_work", todayStart.Add(10*time.Hour), `{"type":"step-finish","tokens":{"input":100,"output":200}}`)

	report, err := buildMonthDailyReport(db, dir, time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}

	if len(report.Days) != today.Day() {
		t.Fatalf("expected current-month report to include days only through today (%d), got %d", today.Day(), len(report.Days))
	}
	for _, day := range report.Days {
		if day.Date.After(todayStart) {
			t.Fatalf("expected no future dates in current-month report, got %v", day.Date)
		}
	}
}

func TestBuildMonthDailyReport_CurrentMonthSkipsFutureRowsWithinSameMonth(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)
	today := time.Now().In(time.Local)
	todayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	future := todayStart.AddDate(0, 0, 2).Add(9 * time.Hour)
	insertMessage(t, db, "msg_future", "ses_work", future, `{"role":"assistant","cost":5.0}`)
	insertPart(t, db, "step_future", "msg_future", "ses_work", future, `{"type":"step-finish","tokens":{"input":100,"output":200},"cost":1.0}`)

	report, err := buildMonthDailyReport(db, dir, time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	for _, day := range report.Days {
		if day.Date.After(todayStart) {
			t.Fatalf("expected future same-month rows to be ignored, got %v", day.Date)
		}
	}
	if report.TotalMessages != 0 || report.TotalTokens != 0 || report.TotalCost != 0 {
		t.Fatalf("expected future rows not to affect totals, got messages=%d tokens=%d cost=%v", report.TotalMessages, report.TotalTokens, report.TotalCost)
	}
}

func TestMonthReportVisibleEnd_ClampsCurrentMonthToTomorrow(t *testing.T) {
	now := time.Date(2026, time.April, 3, 15, 30, 0, 0, time.Local)
	monthStart := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)

	visibleEnd := monthReportVisibleEnd(monthStart, now)

	want := time.Date(2026, time.April, 4, 0, 0, 0, 0, time.Local)
	if !visibleEnd.Equal(want) {
		t.Fatalf("expected visible end %v, got %v", want, visibleEnd)
	}
}

func TestBuildMonthDailyReport_KeepsDayAndMonthCostConsistent(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)
	stamp := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.Local)
	insertMessage(t, db, "msg_a", "ses_work", stamp, `{"role":"assistant","cost":5.0}`)
	insertPart(t, db, "step_a", "msg_a", "ses_work", stamp, `{"type":"step-finish","cost":1.0,"tokens":{"input":100,"output":100}}`)

	report, err := buildMonthDailyReport(db, dir, time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}

	if report.TotalCost != 5.0 {
		t.Fatalf("expected month total cost 5.0 from message-level cost, got %v", report.TotalCost)
	}
	var march1 *DailySummary
	for i := range report.Days {
		if report.Days[i].Date.Day() == 1 {
			march1 = &report.Days[i]
			break
		}
	}
	if march1 == nil {
		t.Fatal("expected march 1 summary")
	}
	if march1.Cost != report.TotalCost {
		t.Fatalf("expected day cost %v to match report total cost %v", march1.Cost, report.TotalCost)
	}
}

func TestBuildYearMonthlyReport_AggregatesTwelveMonths(t *testing.T) {
	db, tmp := openStatsTestDB(t)

	dir := filepath.Join(tmp, "work")
	insertSession(t, db, "ses_work", dir)
	for i := 0; i < 12; i++ {
		stamp := time.Date(2025, time.April, 1, 10, 0, 0, 0, time.Local).AddDate(0, i, 0)
		msgID := fmt.Sprintf("msg_%02d", i)
		stepID := fmt.Sprintf("step_%02d", i)
		insertMessage(t, db, msgID, "ses_work", stamp, `{"role":"assistant"}`)
		insertPart(t, db, stepID, msgID, "ses_work", stamp, fmt.Sprintf(`{"type":"step-finish","cost":%.1f,"tokens":{"input":%d,"output":%d}}`, float64(i+1), 100*(i+1), 50*(i+1)))
	}

	report, err := buildYearMonthlyReport(db, dir, time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Months) != 12 {
		t.Fatalf("expected 12 months, got %d", len(report.Months))
	}
	if !report.Start.Equal(time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("expected start 2025-04, got %v", report.Start)
	}
	if !report.End.Equal(time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("expected end 2026-04, got %v", report.End)
	}
	if report.ActiveMonths != 12 {
		t.Fatalf("expected 12 active months, got %d", report.ActiveMonths)
	}
	if report.CurrentStreak != 12 || report.BestStreak != 12 {
		t.Fatalf("expected full active streaks, got current=%d best=%d", report.CurrentStreak, report.BestStreak)
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
