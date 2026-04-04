package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestView_RendersTodayAndMetricsSections(t *testing.T) {
	report := stats.WindowReport{
		Start:                time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local),
		End:                  time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local),
		InputTokens:          5_700_000,
		OutputTokens:         237_000,
		CacheReadTokens:      82_900_000,
		ReasoningTokens:      75_000,
		Tokens:               88_912_000,
		Cost:                 38.54,
		Messages:             23,
		Sessions:             5,
		CodeLines:            352,
		ChangedFiles:         32_000,
		TotalAgentModelCalls: 42,
		TotalSubtasks:        23,
		TotalSkillCalls:      1,
		TotalToolCalls:       33,
		ActiveMinutes:        330,
	}
	report.HalfHourSlots[0] = 100
	report.HalfHourSlots[1] = 100
	report.HalfHourSlots[16] = 500
	report.HalfHourSlots[17] = 500
	report.HalfHourSlots[18] = 700
	report.HalfHourSlots[19] = 700
	report.HalfHourSlots[44] = 200

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.globalDaily = report
	model.globalDailyLoaded = true
	model.globalDailyDate = report.Start
	model.width = 100
	model.height = 30
	view := stripANSI(model.View().Content)

	if !strings.Contains(view, "Today") || !strings.Contains(view, "Metrics") {
		t.Fatalf("expected Today and Metrics sections, got %q", view)
	}
	if strings.Contains(view, "My Pulse") {
		t.Fatalf("did not expect My Pulse section, got %q", view)
	}
	if !strings.Contains(view, "• active 5.5/24h (streak 0.5h, best 2h)") {
		t.Fatalf("expected today active summary, got %q", view)
	}
	if !strings.Contains(view, "      00") || !strings.Contains(view, "22") {
		t.Fatalf("expected daily-style hourly axis, got %q", view)
	}
	for _, snippet := range []string{"tokens", "input", "output", "c.read", "c.write", "reasoning", "total", "cost", "hours", "sess", "msgs", "lines", "files", "agents", "skills", "tools", "5.7M", "237k", "82.9M", "75k", "88.9M", "$38.54", "5.5h", "23", "352", "32.0k", "42", "33"} {
		if !strings.Contains(view, snippet) {
			t.Fatalf("expected launcher snippet %q, got %q", snippet, view)
		}
	}
	if strings.Contains(view, "subagents") {
		t.Fatalf("did not expect subagents metric in launcher view, got %q", view)
	}
}

func TestView_RendersTodayPlaceholdersBeforeLauncherStatsLoad(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	view := stripANSI(model.View().Content)

	if !strings.Contains(view, "Today") || !strings.Contains(view, "Metrics") {
		t.Fatalf("expected Today and Metrics sections before launcher stats load, got %q", view)
	}
	if !strings.Contains(view, "• active --") {
		t.Fatalf("expected active placeholder before launcher stats load, got %q", view)
	}
	if strings.Contains(view, "My Pulse") {
		t.Fatalf("did not expect legacy launcher header, got %q", view)
	}
}

func TestInit_LoadsLauncherDailyWindowInsteadOfOverviewStats(t *testing.T) {
	globalStatsCalls := 0
	globalWindowCalls := 0
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).
		WithStatsLoaders(
			func() (stats.Report, error) {
				globalStatsCalls++
				return stats.Report{}, nil
			},
			nil,
			func(label string, start, end time.Time) (stats.WindowReport, error) {
				globalWindowCalls++
				if label != "Daily" {
					t.Fatalf("expected launcher to request Daily window, got %q", label)
				}
				if !startOfStatsDay(start).Equal(start) {
					t.Fatalf("expected launcher window to start at beginning of day, got %v", start)
				}
				if !end.Equal(start.AddDate(0, 0, 1)) {
					t.Fatalf("expected launcher window to span one day, got %v -> %v", start, end)
				}
				return stats.WindowReport{Label: label, Start: start, End: end, ActiveMinutes: 60}, nil
			},
			nil,
		)

	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected launcher init to request today window report")
	}
	msg := cmd()
	windowMsg, ok := msg.(windowReportLoadedMsg)
	if !ok {
		t.Fatalf("expected windowReportLoadedMsg, got %T", msg)
	}
	if windowMsg.label != "Daily" {
		t.Fatalf("expected Daily window report load, got %+v", windowMsg)
	}
	if globalStatsCalls != 0 {
		t.Fatalf("expected no overview stats load on launcher init, got %d", globalStatsCalls)
	}
	if globalWindowCalls != 1 {
		t.Fatalf("expected one launcher window load, got %d", globalWindowCalls)
	}
}

func TestUpdate_LauncherIgnoresStaleDailyMessageAndReloadsCurrentDay(t *testing.T) {
	reloads := 0
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).
		WithStatsLoaders(
			nil,
			nil,
			func(label string, start, end time.Time) (stats.WindowReport, error) {
				reloads++
				return stats.WindowReport{Label: label, Start: start, End: end}, nil
			},
			nil,
		)

	yesterday := startOfStatsDay(time.Now().AddDate(0, 0, -1))
	updated, cmd := model.Update(windowReportLoadedMsg{
		project: false,
		label:   "Daily",
		start:   yesterday,
		end:     yesterday.AddDate(0, 0, 1),
		report:  stats.WindowReport{Label: "Daily", Start: yesterday, End: yesterday.AddDate(0, 0, 1)},
	})
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected stale launcher daily message to trigger reload")
	}
	if model.globalDailyLoaded {
		t.Fatal("expected stale launcher daily message to be ignored")
	}
	if _, ok := cmd().(windowReportLoadedMsg); !ok {
		t.Fatalf("expected reload command to request current daily window, got %T", cmd())
	}
	if reloads != 1 {
		t.Fatalf("expected one reload after stale launcher message, got %d", reloads)
	}
}

func TestUpdate_LauncherScopeToggleReloadsStaleTodayCache(t *testing.T) {
	projectLoads := 0
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).
		WithStatsLoaders(
			nil,
			nil,
			nil,
			func(label string, start, end time.Time) (stats.WindowReport, error) {
				projectLoads++
				return stats.WindowReport{Label: label, Start: start, End: end}, nil
			},
		)
	model.projectDailyLoaded = true
	model.projectDailyDate = startOfStatsDay(time.Now())
	model.projectDailyUpdatedAt = time.Now().Add(-statsViewTTL - time.Minute)

	updated, cmd := model.Update(mockKeyMsg("g"))
	model = updated.(Model)
	if !model.projectScope {
		t.Fatal("expected scope toggle to enable project scope")
	}
	if cmd == nil {
		t.Fatal("expected stale project today cache to trigger reload on scope toggle")
	}
	if _, ok := cmd().(windowReportLoadedMsg); !ok {
		t.Fatalf("expected project scope toggle to load daily window, got %T", cmd())
	}
	if projectLoads != 1 {
		t.Fatalf("expected one project launcher reload, got %d", projectLoads)
	}
}

