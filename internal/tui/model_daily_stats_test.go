package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestStatsContentLines_DailyLoadingShowsMonthHeaderOnly(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.statsMode = true
	model.statsTab = 1
	model.globalMonthDailyLoading = true
	model.dailyMonthAnchor = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	lines := model.statsContentLines()
	if len(lines) != 1 || !strings.Contains(stripANSI(lines[0]), "2026-03") {
		t.Fatalf("expected single daily loading header, got %+v", lines)
	}
}
func TestUpdate_StatsViewScrollsWithUpDown(t *testing.T) {
	model := openDailyStatsViewWithHeight(t, 12, 20)
	before, beforeDate := stripANSI(model.View().Content), model.dailySelectedDate
	updated, _ := model.Update(mockKeyMsg("down"))
	model = updated.(Model)
	after := stripANSI(model.View().Content)
	if before == after || model.dailySelectedDate.Equal(beforeDate) {
		t.Fatalf("expected daily stats view to change after scrolling down")
	}
}
func TestUpdate_StatsViewPageNavigationKeys(t *testing.T) {
	model := openDailyStatsViewWithHeight(t, 12, 24)
	initialSelection := model.dailySelectionIndex()
	updated, _ := model.Update(mockKeyMsg("pgdown"))
	model = updated.(Model)
	if model.statsOffset <= 0 || model.dailySelectionIndex() != initialSelection {
		t.Fatalf("expected pgdown to prefer screen scroll, got offset=%d index=%d", model.statsOffset, model.dailySelectionIndex())
	}
}
func TestUpdate_DailyEnterAndEscSwitchBetweenListAndDetail(t *testing.T) {
	model := openDailyStatsViewWithHeight(t, 12, 8)
	selected := model.dailySelectedDate
	updated, _ := model.Update(mockKeyMsg("enter"))
	model = updated.(Model)
	if !model.dailyDetailMode {
		t.Fatal("expected enter to open daily detail mode")
	}
	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	if model.dailyDetailMode || !model.dailySelectedDate.Equal(selected) {
		t.Fatalf("expected esc to return from daily detail preserving selection")
	}
}
func TestUpdate_DailyListCtrlUpDownScrollsScreenWithoutChangingSelection(t *testing.T) {
	model := openDailyStatsViewWithHeight(t, 12, 20)
	selected := model.dailySelectedDate
	updated, _ := model.Update(mockKeyMsg("ctrl+down"))
	model = updated.(Model)
	if model.statsOffset != 1 || model.dailyListOffset != 1 || !model.dailySelectedDate.Equal(selected) {
		t.Fatalf("expected ctrl+down to scroll daily screen without changing selection")
	}
}
func TestUpdate_DailyMonthNavigationPreservesDayWhenPossible(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	model.dailyMonthAnchor, model.dailySelectedDate = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), time.Date(2026, time.March, 31, 0, 0, 0, 0, time.Local)
	updated, _ = model.Update(mockKeyMsg("["))
	model = updated.(Model)
	if got := model.currentDailyMonth(); got.Month() != time.February || model.currentDailyDate().Day() != 28 {
		t.Fatalf("expected day clamp when moving to prior month, got month=%v day=%d", got, model.currentDailyDate().Day())
	}
}
func TestUpdate_DailyMonthNavigationDoesNotAdvancePastCurrentMonth(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	currentMonth := statsMonthStart(time.Now())
	model.dailyMonthAnchor, model.dailySelectedDate = currentMonth, startOfStatsDay(time.Now())
	updated, _ = model.Update(mockKeyMsg("]"))
	model = updated.(Model)
	if got := model.currentDailyMonth(); !got.Equal(currentMonth) {
		t.Fatalf("expected current month to remain capped, got %v want %v", got, currentMonth)
	}
}
