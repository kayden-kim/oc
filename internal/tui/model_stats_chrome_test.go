package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestUpdate_TabSwitchesToStatsAndEscReturns(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if !model.statsMode || !strings.Contains(model.View().Content, "Overview") {
		t.Fatalf("expected stats mode overview after tab, got %q", model.View().Content)
	}
	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	if model.statsMode {
		t.Fatal("expected esc to return to launcher")
	}
}
func TestUpdate_TabLoadsOverviewForCurrentProjectScope(t *testing.T) {
	var globalOverviewLoads, projectOverviewLoads, globalWindowLoads, projectWindowLoads int
	projectOverview := stats.Report{TotalToolCalls: 5, UniqueToolCount: 1, TopTools: []stats.UsageCount{{Name: "read", Count: 5}}}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{DefaultScope: "project"}, testVersion, true).WithStatsLoaders(func() (stats.Report, error) {
		globalOverviewLoads++
		return stats.Report{Days: make([]stats.Day, 30)}, nil
	}, func() (stats.Report, error) { projectOverviewLoads++; return projectOverview, nil }, func(label string, start, end time.Time) (stats.WindowReport, error) {
		globalWindowLoads++
		return stats.WindowReport{Label: label, Start: start, End: end}, nil
	}, func(label string, start, end time.Time) (stats.WindowReport, error) {
		projectWindowLoads++
		return stats.WindowReport{Label: label, Start: start, End: end}, nil
	})
	if cmd := model.Init(); cmd != nil {
		updated, _ := model.Update(cmd())
		model = updated.(Model)
	}
	updated, cmd := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if !model.statsMode || cmd == nil {
		t.Fatal("expected stats overview load after tab")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if projectOverviewLoads != 1 || globalOverviewLoads != 0 || projectWindowLoads != 1 || globalWindowLoads != 0 {
		t.Fatalf("unexpected current-scope overview loads: g=%d p=%d gw=%d pw=%d", globalOverviewLoads, projectOverviewLoads, globalWindowLoads, projectWindowLoads)
	}
}
func TestUpdate_LeftRightMovesStatsTabs(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	if model.statsTab != 1 {
		t.Fatalf("expected stats tab 1, got %d", model.statsTab)
	}
	updated, _ = model.Update(mockKeyMsg("left"))
	model = updated.(Model)
	if model.statsTab != 0 {
		t.Fatalf("expected stats tab 0, got %d", model.statsTab)
	}
}
func TestRenderStatsTabs_ShowsUnderlineStyleTabsWithMetadata(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: start.AddDate(0, 0, i)}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	rendered := model.renderStatsTabs()
	lines := strings.Split(rendered, "\n")
	if len(lines) != 2 || lipgloss.Width(lines[0]) != maxLayoutWidth || lipgloss.Width(lines[1]) != maxLayoutWidth {
		t.Fatalf("expected two-line stats tabs, got %q", rendered)
	}
}
func TestAvailableStatsRows_AccountsForTwoLineTabs(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	model.height = 12
	if got := model.availableStatsRows(); got != 5 {
		t.Fatalf("expected 5 visible rows after two-line tabs, got %d", got)
	}
}
func TestRenderStatsView_RemovesBlankLineBelowTabs(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 100}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	rendered, tabs := model.renderStatsView(), model.renderStatsTabs()
	if strings.Contains(rendered, tabs+"\n\n") || !strings.Contains(rendered, tabs+"\n") {
		t.Fatalf("expected content to follow tabs directly, got %q", rendered)
	}
}
func TestRenderStatsTabs_UsesWindowRangeForMonthlyTab(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, i)}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.statsTab = 2
	model.globalMonthly = stats.WindowReport{Label: "Monthly", Start: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)}
	plain := stripANSI(model.renderStatsTabs())
	if !strings.Contains(plain, "| GLOBAL") || strings.Contains(plain, "2026-03") {
		t.Fatalf("unexpected monthly tab metadata: %q", plain)
	}
}
func TestRenderStatsTabs_NarrowLayoutStillShowsScopeMeta(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, i)}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.width = 60
	rendered := stripANSI(model.renderStatsTabs())
	if !strings.Contains(rendered, "| GLOBAL") || len(strings.Split(rendered, "\n")) != 2 {
		t.Fatalf("expected narrow stats tabs to keep scope meta, got %q", rendered)
	}
}
func TestUpdate_GTogglesProjectScopeAndHeaders(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	updated, _ := model.Update(mockKeyMsg("g"))
	model = updated.(Model)
	if !model.projectScope {
		t.Fatal("expected project scope after g toggle")
	}
	updated, _ = model.Update(mockKeyMsg("g"))
	model = updated.(Model)
	if model.projectScope {
		t.Fatal("expected g to toggle back to global scope")
	}
}
func TestNewModel_UsesConfiguredDefaultProjectScope(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{DefaultScope: "project"}, testVersion, true)
	if !model.projectScope {
		t.Fatal("expected project scope from config default")
	}
}
func TestModel_LoadsOnlyVisibleStatsViewAndCachesWithinTTL(t *testing.T) {
	var overviewLoads, monthLoads, dailyLoads int
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).WithStatsLoaders(func() (stats.Report, error) { overviewLoads++; return stats.Report{Days: make([]stats.Day, 30)}, nil }, func() (stats.Report, error) { overviewLoads++; return stats.Report{Days: make([]stats.Day, 30)}, nil }, func(label string, start, end time.Time) (stats.WindowReport, error) {
		dailyLoads++
		return stats.WindowReport{Label: label}, nil
	}, func(label string, start, end time.Time) (stats.WindowReport, error) {
		dailyLoads++
		return stats.WindowReport{Label: label}, nil
	}).WithMonthDailyLoaders(func(time.Time) (stats.MonthDailyReport, error) {
		monthLoads++
		return stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)}, nil
	}, func(time.Time) (stats.MonthDailyReport, error) {
		monthLoads++
		return stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)}, nil
	})
	if cmd := model.Init(); cmd != nil {
		updated, _ := model.Update(cmd())
		model = updated.(Model)
	}
	updated, cmd := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected overview load after entering stats")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	updated, cmd = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected month-daily load after moving to daily tab")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if overviewLoads != 1 || monthLoads != 1 || dailyLoads != 1 {
		t.Fatalf("unexpected visible stats loads: overview=%d month=%d daily=%d", overviewLoads, monthLoads, dailyLoads)
	}
}
func TestAvailableStatsRows_UsesCollapsedStatsChromeOnNarrowWidth(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	model.height, model.width = 12, 35
	if got := model.availableStatsRows(); got != 6 {
		t.Fatalf("expected 6 visible rows with collapsed narrow stats chrome, got %d", got)
	}
}
func TestRenderStatsHelpLine_IncludesScrollNavigationTokens(t *testing.T) {
	helpLine := renderStatsHelpLine(maxLayoutWidth)
	for _, token := range []string{"↑/↓", "pgup/pgdn", "ctrl+u/d", "home/end", "tab", "g", "←/→", "esc"} {
		if !strings.Contains(helpLine, helpBgKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}
}
