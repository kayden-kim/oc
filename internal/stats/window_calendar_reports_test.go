package stats

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

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
