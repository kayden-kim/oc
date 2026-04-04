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
	report := stats.WindowReport{Start: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local), InputTokens: 5_700_000, OutputTokens: 237_000, CacheReadTokens: 82_900_000, ReasoningTokens: 75_000, Tokens: 88_912_000, Cost: 38.54, Messages: 23, Sessions: 5, CodeLines: 352, ChangedFiles: 32_000, TotalAgentModelCalls: 42, TotalSubtasks: 23, TotalSkillCalls: 1, TotalToolCalls: 33, ActiveMinutes: 330}
	report.HalfHourSlots[0], report.HalfHourSlots[1], report.HalfHourSlots[16], report.HalfHourSlots[17], report.HalfHourSlots[18], report.HalfHourSlots[19], report.HalfHourSlots[44] = 100, 100, 500, 500, 700, 700, 200
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.globalDaily, model.globalDailyLoaded, model.globalDailyDate, model.width, model.height = report, true, report.Start, 100, 30
	view := stripANSI(model.View().Content)
	if !strings.Contains(view, "Today") || !strings.Contains(view, "Metrics") || strings.Contains(view, "My Pulse") || !strings.Contains(view, "• active 5.5/24h (streak 0.5h, best 2h)") || !strings.Contains(view, "      00") || !strings.Contains(view, "22") {
		t.Fatalf("unexpected launcher today view: %q", view)
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

func TestRenderOverviewLines_GroupsPostMetricsIntoSections(t *testing.T) {
	// moved intact from prior split file
	report := stats.Report{TodayCost: 1.84, TodayTokens: 148000, TodaySessionMinutes: 95, TodayReasoningShare: 0.25, RecentReasoningShare: 0.18, ThirtyDayCost: 7.42, ThirtyDayTokens: 420000, ThirtyDaySessionMinutes: 765, TotalSubtasks: 11, TotalAgentModelCalls: 11, TotalToolCalls: 42, TotalSkillCalls: 7, UniqueProjectCount: 2, UniqueAgentCount: 3, UniqueAgentModelCount: 6, UniqueSkillCount: 2, UniqueToolCount: 9, HighestBurnDay: stats.Day{Date: time.Now().AddDate(0, 0, -1), Cost: 12.34}, MostEfficientDay: stats.Day{Date: time.Now().AddDate(0, 0, -3), Cost: 0.42, Tokens: 25000}, Days: make([]stats.Day, 30)}
	report.TopProjects = []stats.UsageCount{{Name: "/tmp/work-a", Amount: 280000, Cost: 4.20}, {Name: "/tmp/work-b", Amount: 140000, Cost: 2.10}}
	report.TotalProjectCost = 6.30
	setRankedUsageField(&report, "TopTools", []usageFixture{{"bash", 21}, {"read", 11}, {"edit", 8}, {"grep", 6}, {"write", 4}, {"glob", 2}})
	setRankedUsageField(&report, "TopSkills", []usageFixture{{"writing-plans", 5}, {"test-driven-development", 2}})
	setRankedUsageField(&report, "TopAgentModels", []usageFixture{{"explore\x00gpt-5.4", 4}, {"oracle\x00gpt-5.4", 2}, {"planner\x00claude-sonnet-4.5", 2}, {"review\x00gemini-2.5-pro", 1}, {"debug\x00o4-mini", 1}, {"legacy\x00claude-haiku-4.5", 1}})
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}
	report.TodayCodeLines, report.TodayChangedFiles, report.ThirtyDayCodeLines, report.ThirtyDayChangedFiles = 150, 7, 1820, 84
	report.HighestCodeDay, report.HighestChangedFilesDay = stats.Day{Date: time.Now().AddDate(0, 0, -1), CodeLines: 190}, stats.Day{Date: time.Now().AddDate(0, 0, -1), ChangedFiles: 9}
	report.Days[len(report.Days)-1].CodeLines, report.Days[len(report.Days)-1].ChangedFiles, report.Days[len(report.Days)-2].CodeLines, report.Days[len(report.Days)-2].ChangedFiles = 150, 7, 190, 9
	report.WeekdayActiveCounts, report.WeekdayAgentCounts, report.LongestSessionDay = [7]int{4, 4, 4, 3, 3, 3, 1}, [7]int{4, 4, 4, 3, 3, 3, 1}, report.Days[len(report.Days)-1]
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content, plainContent := strings.Join(model.renderOverviewLines(), "\n"), stripANSI(strings.Join(model.renderOverviewLines(), "\n"))
	for _, section := range []string{"Trends", "Models (0)", "Projects (2)", "Agents (6)", "Skills (2)", "Tools (9)"} {
		if !strings.Contains(plainContent, section) {
			t.Fatalf("expected %s section in overview, got %q", section, plainContent)
		}
	}
	if strings.Contains(content, "Extremes") {
		t.Fatalf("expected Extremes section to be removed, got %q", content)
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

func TestRenderOverviewLines_KeepsTrendsAsCompactList(t *testing.T) { /* moved intact */
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
	plainContent, plainModelSection := stripANSI(content), stripANSI(modelSection)
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
}
func TestRenderWindowLines_GroupsSummaryCounts(t *testing.T) {
	report := stats.WindowReport{Label: "Daily", Start: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local), Messages: 12345, Sessions: 2345, Tokens: 987654, Cost: 1234.56}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	plain := stripANSI(strings.Join(model.renderWindowLines(report), "\n"))
	for _, snippet := range []string{"Token Used", "Top Sessions", "12,345", "2,345", "988k", "$1,234.56"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected grouped window snippet %q, got %q", snippet, plain)
		}
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
}
func TestRenderWindowLines_UsesCompactLayoutOnNarrowWidth(t *testing.T) {
	report := stats.WindowReport{Label: "Daily", Start: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local), Messages: 12345, Sessions: 2345, Tokens: 987654, Cost: 1234.56, Models: []stats.ModelUsage{{Model: "gpt-5.4-with-a-long-name", TotalTokens: 123456, Cost: 12.34}}, TopSessions: []stats.SessionUsage{{ID: "ses_abcdefghijklmnopqrstuvwxyz", Title: "Very long session title", Messages: 123, Tokens: 456789, Cost: 45.67}}}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.width = 35
	content := strings.Join(model.renderWindowLines(report), "\n")
	if got := maxRenderedLineWidth(content); got > 35 {
		t.Fatalf("expected compact window lines width <= 35, got %d", got)
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
	if normal, today := model.renderHeatmapCell(day, false), model.renderHeatmapCell(day, true); normal == today {
		t.Fatalf("expected today heatmap cell to differ from normal cell: %q", today)
	}
}
func TestActivityLevel_UsesTokenThresholds(t *testing.T) {
	for _, tc := range []struct {
		name string
		day  stats.Day
		want int
	}{{"inactive", stats.Day{}, 0}, {"low from activity", stats.Day{AssistantMessages: 1}, 1}, {"medium tokens", stats.Day{Tokens: 1_000_000}, 2}, {"high tokens", stats.Day{Tokens: 5_000_000}, 3}} {
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
	if normal, highlighted := sparklineCell(3, false, true), sparklineCell(3, true, true); normal == highlighted {
		t.Error("current slot should produce different styled output than normal slot")
	}
}
func TestRender24hSparkline_BasicRendering(t *testing.T) {
	var slots [48]int64
	slots[20], slots[21] = 50000, 200000
	report := stats.Report{Days: make([]stats.Day, 30), Rolling24hSlots: slots}
	cfg := config.StatsConfig{HighTokens: 5000000, MediumTokens: 1000000}
	m := NewModel(nil, nil, nil, SessionItem{}, report, stats.Report{}, cfg, "test", false)
	m.width = 80
	if result := m.render24hSparkline(report); result == "" {
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
	yesterdayIndex, todayIndex := 0, 47-(now.Hour()*2+now.Minute()/30)
	slots[yesterdayIndex], slots[todayIndex] = 300000, 300000
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
