package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestWindowModelColumns_UsesReasonHeaderAndExpandableNameColumn(t *testing.T) {
	columns := windowModelColumns()

	if len(columns) < 6 {
		t.Fatalf("expected model columns, got %#v", columns)
	}
	if !columns[0].Expand {
		t.Fatalf("expected first model column to absorb remaining width, got %#v", columns[0])
	}
	if columns[5].Header != "reason" {
		t.Fatalf("expected reasoning column header to be renamed to reason, got %q", columns[5].Header)
	}
}

func TestRenderMetricsLines_PutSessionsBeforeMessages(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.dailySelectedDate = time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local)
	m.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)

	daily := stats.MonthDailyReport{
		MonthStart:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		MonthEnd:      time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		TotalMessages: 30,
		TotalSessions: 10,
		TotalTokens:   1000,
		TotalCost:     12.34,
		Days: []stats.DailySummary{{
			Date:     time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local),
			Messages: 20,
			Sessions: 5,
			Tokens:   900,
			Cost:     10.11,
		}},
	}
	monthly := stats.YearMonthlyReport{
		TotalMessages: 30,
		TotalSessions: 10,
		TotalTokens:   1000,
		TotalCost:     12.34,
		Months: []stats.MonthlySummary{{
			MonthStart:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
			MonthEnd:      time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
			TotalMessages: 20,
			TotalSessions: 5,
			TotalTokens:   900,
			TotalCost:     10.11,
		}},
	}

	dailyPlain := stripANSI(strings.Join(m.renderMonthDailyMetricsLines(daily), "\n"))
	if strings.Index(dailyPlain, "sessions") > strings.Index(dailyPlain, "messages") {
		t.Fatalf("expected daily metrics to list sessions before messages, got %q", dailyPlain)
	}
	monthlyPlain := stripANSI(strings.Join(m.renderYearMonthlyMetricsLines(monthly), "\n"))
	if strings.Index(monthlyPlain, "sessions") > strings.Index(monthlyPlain, "messages") {
		t.Fatalf("expected monthly metrics to list sessions before messages, got %q", monthlyPlain)
	}
}

func TestRenderDailyDetailLines_UsesRequestedDetailLayout(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.height = 30
	m.dailySelectedDate = time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local)
	m.session = SessionItem{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유"}
	m.globalMonthDaily = stats.MonthDailyReport{Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 17, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 18, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 19, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 20, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 21, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 22, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 23, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local), Tokens: 1}}}
	m.globalMonthDailyLoaded = true
	report := stats.WindowReport{Label: "Daily", Start: time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 25, 0, 0, 0, 0, time.Local), Messages: 1259, Sessions: 2, Tokens: 215900000, Cost: 130.28, ActiveMinutes: 324, Models: []stats.ModelUsage{{Model: "openai\x00gpt-5.4", TotalTokens: 1000, Cost: 1.23}}, TopProjects: []stats.UsageCount{{Name: "/tmp/work-a", Amount: 3000, Cost: 4.56}}, TopAgents: []stats.UsageCount{{Name: "explore", Count: 2}}, TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 2}}, TopSkills: []stats.UsageCount{{Name: "writing-plans", Count: 1}}, TopTools: []stats.UsageCount{{Name: "bash", Count: 3}}, TotalProjectCost: 4.56, TotalSubtasks: 2, TotalAgentModelCalls: 2, TotalSkillCalls: 1, TotalToolCalls: 3, AllSessions: []stats.SessionUsage{{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유", Messages: 12, Tokens: 3000, Cost: 4.56}, {ID: "ses_2", Title: "다른 세션", Messages: 8, Tokens: 2000, Cost: 2.34}}, TopSessions: []stats.SessionUsage{{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유", Messages: 12, Tokens: 3000, Cost: 4.56}}}
	for i := 41; i <= 47; i++ {
		report.HalfHourSlots[i] = 1000
	}

	content := strings.Join(m.renderDailyDetailLines(report), "\n")
	plain := stripANSI(content)
	if !strings.Contains(content, detailSectionBarStyle.Render("┃")) {
		t.Fatalf("expected daily detail first header to use blue bar, got %q", content)
	}
	if strings.Contains(content, renderSubSectionHeader("2026-03-24", todaySectionTitleStyle)) {
		t.Fatalf("expected daily detail first header not to use orange bar, got %q", content)
	}
	for _, snippet := range []string{"2026-03-24", "5.4h active • streak 3.5h (best 8d)", "00", "22", "Models (1)", "Sessions (2)", "Projects (1)", "Agents (1)", "Skills (1)", "Tools (1)", "gpt-5.4", "CLI 앱이 brew cask인 이유", "다른 세션", "/tmp/work-a", "explore", "writing-plans", "bash", "model"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected detail layout snippet %q, got %q", snippet, plain)
		}
	}
	if !strings.Contains(plain, "Total") {
		t.Fatalf("expected total rows in detail tables, got %q", plain)
	}
	if !strings.Contains(plain, "oa| gpt-5.4") {
		t.Fatalf("expected daily detail model label to include provider abbreviation, got %q", plain)
	}
	if !strings.Contains(plain, "*    ses_1") && !strings.Contains(plain, "*        ses_1") && !strings.Contains(plain, "*  ses_1") {
		t.Fatalf("expected current session marker in daily detail sessions table, got %q", plain)
	}
	for _, removed := range []string{"global • back to", "messages 1259 | sessions 70 | tokens", "top model ", "most expensive session ", "most active session ", "average tokens per session", "Top Sessions", "current session not in selected window"} {
		if strings.Contains(plain, removed) {
			t.Fatalf("did not expect old detail snippet %q in %q", removed, plain)
		}
	}
}

