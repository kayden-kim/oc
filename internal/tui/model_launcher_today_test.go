package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestView_RendersTodayAndMetricsSections(t *testing.T) {
	report := stats.WindowReport{Start: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local), InputTokens: 5_700_000, OutputTokens: 237_000, CacheReadTokens: 82_900_000, ReasoningTokens: 75_000, Tokens: 88_912_000, Cost: 38.54, Messages: 23, Sessions: 5, CodeLines: 352, ChangedFiles: 32_000, TotalAgentModelCalls: 42, TotalSubtasks: 23, TotalSkillCalls: 1, TotalToolCalls: 33, ActiveMinutes: 330}
	report.HalfHourSlots[0], report.HalfHourSlots[1], report.HalfHourSlots[16], report.HalfHourSlots[17], report.HalfHourSlots[18], report.HalfHourSlots[19], report.HalfHourSlots[44] = 100, 100, 500, 500, 700, 700, 200
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.globalDaily, model.globalDailyLoaded, model.globalDailyDate, model.width, model.height = report, true, report.Start, 100, 30
	view := stripANSI(model.View().Content)
	if !strings.Contains(view, "Today") || !strings.Contains(view, "Metrics") || strings.Contains(view, "My Pulse") || !strings.Contains(view, "• active 5.5/24h (streak 0.5h, best 2h)") || !strings.Contains(view, "      00") || !strings.Contains(view, "22") {
		t.Fatalf("unexpected launcher today view: %q", view)
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
	if !strings.Contains(view, "Today") || !strings.Contains(view, "Metrics") || !strings.Contains(view, "• active --") || strings.Contains(view, "My Pulse") {
		t.Fatalf("unexpected launcher placeholder view: %q", view)
	}
}

func TestInit_LoadsLauncherDailyWindowInsteadOfOverviewStats(t *testing.T) {
	globalStatsCalls, globalWindowCalls := 0, 0
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).WithStatsLoaders(func() (stats.Report, error) { globalStatsCalls++; return stats.Report{}, nil }, nil, func(label string, start, end time.Time) (stats.WindowReport, error) {
		globalWindowCalls++
		if label != "Daily" || !startOfStatsDay(start).Equal(start) || !end.Equal(start.AddDate(0, 0, 1)) {
			t.Fatalf("unexpected launcher daily request: %q %v %v", label, start, end)
		}
		return stats.WindowReport{Label: label, Start: start, End: end, ActiveMinutes: 60}, nil
	}, nil)
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected launcher init to request today window report")
	}
	msg, ok := cmd().(windowReportLoadedMsg)
	if !ok || msg.label != "Daily" || globalStatsCalls != 0 || globalWindowCalls != 1 {
		t.Fatalf("unexpected launcher init load result: %#v statsCalls=%d windowCalls=%d", msg, globalStatsCalls, globalWindowCalls)
	}
}

func TestUpdate_LauncherIgnoresStaleDailyMessageAndReloadsCurrentDay(t *testing.T) {
	reloads := 0
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).WithStatsLoaders(nil, nil, func(label string, start, end time.Time) (stats.WindowReport, error) {
		reloads++
		return stats.WindowReport{Label: label, Start: start, End: end}, nil
	}, nil)
	yesterday := startOfStatsDay(time.Now().AddDate(0, 0, -1))
	updated, cmd := model.Update(windowReportLoadedMsg{project: false, label: "Daily", start: yesterday, end: yesterday.AddDate(0, 0, 1), report: stats.WindowReport{Label: "Daily", Start: yesterday, End: yesterday.AddDate(0, 0, 1)}})
	model = updated.(Model)
	if cmd == nil || model.globalDailyLoaded {
		t.Fatal("expected stale launcher daily message to be ignored and reloaded")
	}
	if _, ok := cmd().(windowReportLoadedMsg); !ok || reloads != 1 {
		t.Fatalf("expected one reload after stale launcher message, got reloads=%d", reloads)
	}
}