func TestUpdate_ReturningToLauncherReloadsStaleTodayCache(t *testing.T) {
	globalLoads := 0
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).
		WithStatsLoaders(
			func() (stats.Report, error) { return stats.Report{}, nil },
			nil,
			func(label string, start, end time.Time) (stats.WindowReport, error) {
				globalLoads++
				return stats.WindowReport{Label: label, Start: start, End: end}, nil
			},
			nil,
		)
	model.statsMode = true
	model.globalDailyLoaded = true
	model.globalDailyDate = startOfStatsDay(time.Now())
	model.globalDailyUpdatedAt = time.Now().Add(-statsViewTTL - time.Minute)

	updated, cmd := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if model.statsMode {
		t.Fatal("expected tab to return to launcher")
	}
	if cmd == nil {
		t.Fatal("expected stale launcher cache to reload when returning from stats")
	}
	if _, ok := cmd().(windowReportLoadedMsg); !ok {
		t.Fatalf("expected launcher return to load daily window, got %T", cmd())
	}
	if globalLoads != 1 {
		t.Fatalf("expected one launcher reload after leaving stats mode, got %d", globalLoads)
	}
}

func TestUpdate_TabSwitchesToStatsAndEscReturns(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if !model.statsMode {
		t.Fatal("expected stats mode after tab")
	}
	if !strings.Contains(model.View().Content, "Overview") {
		t.Fatalf("expected stats tab view, got %q", model.View().Content)
	}
	if strings.Contains(model.View().Content, "active days(30d)") || strings.Contains(model.View().Content, "current streak") || strings.Contains(model.View().Content, "best streak") || strings.Contains(model.View().Content, "5-week heatmap") {
		t.Fatalf("expected overview cleanup to remove duplicated summary block, got %q", model.View().Content)
	}
	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	if model.statsMode {
		t.Fatal("expected esc to return to launcher")
	}
}

func TestUpdate_TabLoadsOverviewForCurrentProjectScope(t *testing.T) {
	var globalOverviewLoads, projectOverviewLoads, globalWindowLoads, projectWindowLoads int
	projectOverview := stats.Report{
		TotalToolCalls:  5,
		UniqueToolCount: 1,
		TopTools:        []stats.UsageCount{{Name: "read", Count: 5}},
	}
	model := NewModel(
		[]PluginItem{{Name: "plugin-a"}},
		nil,
		nil,
		SessionItem{},
		stats.Report{},
		stats.Report{},
		config.StatsConfig{DefaultScope: "project"},
		testVersion,
		true,
	).WithStatsLoaders(
		func() (stats.Report, error) {
			globalOverviewLoads++
			return stats.Report{Days: make([]stats.Day, 30)}, nil
		},
		func() (stats.Report, error) {
			projectOverviewLoads++
			return projectOverview, nil
		},
		func(label string, start, end time.Time) (stats.WindowReport, error) {
			globalWindowLoads++
			return stats.WindowReport{Label: label, Start: start, End: end}, nil
		},
		func(label string, start, end time.Time) (stats.WindowReport, error) {
			projectWindowLoads++
			return stats.WindowReport{Label: label, Start: start, End: end}, nil
		},
	)

	if cmd := model.Init(); cmd == nil {
		t.Fatal("expected init to load launcher daily window for current project scope")
	} else {
		updated, _ := model.Update(cmd())
		model = updated.(Model)
	}
	if projectWindowLoads != 1 {
		t.Fatalf("expected one project launcher daily window load, got %d", projectWindowLoads)
	}
	if globalWindowLoads != 0 {
		t.Fatalf("expected no global launcher daily window load, got %d", globalWindowLoads)
	}

	updated, cmd := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if !model.statsMode {
		t.Fatal("expected stats mode after tab")
	}
	if cmd == nil {
		t.Fatal("expected entering stats overview to trigger current-scope overview load")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)

	if projectOverviewLoads != 1 {
		t.Fatalf("expected one project overview load after entering stats, got %d", projectOverviewLoads)
	}
	if globalOverviewLoads != 0 {
		t.Fatalf("expected no global overview load after entering stats, got %d", globalOverviewLoads)
	}
	if projectWindowLoads != 1 {
		t.Fatalf("expected no extra project launcher window loads after entering stats, got %d", projectWindowLoads)
	}
	if !model.projectStatsLoaded {
		t.Fatal("expected project overview to be marked loaded after handling the stats command")
	}
	if got := model.projectStats.TopTools; len(got) != 1 || got[0].Name != "read" || got[0].Count != 5 {
		t.Fatalf("expected loaded project overview to be applied to the model, got %#v", got)
	}
	view := stripANSI(model.View().Content)
	for _, snippet := range []string{"Tools (1)", "read"} {
		if !strings.Contains(view, snippet) {
			t.Fatalf("expected project overview snippet %q in stats view, got %q", snippet, view)
		}
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
	if !strings.Contains(stripANSI(model.View().Content), time.Now().Format("2006-01")) {
		t.Fatalf("expected daily tab content, got %q", model.View().Content)
	}
	updated, _ = model.Update(mockKeyMsg("left"))
	model = updated.(Model)
	if model.statsTab != 0 {
		t.Fatalf("expected stats tab 0, got %d", model.statsTab)
	}
}

func TestStatsContentLines_DailyLoadingShowsMonthHeaderOnly(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.statsMode = true
	model.statsTab = 1
	model.globalMonthDailyLoading = true
	model.dailyMonthAnchor = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	lines := model.statsContentLines()
	if len(lines) != 1 {
		t.Fatalf("expected single loading header line, got %+v", lines)
	}
	plain := stripANSI(lines[0])
	if !strings.Contains(plain, "2026-03") {
		t.Fatalf("expected loading header month label, got %q", plain)
	}
	if strings.Contains(plain, "active") || strings.Contains(plain, "streak") {
		t.Fatalf("expected no right-side loading meta, got %q", plain)
	}
}

func TestStatsContentLines_MonthlyLoadingShowsMonthHeaderOnly(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.statsMode = true
	model.statsTab = 2
	model.globalYearMonthlyLoading = true
	model.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	lines := model.statsContentLines()
	if len(lines) != 1 {
		t.Fatalf("expected single loading header line, got %+v", lines)
	}
	plain := stripANSI(lines[0])
	if !strings.Contains(plain, "2026-03") {
		t.Fatalf("expected loading header month label, got %q", plain)
	}
	if strings.Contains(plain, "active") || strings.Contains(plain, "streak") {
		t.Fatalf("expected no right-side loading meta, got %q", plain)
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
	lines := model.statsContentLines()
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "2026-03") {
		t.Fatalf("expected monthly detail loading header month label, got %q", plain)
	}
	if strings.Contains(plain, "Loading stats...") && !strings.Contains(plain, "2026-03") {
		t.Fatalf("expected monthly detail loading to route through month header helper, got %q", plain)
	}
	if strings.Contains(plain, "selected") || strings.Contains(plain, "active ") || strings.Contains(plain, "streak") {
		t.Fatalf("expected monthly detail loading header to omit right-side meta, got %q", plain)
	}
}

func TestRenderStatsTabs_ShowsUnderlineStyleTabsWithMetadata(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: start.AddDate(0, 0, i)}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.statsTab = 0

	rendered := model.renderStatsTabs()
	lines := strings.Split(rendered, "\n")

	if len(lines) != 2 {
		t.Fatalf("expected two-line tab render with underline row, got %d in %q", len(lines), rendered)
	}
	if got := lipgloss.Width(lines[0]); got != maxLayoutWidth {
		t.Fatalf("expected first tab row width %d, got %d in %q", maxLayoutWidth, got, lines[0])
	}
	if got := lipgloss.Width(lines[1]); got != maxLayoutWidth {
		t.Fatalf("expected underline row width %d, got %d in %q", maxLayoutWidth, got, lines[1])
	}
	for _, snippet := range []string{"Overview", "Daily", "Monthly", "GLOBAL"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in tab row, got %q", snippet, rendered)
		}
	}
	plainRendered := stripANSI(rendered)
	for _, snippet := range []string{"   Overview   ", "   Daily   ", "   Monthly   ", "| GLOBAL"} {
		if !strings.Contains(plainRendered, snippet) {
			t.Fatalf("expected padded snippet %q in tab row, got %q", snippet, rendered)
		}
	}
	if strings.Contains(plainRendered, "|") && !strings.Contains(plainRendered, "| GLOBAL") && !strings.Contains(plainRendered, "| PROJECT") {
		t.Fatalf("expected literal pipe only in monthly scope meta, got %q", rendered)
	}
	model.statsTab = 2
	monthlyRendered := model.renderStatsTabs()
	if !strings.Contains(stripANSI(monthlyRendered), "| GLOBAL") {
		t.Fatalf("expected monthly tab scope label in pipe format, got %q", monthlyRendered)
	}
	activeIndicator := statsTabIndicatorStyle.Render(strings.Repeat("▔", statsTabWidth))
	if !strings.Contains(lines[1], activeIndicator) {
		t.Fatalf("expected active underline indicator in %q", lines[1])
	}
	model.projectScope = true
	updated := model.renderStatsTabs()
	if rendered == updated {
		t.Fatal("expected tab row to change when scope changes")
	}
	if !strings.Contains(stripANSI(updated), "PROJECT") {
		t.Fatalf("expected project scope label, got %q", updated)
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
	rendered := model.renderStatsView()
	tabs := model.renderStatsTabs()
	if strings.Contains(rendered, tabs+"\n\n") {
		t.Fatalf("expected stats content to start immediately below tabs, got %q", rendered)
	}
	if !strings.Contains(rendered, tabs+"\n") {
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

	rendered := model.renderStatsTabs()
	plainRendered := stripANSI(rendered)
	if !strings.Contains(plainRendered, "| GLOBAL") {
		t.Fatalf("expected scope-only tab metadata, got %q", rendered)
	}
	if strings.Contains(plainRendered, "2026-03") {
		t.Fatalf("did not expect monthly date range in tab metadata, got %q", rendered)
	}
}

func TestRenderStatsTabs_NarrowLayoutStillShowsScopeMeta(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, i)}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.width = 60
	model.statsTab = 0
	rendered := stripANSI(model.renderStatsTabs())
	if !strings.Contains(rendered, "| GLOBAL") {
		t.Fatalf("expected narrow stats tabs to include scope meta, got %q", rendered)
	}
	lines := strings.Split(rendered, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected narrow stats tabs to render scope on a second line, got %q", rendered)
	}
}

