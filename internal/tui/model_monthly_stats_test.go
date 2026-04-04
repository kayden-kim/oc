package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestStatsContentLines_MonthlyLoadingShowsMonthHeaderOnly(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.statsMode = true
	model.statsTab = 2
	model.globalYearMonthlyLoading = true
	model.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	lines := model.statsContentLines()
	if len(lines) != 1 || !strings.Contains(stripANSI(lines[0]), "2026-03") {
		t.Fatalf("expected monthly loading header, got %+v", lines)
	}
}
func TestStatsContentLines_MonthlyDetailLoadingShowsMonthHeaderOnly(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.statsMode = true
	model.statsTab = 2
	model.monthlyDetailMode = true
	model.globalMonthlyLoading = true
	model.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	model.globalYearMonthly = stats.YearMonthlyReport{Months: []stats.MonthlySummary{{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)}}}
	plain := stripANSI(strings.Join(model.statsContentLines(), "\n"))
	if !strings.Contains(plain, "2026-03") {
		t.Fatalf("expected monthly detail loading header, got %q", plain)
	}
}
func TestUpdate_MonthlyListPageKeysPreferScreenScroll(t *testing.T) {
	model := openMonthlyStatsViewWithHeight(t, 12)
	beforeSelection := model.monthlySelectionIndex()
	updated, _ := model.Update(mockKeyMsg("pgdown"))
	model = updated.(Model)
	if model.statsOffset <= 0 || model.monthlySelectionIndex() != beforeSelection {
		t.Fatalf("expected monthly pgdown to scroll screen without changing selection")
	}
}
func TestUpdate_MonthlyEnterAndEscSwitchBetweenListAndDetail(t *testing.T) {
	model := openMonthlyStatsViewWithHeight(t, 14)
	selected := model.monthlySelectedMonth
	updated, _ := model.Update(mockKeyMsg("enter"))
	model = updated.(Model)
	if !model.monthlyDetailMode {
		t.Fatal("expected enter to open monthly detail mode")
	}
	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	if model.monthlyDetailMode || !model.monthlySelectedMonth.Equal(selected) {
		t.Fatalf("expected esc to return from monthly detail preserving selection")
	}
}
func TestUpdate_MonthlySelectionMovesWithUpDown(t *testing.T) {
	model := openMonthlyStatsViewWithHeight(t, 12)
	before := model.monthlySelectedMonth
	updated, _ := model.Update(mockKeyMsg("down"))
	model = updated.(Model)
	if model.monthlySelectedMonth.Equal(before) {
		t.Fatalf("expected selected month to change after moving down")
	}
	updated, _ = model.Update(mockKeyMsg("up"))
	model = updated.(Model)
	if !model.monthlySelectedMonth.Equal(before) {
		t.Fatalf("expected up to restore previous month")
	}
}
func TestUpdate_MonthlyDetailAcceptsMonthDailyForSelectedMonth(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.statsMode, model.statsTab, model.monthlyDetailMode = true, 2, true
	model.monthlySelectedMonth = time.Date(2025, time.December, 1, 0, 0, 0, 0, time.Local)
	model.dailyMonthAnchor, model.dailySelectedDate = time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)
	msg := monthDailyReportLoadedMsg{project: false, monthStart: time.Date(2025, time.December, 1, 0, 0, 0, 0, time.Local), report: stats.MonthDailyReport{MonthStart: time.Date(2025, time.December, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)}}
	updated, _ := model.Update(msg)
	model = updated.(Model)
	if !model.globalMonthDailyLoaded || !model.globalMonthDailyMonth.Equal(time.Date(2025, time.December, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("expected selected monthly detail month-daily report to be accepted")
	}
}