func TestUpdate_LauncherScopeToggleReloadsStaleTodayCache(t *testing.T) {
	projectLoads := 0
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).WithStatsLoaders(nil, nil, nil, func(label string, start, end time.Time) (stats.WindowReport, error) {
		projectLoads++
		return stats.WindowReport{Label: label, Start: start, End: end}, nil
	})
	model.projectDailyLoaded, model.projectDailyDate, model.projectDailyUpdatedAt = true, startOfStatsDay(time.Now()), time.Now().Add(-statsViewTTL-time.Minute)
	updated, cmd := model.Update(mockKeyMsg("g"))
	model = updated.(Model)
	if !model.projectScope || cmd == nil {
		t.Fatal("expected project scope toggle to reload stale today cache")
	}
	if _, ok := cmd().(windowReportLoadedMsg); !ok || projectLoads != 1 {
		t.Fatalf("expected one project launcher reload, got %d", projectLoads)
	}
}

func TestUpdate_ReturningToLauncherReloadsStaleTodayCache(t *testing.T) {
	globalLoads := 0
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).WithStatsLoaders(func() (stats.Report, error) { return stats.Report{}, nil }, nil, func(label string, start, end time.Time) (stats.WindowReport, error) {
		globalLoads++
		return stats.WindowReport{Label: label, Start: start, End: end}, nil
	}, nil)
	model.statsMode, model.globalDailyLoaded, model.globalDailyDate, model.globalDailyUpdatedAt = true, true, startOfStatsDay(time.Now()), time.Now().Add(-statsViewTTL-time.Minute)
	updated, cmd := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if model.statsMode || cmd == nil {
		t.Fatal("expected tab to return to launcher and reload stale cache")
	}
	if _, ok := cmd().(windowReportLoadedMsg); !ok || globalLoads != 1 {
		t.Fatalf("expected one launcher reload, got %d", globalLoads)
	}
}

func TestView_LauncherTodayGraphHidesOnNarrowWidths(t *testing.T) {
	report := stats.WindowReport{ActiveMinutes: 90}
	report.HalfHourSlots[10], report.HalfHourSlots[11] = 100, 100
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.globalDaily, model.globalDailyLoaded, model.globalDailyDate = report, true, startOfStatsDay(time.Now())
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	wideView := stripANSI(updated.(Model).View().Content)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 50, Height: 30})
	narrowView := stripANSI(updated.(Model).View().Content)
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 35, Height: 30})
	tinyView := stripANSI(updated.(Model).View().Content)
	if !strings.Contains(wideView, "00") || !strings.Contains(wideView, "22") || strings.Contains(narrowView, "00") || strings.Contains(narrowView, "22") || strings.Contains(tinyView, "00") || strings.Contains(tinyView, "22") {
		t.Fatalf("unexpected launcher width adaptation: wide=%q narrow=%q tiny=%q", wideView, narrowView, tinyView)
	}
	if !strings.Contains(tinyView, "• active ") || !strings.Contains(tinyView, "Metrics") {
		t.Fatalf("expected tiny launcher view to keep core today summary, got %q", tinyView)
	}
}

func TestView_RendersLauncherTodayWithDailyStyleGraph(t *testing.T) {
	var slots [48]int64
	slots[12], slots[13], slots[14] = 100000, 100000, 100000
	report := stats.WindowReport{HalfHourSlots: slots, ActiveMinutes: 90}
	cfg := config.StatsConfig{HighTokens: 5000000, MediumTokens: 1000000}
	m := NewModel([]PluginItem{{Name: "test-plugin", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, cfg, "test", false)
	m.globalDaily, m.globalDailyLoaded, m.globalDailyDate, m.width = report, true, startOfStatsDay(time.Now()), 80
	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "Today") || !strings.Contains(view, "• active 1.5/24h (streak 1.5h)") || !strings.Contains(view, "00") || !strings.Contains(view, "22") {
		t.Fatalf("expected launcher today graph content, got %q", view)
	}
}

func TestView_LauncherSingleEventStillShowsActiveDuration(t *testing.T) {
	report := stats.WindowReport{ActiveMinutes: 0}
	report.HalfHourSlots[20] = 500
	m := NewModel([]PluginItem{{Name: "test-plugin", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, false)
	m.globalDaily, m.globalDailyLoaded, m.globalDailyDate, m.width = report, true, startOfStatsDay(time.Now()), 80
	view := stripANSI(m.View().Content)
	if !strings.Contains(view, "• active 0.0/24h") {
		t.Fatalf("expected single-event launcher activity to show zero-hour summary instead of placeholder, got %q", view)
	}
}