func TestUpdate_GTogglesProjectScopeAndHeaders(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	updated, _ := model.Update(mockKeyMsg("g"))
	model = updated.(Model)
	if !model.projectScope {
		t.Fatal("expected project scope after g toggle")
	}
	view := model.View().Content
	if !strings.Contains(view, "Today") || !strings.Contains(view, "Metrics") {
		t.Fatalf("expected headers without project prefix, got %q", view)
	}
	if strings.Contains(view, "[Project] Today") || strings.Contains(view, "[Project] Metrics") {
		t.Fatalf("did not expect project-prefixed headers, got %q", view)
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
	view := model.View().Content
	if !strings.Contains(view, "Today") || !strings.Contains(view, "Metrics") {
		t.Fatalf("expected headers without project prefix from default scope, got %q", view)
	}
	if strings.Contains(view, "[Project] Today") || strings.Contains(view, "[Project] Metrics") {
		t.Fatalf("did not expect project-prefixed headers from default scope, got %q", view)
	}
}

func TestUpdate_StatsViewScrollsWithUpDown(t *testing.T) {
	model := openDailyStatsViewWithHeight(t, 12, 20)
	before := stripANSI(model.View().Content)
	beforeDate := model.dailySelectedDate
	updated, _ := model.Update(mockKeyMsg("down"))
	model = updated.(Model)
	after := stripANSI(model.View().Content)
	if before == after {
		t.Fatalf("expected stats view to change after scrolling down")
	}
	if model.dailySelectedDate.Equal(beforeDate) {
		t.Fatalf("expected selected day to change after moving down")
	}
}

func TestUpdate_StatsViewPageNavigationKeys(t *testing.T) {
	model := openDailyStatsViewWithHeight(t, 12, 24)
	initialSelection := model.dailySelectionIndex()
	updated, _ := model.Update(mockKeyMsg("pgdown"))
	model = updated.(Model)
	if model.statsOffset <= 0 {
		t.Fatalf("expected pgdown to prefer screen scroll, got offset %d", model.statsOffset)
	}
	if model.dailySelectionIndex() != initialSelection {
		t.Fatalf("expected pgdown not to move selection when screen scroll is available, got index %d want %d", model.dailySelectionIndex(), initialSelection)
	}
	updated, _ = model.Update(mockKeyMsg("end"))
	model = updated.(Model)
	if model.dailySelectionIndex() != len(model.currentMonthDaily().Days)-1 {
		t.Fatalf("expected end to jump to last day, got index %d", model.dailySelectionIndex())
	}
	updated, _ = model.Update(mockKeyMsg("home"))
	model = updated.(Model)
	if model.dailySelectionIndex() != 0 {
		t.Fatalf("expected home to reset selection to top, got %d", model.dailySelectionIndex())
	}
	updated, _ = model.Update(mockKeyMsg("ctrl+d"))
	model = updated.(Model)
	if model.statsOffset == 0 {
		t.Fatalf("expected ctrl+d to prefer screen scroll over selection movement")
	}
	updated, _ = model.Update(mockKeyMsg("home"))
	model = updated.(Model)
	if model.dailySelectionIndex() != 0 {
		t.Fatalf("expected home to reset selection after ctrl+d, got %d", model.dailySelectionIndex())
	}
	updated, _ = model.Update(mockKeyMsg("pgup"))
	model = updated.(Model)
	if model.dailySelectionIndex() != 0 {
		t.Fatalf("expected pgup at top to stay clamped, got %d", model.dailySelectionIndex())
	}
	updated, _ = model.Update(mockKeyMsg("ctrl+u"))
	model = updated.(Model)
	if model.dailySelectionIndex() != 0 {
		t.Fatalf("expected ctrl+u at top to stay clamped, got %d", model.dailySelectionIndex())
	}
}

func TestUpdate_MonthlyListPageKeysPreferScreenScroll(t *testing.T) {
	model := openMonthlyStatsViewWithHeight(t, 12)
	beforeSelection := model.monthlySelectionIndex()
	updated, _ := model.Update(mockKeyMsg("pgdown"))
	model = updated.(Model)
	if model.statsOffset <= 0 {
		t.Fatalf("expected monthly pgdown to scroll screen, got offset %d", model.statsOffset)
	}
	if model.monthlySelectionIndex() != beforeSelection {
		t.Fatalf("expected monthly pgdown not to move selection when screen scroll is available, got %d want %d", model.monthlySelectionIndex(), beforeSelection)
	}
	updated, _ = model.Update(mockKeyMsg("ctrl+d"))
	model = updated.(Model)
	if model.statsOffset <= 0 {
		t.Fatalf("expected monthly ctrl+d to keep using screen scroll, got offset %d", model.statsOffset)
	}
}

func TestModel_LoadsOnlyVisibleStatsViewAndCachesWithinTTL(t *testing.T) {
	var overviewLoads, monthLoads, dailyLoads int
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).
		WithStatsLoaders(
			func() (stats.Report, error) {
				overviewLoads++
				return stats.Report{Days: make([]stats.Day, 30)}, nil
			},
			func() (stats.Report, error) {
				overviewLoads++
				return stats.Report{Days: make([]stats.Day, 30)}, nil
			},
			func(label string, start, end time.Time) (stats.WindowReport, error) {
				dailyLoads++
				return stats.WindowReport{Label: label}, nil
			},
			func(label string, start, end time.Time) (stats.WindowReport, error) {
				dailyLoads++
				return stats.WindowReport{Label: label}, nil
			},
		).
		WithMonthDailyLoaders(
			func(time.Time) (stats.MonthDailyReport, error) {
				monthLoads++
				return stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)}, nil
			},
			func(time.Time) (stats.MonthDailyReport, error) {
				monthLoads++
				return stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)}, nil
			},
		)

	if cmd := model.Init(); cmd == nil {
		t.Fatal("expected init to load launcher daily window")
	} else {
		updated, _ := model.Update(cmd())
		model = updated.(Model)
	}
	if dailyLoads != 1 {
		t.Fatalf("expected one launcher daily window load, got %d", dailyLoads)
	}
	if overviewLoads != 0 {
		t.Fatalf("expected no overview load during launcher init, got %d", overviewLoads)
	}
	updated, cmd := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected entering stats overview to trigger overview load")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if overviewLoads != 1 {
		t.Fatalf("expected one overview load after entering stats, got %d", overviewLoads)
	}
	updated, cmd = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected daily tab to trigger month load")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if monthLoads != 1 {
		t.Fatalf("expected one month-daily load, got %d", monthLoads)
	}
	updated, cmd = model.Update(mockKeyMsg("left"))
	model = updated.(Model)
	updated, cmd = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("expected fresh daily cache to avoid reload")
	}
	if dailyLoads != 1 {
		t.Fatalf("expected launcher-only day window load before daily detail, got %d", dailyLoads)
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
	if model.dailyDetailMode {
		t.Fatal("expected esc to return to month list mode")
	}
	if !model.dailySelectedDate.Equal(selected) {
		t.Fatalf("expected selected date preserved after returning from detail, got %v want %v", model.dailySelectedDate, selected)
	}
}

