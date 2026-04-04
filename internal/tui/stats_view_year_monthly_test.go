package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestRenderYearMonthlyLines_FormatsNumericGridAndMetrics(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.height = 30
	m.statsTab = 2
	m.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	report := stats.YearMonthlyReport{Start: time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), ActiveMonths: 10, CurrentStreak: 4, BestStreak: 5}
	for i := 0; i < 12; i++ {
		month := report.Start.AddDate(0, i, 0)
		tokens := int64((i + 1) * 1_000_000)
		if i == 3 {
			tokens = 0
		}
		item := stats.MonthlySummary{MonthStart: month, MonthEnd: month.AddDate(0, 1, 0), ActiveDays: i + 1, TotalMessages: (i + 1) * 100, TotalSessions: (i + 1) * 10, TotalTokens: tokens, TotalCost: float64(i+1) * 9.25}
		report.Months = append(report.Months, item)
		report.TotalMessages += item.TotalMessages
		report.TotalSessions += item.TotalSessions
		report.TotalTokens += item.TotalTokens
		report.TotalCost += item.TotalCost
	}
	content := strings.Join(m.renderYearMonthlyLines(report), "\n")
	plain := stripANSI(content)
	for _, snippet := range []string{"2025-04 .. 2026-03", "active 10/12mo • streak 4mo (best 5mo)", "04", "03", "Metrics", "peak month", "2026-03", "messages", "sessions", "tokens", "cost"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected yearly monthly snippet %q, got %q", snippet, plain)
		}
	}
	if !strings.Contains(content, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900")).Bold(true).Render("03 ■■■■")) {
		t.Fatalf("expected selected month to use orange emphasis, got %q", content)
	}
	for _, row := range strings.Split(content, "\n") {
		if width := lipgloss.Width(stripANSI(row)); width > m.layoutWidth() {
			t.Fatalf("expected monthly view width <= %d, got %d in %q", m.layoutWidth(), width, stripANSI(row))
		}
	}
}

func TestRenderCompactYearMonthlyLines_UsesActiveCurrentTotalMonthFormat(t *testing.T) {
	m := newStatsTestModel()
	m.width = 60
	m.height = 24
	m.statsTab = 2
	report := stats.YearMonthlyReport{Start: time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), ActiveMonths: 6, CurrentStreak: 3, BestStreak: 4}
	for i := 0; i < 12; i++ {
		month := report.Start.AddDate(0, i, 0)
		report.Months = append(report.Months, stats.MonthlySummary{MonthStart: month, MonthEnd: month.AddDate(0, 1, 0), TotalTokens: int64(i + 1)})
	}
	plain := stripANSI(strings.Join(m.renderCompactYearMonthlyLines(report), "\n"))
	if !strings.Contains(plain, "active 6/12mo | streak 3mo (best 4mo)") {
		t.Fatalf("expected compact monthly header format, got %q", plain)
	}
}