func TestRenderDailyDetailLines_OmitsProjectsInProjectScope(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.height = 30
	m.projectScope = true
	m.dailySelectedDate = time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local)
	m.globalMonthDaily = stats.MonthDailyReport{Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local), Tokens: 1}}}
	m.globalMonthDailyLoaded = true
	report := stats.WindowReport{Label: "Daily", Start: time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 25, 0, 0, 0, 0, time.Local), TopProjects: []stats.UsageCount{{Name: "/tmp/work-a", Amount: 3000, Cost: 4.56}}, TopAgents: []stats.UsageCount{{Name: "explore", Count: 2}}, TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 2}}, TotalAgentModelCalls: 2, TotalSubtasks: 2}
	plain := stripANSI(strings.Join(m.renderDailyDetailLines(report), "\n"))
	if strings.Contains(plain, "Projects (") {
		t.Fatalf("expected Projects table omitted in project scope, got %q", plain)
	}
	if !strings.Contains(plain, "Agents (1)") {
		t.Fatalf("expected other activity tables to remain, got %q", plain)
	}
}

func TestWindowModelDisplayName_UsesProviderAbbreviationPrefix(t *testing.T) {
	if got := windowModelDisplayName("openai\x00gpt-5.4"); got != "oa| gpt-5.4" {
		t.Fatalf("expected provider abbreviation prefix, got %q", got)
	}
	if got := windowModelDisplayName("gpt-5.4"); got != "gpt-5.4" {
		t.Fatalf("expected plain model unchanged, got %q", got)
	}
}

func TestRenderDailyDetailHourlyLines_ShiftsGraphLeftByFourSpaces(t *testing.T) {
	m := newStatsTestModel()
	report := stats.WindowReport{}
	axis := stripANSI(m.renderDailyDetailAxisLine())
	spark := stripANSI(m.renderDailyDetailSparkline(report))
	if !strings.HasPrefix(axis, "      00") {
		t.Fatalf("expected axis line to start two columns further right, got %q", axis)
	}
	if !strings.HasPrefix(spark, "      ") {
		t.Fatalf("expected sparkline to start two columns further right, got %q", spark)
	}
}

func TestCurrentTrailingActiveSlots(t *testing.T) {
	var slots [48]int64
	slots[40] = 1
	slots[41] = 1
	slots[45] = 1
	slots[46] = 1
	slots[47] = 1

	if got := currentTrailingActiveSlots(slots); got != 3 {
		t.Fatalf("currentTrailingActiveSlots() = %d, want 3", got)
	}
}

func TestRenderDetailModeHelpLine_MatchesDetailNavigationCopy(t *testing.T) {
	plain := stripANSI(renderDetailModeHelpLine(80))
	for _, snippet := range []string{"↑/↓", "scroll", "pgup/pgdn", "page", "ctrl+u/d", "half", "home/end", "top/bottom", "esc", "month list", "g", "scope", "←/→", "tabs", "tab", "launcher"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected detail help snippet %q, got %q", snippet, plain)
		}
	}
}

func TestRenderHalfHourSparkline_HighlightsCurrentSlotWhenToday(t *testing.T) {
	now := time.Date(2026, time.April, 2, 10, 30, 0, 0, time.Local)
	var slots [48]int64
	slots[21] = 1000
	line := renderHalfHourSparkline(slots, now, true)
	expected := lipgloss.NewStyle().Foreground(lipgloss.Color(currentHalfHourHighlightColor)).Render(string(sparklineChars[7]))

	if !strings.Contains(line, expected) {
		t.Fatalf("expected current half-hour slot to be highlighted white, got %q", line)
	}
}

func TestRenderHalfHourSparkline_DoesNotHighlightCurrentSlotWhenNotToday(t *testing.T) {
	now := time.Date(2026, time.April, 2, 10, 30, 0, 0, time.Local)
	var slots [48]int64
	slots[21] = 1000
	line := renderHalfHourSparkline(slots, now, false)
	unexpected := lipgloss.NewStyle().Foreground(lipgloss.Color(currentHalfHourHighlightColor)).Render(string(sparklineChars[7]))

	if strings.Contains(line, unexpected) {
		t.Fatalf("did not expect white current-slot highlight for non-today data, got %q", line)
	}
}

func TestFocusTagStyle_ReturnsConsistentStyle(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)

	for _, tt := range []struct{ tag string }{{"spike"}, {"heavy"}, {"quiet"}, {"--"}} {
		style := m.focusTagStyle(tt.tag)
		rendered := style.Render("test")
		if len(rendered) == 0 {
			t.Fatalf("expected non-empty rendered output for tag %q", tt.tag)
		}
	}
}