func TestUpdate_DailyListCtrlUpDownScrollsScreenWithoutChangingSelection(t *testing.T) {
	model := openDailyStatsViewWithHeight(t, 12, 20)
	selected := model.dailySelectedDate
	if !model.statsListCanScreenScroll() {
		t.Fatal("expected daily stats list to require screen scrolling for this fixture")
	}
	updated, _ := model.Update(mockKeyMsg("ctrl+down"))
	model = updated.(Model)
	if model.statsOffset != 1 || model.dailyListOffset != 1 {
		t.Fatalf("expected ctrl+down to scroll screen by one line, got statsOffset=%d dailyListOffset=%d", model.statsOffset, model.dailyListOffset)
	}
	if !model.dailySelectedDate.Equal(selected) {
		t.Fatalf("expected ctrl+down to preserve selection, got %v want %v", model.dailySelectedDate, selected)
	}
	updated, _ = model.Update(mockKeyMsg("ctrl+up"))
	model = updated.(Model)
	if model.statsOffset != 0 || model.dailyListOffset != 0 {
		t.Fatalf("expected ctrl+up to scroll back to top, got statsOffset=%d dailyListOffset=%d", model.statsOffset, model.dailyListOffset)
	}
	if !model.dailySelectedDate.Equal(selected) {
		t.Fatalf("expected ctrl+up to preserve selection, got %v want %v", model.dailySelectedDate, selected)
	}
}

func TestUpdate_DailyMonthNavigationPreservesDayWhenPossible(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	model.dailyMonthAnchor = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	model.dailySelectedDate = time.Date(2026, time.March, 31, 0, 0, 0, 0, time.Local)
	updated, _ = model.Update(mockKeyMsg("["))
	model = updated.(Model)
	if got := model.currentDailyMonth(); got.Month() != time.February {
		t.Fatalf("expected previous month February, got %v", got)
	}
	if got := model.currentDailyDate().Day(); got != 28 {
		t.Fatalf("expected day clamped to Feb 28, got %d", got)
	}
}

func TestUpdate_DailyMonthNavigationDoesNotAdvancePastCurrentMonth(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	currentMonth := statsMonthStart(time.Now())
	model.dailyMonthAnchor = currentMonth
	model.dailySelectedDate = startOfStatsDay(time.Now())
	updated, _ = model.Update(mockKeyMsg("]"))
	model = updated.(Model)
	if got := model.currentDailyMonth(); !got.Equal(currentMonth) {
		t.Fatalf("expected current month to remain capped, got %v want %v", got, currentMonth)
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
	if model.monthlyDetailMode {
		t.Fatal("expected esc to return to monthly list mode")
	}
	if !model.monthlySelectedMonth.Equal(selected) {
		t.Fatalf("expected selected month preserved after returning from detail, got %v want %v", model.monthlySelectedMonth, selected)
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
		t.Fatalf("expected up to restore previous month, got %v want %v", model.monthlySelectedMonth, before)
	}
}

func TestUpdate_MonthlyDetailAcceptsMonthDailyForSelectedMonth(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.statsMode = true
	model.statsTab = 2
	model.monthlyDetailMode = true
	model.monthlySelectedMonth = time.Date(2025, time.December, 1, 0, 0, 0, 0, time.Local)
	model.dailyMonthAnchor = time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)
	model.dailySelectedDate = time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)
	msg := monthDailyReportLoadedMsg{project: false, monthStart: time.Date(2025, time.December, 1, 0, 0, 0, 0, time.Local), report: stats.MonthDailyReport{MonthStart: time.Date(2025, time.December, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)}}
	updated, _ := model.Update(msg)
	model = updated.(Model)
	if !model.globalMonthDailyLoaded {
		t.Fatal("expected selected monthly detail month-daily report to be accepted")
	}
	if !model.globalMonthDailyMonth.Equal(time.Date(2025, time.December, 1, 0, 0, 0, 0, time.Local)) {
		t.Fatalf("expected loaded month to track selected month, got %v", model.globalMonthDailyMonth)
	}
}