func TestRenderYearMonthlyDetailLines_AppendsSelectedMonthDetail(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{ID: "ses_1", Title: "Current session"}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 100
	m.height = 30
	m.statsTab = 2
	m.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	m.globalMonthDaily = stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), ActiveDays: 1, TotalMessages: 120, TotalSessions: 12, TotalTokens: 123456789, TotalCost: 42.42, Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Messages: 120, Sessions: 12, Tokens: 123456789, Cost: 42.42}}}
	m.globalMonthDailyLoaded = true
	m.globalMonthDailyMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	m.dailySelectedDate = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	yearly := stats.YearMonthlyReport{Start: time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), ActiveMonths: 9, CurrentStreak: 3, BestStreak: 4, Months: []stats.MonthlySummary{{MonthStart: time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2025, time.May, 1, 0, 0, 0, 0, time.Local), TotalMessages: 10, TotalSessions: 2, TotalTokens: 1000, TotalCost: 1.25}, {MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), TotalMessages: 120, TotalSessions: 12, TotalTokens: 123456789, TotalCost: 42.42}}, TotalMessages: 130, TotalSessions: 14, TotalTokens: 123457789, TotalCost: 43.67}
	detail := stats.WindowReport{Label: "Monthly", Start: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), Messages: 120, Sessions: 12, Tokens: 123456789, Cost: 42.42, Models: []stats.ModelUsage{{Model: "openai\x00gpt-5.4", TotalTokens: 1000, Cost: 1.23, InputTokens: 500, OutputTokens: 200, CacheReadTokens: 250, ReasoningTokens: 50}}, TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 3}}, TopSkills: []stats.UsageCount{{Name: "golang-patterns", Count: 2}}, TopTools: []stats.UsageCount{{Name: "read", Count: 5}}, TotalAgentModelCalls: 3, TotalSkillCalls: 2, TotalToolCalls: 5, AllSessions: []stats.SessionUsage{{ID: "ses_1", Title: "Current session", Messages: 7, Tokens: 700, Cost: 0.77}}}
	plain := stripANSI(strings.Join(m.renderYearMonthlyDetailLines(yearly, detail), "\n"))
	content := strings.Join(m.renderYearMonthlyDetailLines(yearly, detail), "\n")
	if !strings.Contains(content, detailSectionBarStyle.Render("┃")) {
		t.Fatalf("expected monthly detail first header to use blue bar, got %q", content)
	}
	for _, snippet := range []string{"2026-03", "active 1/31d • streak 1d (best)", "Metrics", "peak day", "Providers (1)", "openai", "Models (1)", "oa| gpt-5.4", "Agents (1)", "explore", "Skills (1)", "golang-patterns", "Tools (1)", "read", "Total"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected monthly detail snippet %q, got %q", snippet, plain)
		}
	}
	if strings.Contains(plain, "03-01               peak day") || strings.Contains(plain, "03-01             peak day") {
		t.Fatalf("expected monthly detail metrics to omit selected day header column, got %q", plain)
	}
}

func TestRenderYearMonthlyDetailLines_LoadingHidesHeaderMeta(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.height = 30
	m.statsTab = 2
	m.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	yearly := stats.YearMonthlyReport{Months: []stats.MonthlySummary{{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)}}}
	plain := stripANSI(strings.Join(m.renderYearMonthlyDetailLines(yearly, stats.WindowReport{}), "\n"))
	if !strings.Contains(plain, "2026-03") || strings.Contains(plain, "selected") || strings.Contains(plain, "active ") || strings.Contains(plain, "streak") {
		t.Fatalf("expected loading detail header to omit right-side meta, got %q", plain)
	}
}

func TestRenderYearMonthlyDetailLines_ShowsProjectsBeforeModelsInProjectScope(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.height = 30
	m.statsTab = 2
	m.projectScope = true
	m.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	m.globalMonthDaily = stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Tokens: 1}}}
	m.globalMonthDailyLoaded = true
	m.globalMonthDailyMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	m.dailySelectedDate = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	yearly := stats.YearMonthlyReport{Months: []stats.MonthlySummary{{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)}}}
	detail := stats.WindowReport{Label: "Monthly", Start: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), Tokens: 76200000, TotalProjectCost: 33.45, TopProjects: []stats.UsageCount{{Name: "d:/workspace/opencode-workspace/oc", Amount: 76200000, Cost: 33.45}}, Models: []stats.ModelUsage{{Model: "openai\x00gpt-5.4", TotalTokens: 1000, Cost: 1.23}}}
	plain := stripANSI(strings.Join(m.renderYearMonthlyDetailLines(yearly, detail), "\n"))
	if !strings.Contains(plain, "Projects (1)") {
		t.Fatalf("expected project-scope monthly detail to include projects table, got %q", plain)
	}
	if strings.Index(plain, "Projects (1)") > strings.Index(plain, "Models (1)") {
		t.Fatalf("expected projects table before models table, got %q", plain)
	}
}
