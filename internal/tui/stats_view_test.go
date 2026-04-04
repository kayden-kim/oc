package tui

import (
	"strings"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestRenderMetricsLines_PutSessionsBeforeMessages(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	m.dailySelectedDate = mustDate(2026, 3, 2)
	m.monthlySelectedMonth = mustDate(2026, 3, 1)
	daily := stats.MonthDailyReport{MonthStart: mustDate(2026, 3, 1), MonthEnd: mustDate(2026, 4, 1), TotalMessages: 30, TotalSessions: 10, TotalTokens: 1000, TotalCost: 12.34, Days: []stats.DailySummary{{Date: mustDate(2026, 3, 2), Messages: 20, Sessions: 5, Tokens: 900, Cost: 10.11}}}
	monthly := stats.YearMonthlyReport{TotalMessages: 30, TotalSessions: 10, TotalTokens: 1000, TotalCost: 12.34, Months: []stats.MonthlySummary{{MonthStart: mustDate(2026, 3, 1), MonthEnd: mustDate(2026, 4, 1), TotalMessages: 20, TotalSessions: 5, TotalTokens: 900, TotalCost: 10.11}}}
	dailyPlain := stripANSI(strings.Join(m.renderMonthDailyMetricsLines(daily), "\n"))
	if strings.Index(dailyPlain, "sessions") > strings.Index(dailyPlain, "messages") {
		t.Fatalf("expected daily metrics to list sessions before messages, got %q", dailyPlain)
	}
	monthlyPlain := stripANSI(strings.Join(m.renderYearMonthlyMetricsLines(monthly), "\n"))
	if strings.Index(monthlyPlain, "sessions") > strings.Index(monthlyPlain, "messages") {
		t.Fatalf("expected monthly metrics to list sessions before messages, got %q", monthlyPlain)
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

func TestFocusTagStyle_ReturnsConsistentStyle(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	for _, tt := range []struct{ tag string }{{"spike"}, {"heavy"}, {"quiet"}, {"--"}} {
		if rendered := m.focusTagStyle(tt.tag).Render("test"); len(rendered) == 0 {
			t.Fatalf("expected non-empty rendered output for tag %q", tt.tag)
		}
	}
}