func TestRenderOverviewLines_GroupsPostMetricsIntoSections(t *testing.T) {
	report := stats.Report{
		TodayCost:               1.84,
		TodayTokens:             148000,
		TodaySessionMinutes:     95,
		TodayReasoningShare:     0.25,
		RecentReasoningShare:    0.18,
		ThirtyDayCost:           7.42,
		ThirtyDayTokens:         420000,
		ThirtyDaySessionMinutes: 765,
		TotalSubtasks:           11,
		TotalAgentModelCalls:    11,
		TotalToolCalls:          42,
		TotalSkillCalls:         7,
		UniqueProjectCount:      2,
		UniqueAgentCount:        3,
		UniqueAgentModelCount:   6,
		UniqueSkillCount:        2,
		UniqueToolCount:         9,
		HighestBurnDay:          stats.Day{Date: time.Now().AddDate(0, 0, -1), Cost: 12.34},
		MostEfficientDay:        stats.Day{Date: time.Now().AddDate(0, 0, -3), Cost: 0.42, Tokens: 25000},
		Days:                    make([]stats.Day, 30),
	}
	report.TopProjects = []stats.UsageCount{{Name: "/tmp/work-a", Amount: 280000, Cost: 4.20}, {Name: "/tmp/work-b", Amount: 140000, Cost: 2.10}}
	report.TotalProjectCost = 6.30
	setRankedUsageField(&report, "TopTools", []usageFixture{{"bash", 21}, {"read", 11}, {"edit", 8}, {"grep", 6}, {"write", 4}, {"glob", 2}})
	setRankedUsageField(&report, "TopSkills", []usageFixture{{"writing-plans", 5}, {"test-driven-development", 2}})
	setRankedUsageField(&report, "TopAgentModels", []usageFixture{{"explore\x00gpt-5.4", 4}, {"oracle\x00gpt-5.4", 2}, {"planner\x00claude-sonnet-4.5", 2}, {"review\x00gemini-2.5-pro", 1}, {"debug\x00o4-mini", 1}, {"legacy\x00claude-haiku-4.5", 1}})
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}
	report.TodayCodeLines = 150
	report.TodayChangedFiles = 7
	report.ThirtyDayCodeLines = 1820
	report.ThirtyDayChangedFiles = 84
	report.HighestCodeDay = stats.Day{Date: time.Now().AddDate(0, 0, -1), CodeLines: 190}
	report.HighestChangedFilesDay = stats.Day{Date: time.Now().AddDate(0, 0, -1), ChangedFiles: 9}
	report.Days[len(report.Days)-1].CodeLines = 150
	report.Days[len(report.Days)-1].ChangedFiles = 7
	report.Days[len(report.Days)-2].CodeLines = 190
	report.Days[len(report.Days)-2].ChangedFiles = 9
	report.WeekdayActiveCounts = [7]int{4, 4, 4, 3, 3, 3, 1}
	report.WeekdayAgentCounts = [7]int{4, 4, 4, 3, 3, 3, 1}
	report.LongestSessionDay = report.Days[len(report.Days)-1]

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	plainContent := stripANSI(content)

	for _, section := range []string{"Trends", "Models (0)", "Projects (2)", "Agents (6)", "Skills (2)", "Tools (9)"} {
		if !strings.Contains(plainContent, section) {
			t.Fatalf("expected %s section in overview, got %q", section, plainContent)
		}
	}
	if strings.Contains(content, "Extremes") {
		t.Fatalf("expected Extremes section to be removed, got %q", content)
	}
	if strings.Contains(content, "weekday pattern     ") || strings.Contains(content, "daily cost trend    ") || strings.Contains(content, "reasoning share     ") {
		t.Fatalf("expected old flat overview labels to be removed, got %q", content)
	}
	if strings.Contains(content, renderSubSectionHeader("Activity", habitSectionTitleStyle)) {
		t.Fatalf("expected old activity header to be replaced, got %q", content)
	}
	for _, snippet := range []string{defaultTextStyle.Render("• calls ") + statsValueTextStyle.Render("42"), defaultTextStyle.Render("• delegated ") + statsValueTextStyle.Render("11"), defaultTextStyle.Render("• unique ")} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected activity summary snippet %q to be removed, got %q", snippet, content)
		}
	}
	for _, snippet := range []string{"/tmp/work-a", "/tmp/work-b", "$4.20", "$2.10", "$6.30", "bash", "read", "write", "explore", "oracle", "debug", "gpt-5.4", "claude-haiku-4.5", "writing-plans", "test-driven-development", "provider", "cost", "share", "Total", "100%", "50%", "36%"} {
		if !strings.Contains(plainContent, snippet) {
			t.Fatalf("expected ranked activity snippet %q, got %q", snippet, plainContent)
		}
	}
	for _, snippet := range []string{"• 1 bash ", "• 2 read ", "• 1 explore ", "• 1 writing-plans "} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected ordinal prefixes to be removed, got %q", content)
		}
	}
	for _, snippet := range []string{"• hours ", "1.6h", "150 (79%)", "7 (78%)", "93k (max)", "95 (24%)", "today", "peak day", "30d total", "tokens", "tok/h", "lines", "files", "line/h", "(" + maxTokensPerHourDay(report.Days).Date.Format("2006-01-02") + ")", "420k", "1.8k", "84"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected hours snippet %q, got %q", snippet, content)
		}
	}
	if strings.Count(content, statsTableDividerLine(statsTableMaxWidth)) < 2 {
		t.Fatalf("expected header and section divider lines in overview, got %q", content)
	}
	if !strings.Contains(content, renderSubSectionHeader("Metrics", todaySectionTitleStyle)) {
		t.Fatalf("expected Metrics section in overview, got %q", content)
	}
	if strings.Contains(content, renderSubSectionHeader("Today", todaySectionTitleStyle)) {
		t.Fatalf("did not expect Today section in overview, got %q", content)
	}
	if !strings.Contains(content, defaultTextStyle.Render("tokens")) || !strings.Contains(content, defaultTextStyle.Render("tok/h")) || !strings.Contains(content, defaultTextStyle.Render("cost")) || !strings.Contains(content, defaultTextStyle.Render("hours")) || !strings.Contains(content, defaultTextStyle.Render("lines")) || !strings.Contains(content, defaultTextStyle.Render("files")) || !strings.Contains(content, defaultTextStyle.Render("line/h")) {
		t.Fatalf("expected metrics table rows, got %q", content)
	}
	if !strings.Contains(content, defaultTextStyle.Render("• lines ")) {
		t.Fatalf("expected lines trend row, got %q", content)
	}
	if !strings.Contains(content, defaultTextStyle.Render("• files ")) {
		t.Fatalf("expected files trend row, got %q", content)
	}
	metricsSection := strings.SplitN(strings.SplitN(plainContent, "Metrics", 2)[1], "Trends", 2)[0]
	if !(strings.Count(metricsSection, strings.Repeat("┈", 10)) >= 2 && strings.Index(metricsSection, "lines") < strings.LastIndex(metricsSection, strings.Repeat("┈", 10)) && strings.Index(metricsSection, "files") < strings.LastIndex(metricsSection, strings.Repeat("┈", 10)) && strings.LastIndex(metricsSection, strings.Repeat("┈", 10)) < strings.Index(metricsSection, "tok/h") && strings.Index(metricsSection, "tok/h") < strings.Index(metricsSection, "line/h")) {
		t.Fatalf("expected divider between summary and rate metrics in overview, got %q", metricsSection)
	}
	for _, snippet := range []string{"• high burn ", "• longest day ", "• code peak ", "• efficient day "} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected extremes snippet %q to be removed, got %q", snippet, content)
		}
	}
}

type usageFixture struct {
	Name  string
	Count int
}

