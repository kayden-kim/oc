package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestRenderDailyDetailLines_UsesRequestedDetailLayout(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.height = 30
	m.dailySelectedDate = mustDate(2026, 3, 24)
	m.session = SessionItem{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유"}
	m.globalMonthDaily = stats.MonthDailyReport{Days: []stats.DailySummary{{Date: mustDate(2026, 3, 17), Tokens: 1}, {Date: mustDate(2026, 3, 18), Tokens: 1}, {Date: mustDate(2026, 3, 19), Tokens: 1}, {Date: mustDate(2026, 3, 20), Tokens: 1}, {Date: mustDate(2026, 3, 21), Tokens: 1}, {Date: mustDate(2026, 3, 22), Tokens: 1}, {Date: mustDate(2026, 3, 23), Tokens: 1}, {Date: mustDate(2026, 3, 24), Tokens: 1}}}
	m.globalMonthDailyLoaded = true
	report := stats.WindowReport{Label: "Daily", Start: mustDate(2026, 3, 24), End: mustDate(2026, 3, 25), Messages: 1259, Sessions: 2, Tokens: 215900000, Cost: 130.28, ActiveMinutes: 324, Models: []stats.ModelUsage{{Model: "openai\x00gpt-5.4", TotalTokens: 1000, Cost: 1.23}}, TopProjects: []stats.UsageCount{{Name: "/tmp/work-a", Amount: 3000, Cost: 4.56}}, TopAgents: []stats.UsageCount{{Name: "explore", Count: 2}}, TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 2}}, TopSkills: []stats.UsageCount{{Name: "writing-plans", Count: 1}}, TopTools: []stats.UsageCount{{Name: "bash", Count: 3}}, TotalProjectCost: 4.56, TotalSubtasks: 2, TotalAgentModelCalls: 2, TotalSkillCalls: 1, TotalToolCalls: 3, AllSessions: []stats.SessionUsage{{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유", Messages: 12, Tokens: 3000, Cost: 4.56}, {ID: "ses_2", Title: "다른 세션", Messages: 8, Tokens: 2000, Cost: 2.34}}, TopSessions: []stats.SessionUsage{{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유", Messages: 12, Tokens: 3000, Cost: 4.56}}}
	for i := 41; i <= 47; i++ {
		report.HalfHourSlots[i] = 1000
	}
	content := strings.Join(m.renderDailyDetailLines(report), "\n")
	plain := stripANSI(content)
	if !strings.Contains(content, detailSectionBarStyle.Render("┃")) || strings.Contains(content, renderSubSectionHeader("2026-03-24", todaySectionTitleStyle)) {
		t.Fatalf("expected daily detail first header styling, got %q", content)
	}
	for _, snippet := range []string{"2026-03-24", "5.4h active • streak 3.5h (best 8d)", "00", "22", "Models (1)", "Sessions (2)", "Projects (1)", "Agents (1)", "Skills (1)", "Tools (1)", "gpt-5.4", "CLI 앱이 brew cask인 이유", "다른 세션", "/tmp/work-a", "explore", "writing-plans", "bash", "model"} {
		if !strings.Contains(plain, snippet) {
			t.Fatalf("expected detail layout snippet %q, got %q", snippet, plain)
		}
	}
	if !strings.Contains(plain, "Total") || !strings.Contains(plain, "oa| gpt-5.4") {
		t.Fatalf("expected total rows and provider abbreviation, got %q", plain)
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
	m.dailySelectedDate = mustDate(2026, 3, 24)
	m.globalMonthDaily = stats.MonthDailyReport{Days: []stats.DailySummary{{Date: mustDate(2026, 3, 24), Tokens: 1}}}
	m.globalMonthDailyLoaded = true
	report := stats.WindowReport{Label: "Daily", Start: mustDate(2026, 3, 24), End: mustDate(2026, 3, 25), TopProjects: []stats.UsageCount{{Name: "/tmp/work-a", Amount: 3000, Cost: 4.56}}, TopAgents: []stats.UsageCount{{Name: "explore", Count: 2}}, TopAgentModels: []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 2}}, TotalAgentModelCalls: 2, TotalSubtasks: 2}
	plain := stripANSI(strings.Join(m.renderDailyDetailLines(report), "\n"))
	if strings.Contains(plain, "Projects (") || !strings.Contains(plain, "Agents (1)") {
		t.Fatalf("expected Projects omitted and Agents preserved, got %q", plain)
	}
}

func TestRenderDailyDetailHourlyLines_ShiftsGraphLeftByFourSpaces(t *testing.T) {
	m := newStatsTestModel()
	report := stats.WindowReport{}
	axis := stripANSI(m.renderDailyDetailAxisLine())
	spark := stripANSI(m.renderDailyDetailSparkline(report))
	if !strings.HasPrefix(axis, "      00") || !strings.HasPrefix(spark, "      ") {
		t.Fatalf("expected shifted hourly lines, axis=%q spark=%q", axis, spark)
	}
}

func TestCurrentTrailingActiveSlots(t *testing.T) {
	var slots [48]int64
	slots[40], slots[41], slots[45], slots[46], slots[47] = 1, 1, 1, 1, 1
	if got := currentTrailingActiveSlots(slots); got != 3 {
		t.Fatalf("currentTrailingActiveSlots() = %d, want 3", got)
	}
}

func TestRenderHalfHourSparkline_HighlightsCurrentSlotWhenToday(t *testing.T) {
	now := mustClock(2026, 4, 2, 10, 30)
	var slots [48]int64
	slots[21] = 1000
	line := renderHalfHourSparkline(slots, now, true)
	expected := lipgloss.NewStyle().Foreground(lipgloss.Color(currentHalfHourHighlightColor)).Render(string(sparklineChars[7]))
	if !strings.Contains(line, expected) {
		t.Fatalf("expected current half-hour slot to be highlighted white, got %q", line)
	}
}

func TestRenderHalfHourSparkline_DoesNotHighlightCurrentSlotWhenNotToday(t *testing.T) {
	now := mustClock(2026, 4, 2, 10, 30)
	var slots [48]int64
	slots[21] = 1000
	line := renderHalfHourSparkline(slots, now, false)
	unexpected := lipgloss.NewStyle().Foreground(lipgloss.Color(currentHalfHourHighlightColor)).Render(string(sparklineChars[7]))
	if strings.Contains(line, unexpected) {
		t.Fatalf("did not expect white current-slot highlight for non-today data, got %q", line)
	}
}
