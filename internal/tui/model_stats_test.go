package tui

import (
	"strings"
	"testing"
	"time"

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