func setRankedUsageField(report *stats.Report, fieldName string, usage []usageFixture) {
	value := reflect.ValueOf(report).Elem()
	field := value.FieldByName(fieldName)
	if !field.IsValid() || !field.CanSet() {
		return
	}
	items := reflect.MakeSlice(field.Type(), 0, len(usage))
	for _, item := range usage {
		entry := reflect.New(field.Type().Elem()).Elem()
		entry.FieldByName("Name").SetString(item.Name)
		entry.FieldByName("Count").SetInt(int64(item.Count))
		items = reflect.Append(items, entry)
	}
	field.Set(items)
}

func TestRenderOverviewLines_KeepsTrendsAsCompactList(t *testing.T) {
	report := stats.Report{TodayCost: 1.84, TodayTokens: 148000, TodaySessionMinutes: 95, TodayCodeLines: 150, TodayChangedFiles: 7, TodayReasoningShare: 0.25, RecentReasoningShare: 0.18, Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1, CodeLines: i + 2, ChangedFiles: i%5 + 1}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	trendsSection := strings.SplitN(content, renderSubSectionHeader("Models", habitSectionTitleStyle), 2)[0]
	for _, snippet := range []string{defaultTextStyle.Render("• tokens "), defaultTextStyle.Render("• cost "), defaultTextStyle.Render("• hours "), defaultTextStyle.Render("• lines "), defaultTextStyle.Render("• files "), defaultTextStyle.Render("• reasoning ")} {
		if !strings.Contains(trendsSection, snippet) {
			t.Fatalf("expected trends snippet %q, got %q", snippet, trendsSection)
		}
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• tokens ")+statsValueTextStyle.Render(" ")) || strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+statsValueTextStyle.Render(" ")) {
		t.Fatalf("expected trend rows to stay single-line, got %q", trendsSection)
	}
	if strings.Contains(trendsSection, renderColumn("• tokens ", "", 28)+renderColumn("• cost ", "", 28)) || strings.Contains(trendsSection, renderColumn("• hours ", "", 28)+renderColumn("• lines ", "", 28)) {
		t.Fatalf("expected trends to avoid two-column paired rows, got %q", trendsSection)
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+statsValueTextStyle.Render(renderValueTrend(report.Days, func(day stats.Day) float64 { return day.Cost }))) {
		t.Fatalf("expected cost trend label column to include fixed-width padding, got %q", trendsSection)
	}
	if !strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+defaultTextStyle.Render("   ")) {
		t.Fatalf("expected padded cost trend label column, got %q", trendsSection)
	}
}

func TestRenderOverviewLines_HidesReasoningWhenTrendsAreNotShown(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30), TodayReasoningShare: 0.2, RecentReasoningShare: 0.1}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 1000}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.width = 60
	content := strings.Join(model.renderOverviewLines(), "\n")
	if strings.Contains(stripANSI(content), "reasoning") {
		t.Fatalf("expected reasoning line hidden when trends are omitted, got %q", content)
	}
}

func TestRenderOverviewLines_OrdersTrendRowsAsRequested(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i + 1), SessionMinutes: i + 1, CodeLines: i + 2, ChangedFiles: i%4 + 1}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	trendsSection := strings.SplitN(content, renderSubSectionHeader("Models", habitSectionTitleStyle), 2)[0]
	positions := []int{strings.Index(trendsSection, defaultTextStyle.Render("• tokens ")), strings.Index(trendsSection, defaultTextStyle.Render("• cost ")), strings.Index(trendsSection, defaultTextStyle.Render("• hours ")), strings.Index(trendsSection, defaultTextStyle.Render("• lines ")), strings.Index(trendsSection, defaultTextStyle.Render("• files "))}
	for i, pos := range positions {
		if pos < 0 {
			t.Fatalf("expected trend row %d in %q", i, trendsSection)
		}
	}
	if !(positions[0] < positions[1] && positions[1] < positions[2] && positions[2] < positions[3] && positions[3] < positions[4]) {
		t.Fatalf("expected trend order tokens -> cost -> hours -> lines -> files, got %q", trendsSection)
	}
}

func TestRenderOverviewLines_IncludesModelActivitySection(t *testing.T) {
	report := stats.Report{TotalToolCalls: 42, UniqueToolCount: 9, TotalSubtasks: 11, TotalAgentModelCalls: 11, UniqueAgentCount: 3, UniqueAgentModelCount: 11, TotalModelTokens: 730, TotalModelCost: 73.0, UniqueModelCount: 12, TotalSkillCalls: 0, UniqueSkillCount: 0, Days: make([]stats.Day, 30), TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 3}, {Name: "oracle\x00claude-sonnet-4.5", Count: 2}, {Name: "planner\x00gemini-2.5-pro", Count: 1}}, TopModels: []stats.UsageCount{{Name: "openai\x00gpt-5.4", Amount: 120, Cost: 12.0}, {Name: "anthropic\x00claude-sonnet-4.5", Amount: 100, Cost: 10.0}, {Name: "google\x00gemini-2.5-pro", Amount: 90, Cost: 9.0}, {Name: "openrouter\x00qwen/qwen3-coder", Amount: 75, Cost: 7.5}, {Name: "azure\x00gpt-4.1", Amount: 65, Cost: 6.5}, {Name: "bedrock\x00claude-3.7-sonnet", Amount: 55, Cost: 5.5}, {Name: "vertex_ai\x00gemini-2.0-flash", Amount: 50, Cost: 5.0}, {Name: "copilot\x00gpt-4o", Amount: 45, Cost: 4.5}, {Name: "github_models\x00mistral-large", Amount: 40, Cost: 4.0}, {Name: "openai\x00o4-mini", Amount: 35, Cost: 3.5}, {Name: "anthropic\x00claude-haiku-4.5", Amount: 30, Cost: 3.0}}}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	modelSection := strings.SplitN(strings.SplitN(content, renderSubSectionHeader("Models (12)", habitSectionTitleStyle), 2)[1], renderSubSectionHeader("Agents (11)", habitSectionTitleStyle), 2)[0]
	plainContent := stripANSI(content)
	plainModelSection := stripANSI(modelSection)
	for _, snippet := range []string{"Models (12)", "730", "openai", "anthropic", "gpt-5.4", "claude-haiku-4.5", "Total", "$12.00", "$3.00", "$73.00", "16%", "100%"} {
		if !strings.Contains(plainContent, snippet) {
			t.Fatalf("expected model activity snippet %q, got %q", snippet, plainContent)
		}
	}
	for _, snippet := range []string{"provider", "tokens", "cost", "share"} {
		if !strings.Contains(plainModelSection, snippet) {
			t.Fatalf("expected model activity table header %q, got %q", snippet, plainModelSection)
		}
	}
	headerLine := strings.Split(strings.TrimLeft(plainModelSection, "\n"), "\n")[0]
	if strings.Contains(headerLine, "model") || strings.Contains(plainModelSection, "████") || strings.Contains(plainModelSection, "····") {
		t.Fatalf("expected cleaned model activity section, got %q", plainModelSection)
	}
	for _, snippet := range []string{"• tokens ", "• unique ", "• 1 gpt-5.4", "• 10 o4-mini"} {
		if strings.Contains(modelSection, snippet) {
			t.Fatalf("expected old model activity formatting to be removed, got %q", modelSection)
		}
	}
	if strings.Contains(modelSection, "11 claude-haiku-4.5") {
		t.Fatalf("expected model activity section to keep plain labels without ordinal prefixes, got %q", modelSection)
	}
}

func TestView_LauncherTodayGraphHidesOnNarrowWidths(t *testing.T) {
	report := stats.WindowReport{ActiveMinutes: 90}
	report.HalfHourSlots[10] = 100
	report.HalfHourSlots[11] = 100
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.globalDaily = report
	model.globalDailyLoaded = true
	model.globalDailyDate = startOfStatsDay(time.Now())
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	wideView := stripANSI(updated.(Model).View().Content)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 50, Height: 30})
	narrowView := stripANSI(updated.(Model).View().Content)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 35, Height: 30})
	tinyView := stripANSI(updated.(Model).View().Content)
	if !strings.Contains(wideView, "00") || !strings.Contains(wideView, "22") {
		t.Fatalf("expected wide launcher view to show hourly axis, got %q", wideView)
	}
	if strings.Contains(narrowView, "00") || strings.Contains(narrowView, "22") || strings.Contains(tinyView, "00") || strings.Contains(tinyView, "22") {
		t.Fatalf("expected narrow layouts to hide hourly axis, got narrow=%q tiny=%q", narrowView, tinyView)
	}
	if !strings.Contains(tinyView, "• active ") || !strings.Contains(tinyView, "Metrics") {
		t.Fatalf("expected tiny launcher view to keep core today summary, got %q", tinyView)
	}
}

func TestAvailableStatsRows_UsesCollapsedStatsChromeOnNarrowWidth(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	model.height = 12
	model.width = 35
	if got := model.availableStatsRows(); got != 6 {
		t.Fatalf("expected 6 visible rows with collapsed narrow stats chrome, got %d", got)
	}
}

func TestRenderWindowLines_GroupsSummaryCounts(t *testing.T) {
	report := stats.WindowReport{Label: "Daily", Start: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local), Messages: 12345, Sessions: 2345, Tokens: 987654, Cost: 1234.56}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderWindowLines(report), "\n")
	plain := stripANSI(content)
	for _, snippet := range []string{"Token Used", "2026-03-28 00:00 .. 2026-03-28 23:59", "Top Sessions", "12,345", "2,345", "988k", "$1,234.56"} {
		if !strings.Contains(plain, snippet) && !(snippet == "2026-03-28 00:00 .. 2026-03-28 23:59" && strings.Contains(plain, "2026-03-28 00:00 .. 2026-03-28 23:…")) {
			t.Fatalf("expected grouped window snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Contains(plain, "# Token Used") || strings.Contains(plain, "## Models") || strings.Contains(plain, "| Window") {
		t.Fatalf("expected overview-style window rendering without markdown headings or pipe tables, got %q", plain)
	}
}

func TestWindowSessionRows_GroupsMessageCounts(t *testing.T) {
	report := stats.WindowReport{TopSessions: []stats.SessionUsage{{ID: "ses_big", Messages: 12345}}}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	rows := model.windowSessionRows(report)
	if got := rows[0][2]; got != "12,345" {
		t.Fatalf("expected grouped session message count, got %q", got)
	}
}

func TestWindowSessionRows_DoesNotInsertMissingCurrentSessionRow(t *testing.T) {
	report := stats.WindowReport{TopSessions: []stats.SessionUsage{{ID: "ses_other", Messages: 1}}}
	model := NewModel(nil, nil, nil, SessionItem{ID: "ses_current"}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	rows := model.windowSessionRows(report)
	if len(rows) != 1 {
		t.Fatalf("expected only actual session rows, got %+v", rows)
	}
	if strings.Contains(strings.Join(rows[0], " "), "current session not") {
		t.Fatalf("did not expect synthetic current-session row, got %+v", rows)
	}
}

func TestRenderWindowLines_UsesCompactLayoutOnNarrowWidth(t *testing.T) {
	report := stats.WindowReport{Label: "Daily", Start: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local), Messages: 12345, Sessions: 2345, Tokens: 987654, Cost: 1234.56, Models: []stats.ModelUsage{{Model: "gpt-5.4-with-a-long-name", TotalTokens: 123456, Cost: 12.34}}, TopSessions: []stats.SessionUsage{{ID: "ses_abcdefghijklmnopqrstuvwxyz", Title: "Very long session title", Messages: 123, Tokens: 456789, Cost: 45.67}}}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.width = 35
	content := strings.Join(model.renderWindowLines(report), "\n")
	if got := maxRenderedLineWidth(content); got > 35 {
		t.Fatalf("expected compact window lines width <= 35, got %d in %q", got, stripANSI(content))
	}
	if strings.Contains(content, "| Window") {
		t.Fatalf("expected narrow window view to avoid wide tables, got %q", stripANSI(content))
	}
	for _, snippet := range []string{"Token Used", "window 2026-03-28 00:00 ..", "Top Sessions", "messages 12,345", "sessions 2,345", "tokens 988k", "cost $1,234.56"} {
		if !strings.Contains(stripANSI(content), snippet) {
			t.Fatalf("expected compact summary snippet %q, got %q", snippet, stripANSI(content))
		}
	}
}

func TestRenderValueTrend_HighlightsTodayCellLikeRhythm(t *testing.T) {
	days := []stats.Day{{Date: time.Now().AddDate(0, 0, -2), Cost: 1}, {Date: time.Now().AddDate(0, 0, -1), Cost: 2}, {Date: time.Now(), Cost: 3}}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	trend := renderValueTrend(days, func(day stats.Day) float64 { return day.Cost })
	normalTodayCell := lipgloss.NewStyle().Foreground(lipgloss.Color("#B8B8B8")).Render("█")
	highlightedTodayCell := model.renderHeatmapCell(stats.Day{Tokens: 5_000_000, AssistantMessages: 1}, true)
	if !strings.HasSuffix(trend, highlightedTodayCell) || strings.HasSuffix(trend, normalTodayCell) {
		t.Fatalf("expected today trend cell to use highlighted today styling, got %q", trend)
	}
}

func TestRenderHeatmapCell_TodayUsesDifferentColor(t *testing.T) {
	day := stats.Day{Tokens: 5_000_000, AssistantMessages: 1}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	normal := model.renderHeatmapCell(day, false)
	today := model.renderHeatmapCell(day, true)
	if normal == today {
		t.Fatalf("expected today heatmap cell to differ from normal cell: %q", today)
	}
	if !strings.Contains(today, "█") {
		t.Fatalf("expected high activity today cell to keep block rune, got %q", today)
	}
}

func TestActivityLevel_UsesTokenThresholds(t *testing.T) {
	cases := []struct {
		name string
		day  stats.Day
		want int
	}{{"inactive", stats.Day{}, 0}, {"low from activity", stats.Day{AssistantMessages: 1}, 1}, {"medium tokens", stats.Day{Tokens: 1_000_000}, 2}, {"high tokens", stats.Day{Tokens: 5_000_000}, 3}}
	for _, tc := range cases {
		model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
		if got := model.activityLevel(tc.day); got != tc.want {
			t.Fatalf("%s: expected level %d, got %d", tc.name, tc.want, got)
		}
	}
}

func TestActivityLevel_UsesConfiguredTokenThresholds(t *testing.T) {
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{MediumTokens: 2000, HighTokens: 5000}, testVersion, true)
	if got := model.activityLevel(stats.Day{Tokens: 1999, AssistantMessages: 1}); got != 1 {
		t.Fatalf("expected low activity below medium threshold, got %d", got)
	}
	if got := model.activityLevel(stats.Day{Tokens: 2000}); got != 2 {
		t.Fatalf("expected medium activity at configured threshold, got %d", got)
	}
	if got := model.activityLevel(stats.Day{Tokens: 5000}); got != 3 {
		t.Fatalf("expected high activity at configured threshold, got %d", got)
	}
}

func TestFormatCompactTokens_UsesMillions(t *testing.T) {
	if got := formatCompactTokens(999999); got != "1000k" {
		t.Fatalf("expected 1000k below one million boundary, got %q", got)
	}
	if got := formatCompactTokens(1_000_000); got != "1.0M" {
		t.Fatalf("expected 1.0M at one million boundary, got %q", got)
	}
	if got := formatCompactTokens(12_340_000); got != "12.3M" {
		t.Fatalf("expected 12.3M for millions, got %q", got)
	}
	if got := formatCurrency(1234.56); got != "$1,234.56" {
		t.Fatalf("expected grouped currency, got %q", got)
	}
}

func TestRenderUsageLines_GroupsRemainderIntoOthersAfterTop15(t *testing.T) {
	items := make([]stats.UsageCount, 0, 17)
	total := int64(0)
	for i := range 17 {
		count := 20 - i
		items = append(items, stats.UsageCount{Name: fmt.Sprintf("tool-%02d", i+1), Count: count})
		total += int64(count)
	}
	lines := (Model{}).renderUsageLines("count", items, total)
	if len(lines) != 20 {
		t.Fatalf("expected 20 usage lines including header/dividers/others/total, got %d", len(lines))
	}
}

func TestSparklineLevel(t *testing.T) {
	step := int64(100000) / 7
	for _, tt := range []struct {
		tokens int64
		want   int
	}{{0, 0}, {1, 1}, {step, 1}, {step + 1, 2}, {step * 2, 2}, {step*2 + 1, 3}, {step * 6, 6}, {step*6 + 1, 7}, {999999, 7}} {
		if got := sparklineLevel(tt.tokens, step); got != tt.want {
			t.Errorf("sparklineLevel(%d, %d) = %d, want %d", tt.tokens, step, got, tt.want)
		}
	}
}

func TestSparklineCell_Characters(t *testing.T) {
	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	for level := 0; level < 8; level++ {
		cell := sparklineCell(level, false, true)
		if !strings.ContainsRune(cell, chars[level]) {
			t.Errorf("level %d: expected char %c in output %q", level, chars[level], cell)
		}
	}
}

func TestSparklineCell_CurrentSlotHighlight(t *testing.T) {
	normal := sparklineCell(3, false, true)
	highlighted := sparklineCell(3, true, true)
	if normal == highlighted {
		t.Error("current slot should produce different styled output than normal slot")
	}
}

func TestRender24hSparkline_BasicRendering(t *testing.T) {
	var slots [48]int64
	slots[20] = 50000
	slots[21] = 200000
	report := stats.Report{Days: make([]stats.Day, 30), Rolling24hSlots: slots}
	cfg := config.StatsConfig{HighTokens: 5000000, MediumTokens: 1000000}
	m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
	m.width = 80
	result := m.render24hSparkline(report)
	if result == "" {
		t.Fatal("expected non-empty sparkline")
	}
}

func TestRender24hSparkline_UsesHourlyThreshold(t *testing.T) {
	var slots [48]int64
	slots[47] = 100000
	report := stats.Report{Rolling24hSlots: slots}
	cfg := config.StatsConfig{HighTokens: 4_800_000}
	m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
	m.width = 80
	result := m.render24hSparkline(report)
	if !strings.ContainsRune(result, '▂') || strings.ContainsRune(result, '▃') {
		t.Fatalf("expected hourly threshold low-bar rendering, got %q", result)
	}
}

func TestSparklineCell_UsesGrayPaletteForYesterdaySlots(t *testing.T) {
	cell := sparklineCell(3, false, false)
	if !strings.Contains(cell, "38;2;96;96;96") {
		t.Fatalf("expected yesterday sparkline cell to use gray palette, got %q", cell)
	}
}

func TestSparklineCell_UsesDarkerTodayPaletteForLowLevels(t *testing.T) {
	cell := sparklineCell(2, false, true)
	if !strings.Contains(cell, "38;2;86;54;0") {
		t.Fatalf("expected today sparkline cell to use darker orange low-level tone, got %q", cell)
	}
}

func TestRender24hSparklineAt_SplitsYesterdayAndTodayColors(t *testing.T) {
	var slots [48]int64
	now := time.Date(2026, time.March, 30, 10, 15, 0, 0, time.Local)
	yesterdayIndex := 0
	todayIndex := 47 - (now.Hour()*2 + now.Minute()/30)
	slots[yesterdayIndex] = 300000
	slots[todayIndex] = 300000
	report := stats.Report{Rolling24hSlots: slots}
	cfg := config.StatsConfig{HighTokens: 4_800_000}
	m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
	m.width = 80
	result := m.render24hSparklineAt(report, now)
	if !strings.Contains(result, "38;2;64;64;64") || !strings.Contains(result, "38;2;63;40;0") {
		t.Fatalf("expected split yesterday/today colors, got %q", result)
	}
}

func TestRender24hSparkline_WidthAdaptation(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30), Rolling24hSlots: [48]int64{}}
	cfg := config.StatsConfig{HighTokens: 5000000}
	for _, tt := range []struct{ width, wantLen int }{{80, 24}, {50, 0}, {30, 0}} {
		m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
		m.width = tt.width
		result := m.render24hSparkline(report)
		if tt.wantLen == 0 {
			if result != "" {
				t.Errorf("expected empty sparkline at width %d", tt.width)
			}
			continue
		}
		count := 0
		for _, r := range result {
			for _, sc := range sparklineChars {
				if r == sc {
					count++
					break
				}
			}
		}
		if count != tt.wantLen {
			t.Errorf("width %d: got %d sparkline chars, want %d", tt.width, count, tt.wantLen)
		}
	}
}

func TestView_RendersLauncherTodayWithDailyStyleGraph(t *testing.T) {
	var slots [48]int64
	slots[12] = 100000
	slots[13] = 100000
	slots[14] = 100000
	report := stats.WindowReport{HalfHourSlots: slots, ActiveMinutes: 90}
	cfg := config.StatsConfig{HighTokens: 5000000, MediumTokens: 1000000}
	m := NewModel([]PluginItem{{Name: "test-plugin", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, cfg, "test", false)
	m.globalDaily = report
	m.globalDailyLoaded = true
	m.globalDailyDate = startOfStatsDay(time.Now())
	m.width = 80
	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "Today") || !strings.Contains(view, "• active 1.5/24h (streak 1.5h)") || !strings.Contains(view, "00") || !strings.Contains(view, "22") {
		t.Fatalf("expected launcher today graph content, got %q", view)
	}
}

func TestView_LauncherSingleEventStillShowsActiveDuration(t *testing.T) {
	report := stats.WindowReport{ActiveMinutes: 0}
	report.HalfHourSlots[20] = 500
	m := NewModel([]PluginItem{{Name: "test-plugin", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, false)
	m.globalDaily = report
	m.globalDailyLoaded = true
	m.globalDailyDate = startOfStatsDay(time.Now())
	m.width = 80
	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "• active 0.0/24h") {
		t.Fatalf("expected single-event launcher activity to show zero-hour summary instead of placeholder, got %q", view)
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
