package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

func TestMonthDailyColumns_PutsSessionsBeforeMessages(t *testing.T) {
	m := newStatsTestModel()
	m.width = 100
	columns := m.monthDailyColumns()
	if len(columns) < 3 {
		t.Fatalf("expected daily columns to include date, sessions, and messages, got %#v", columns)
	}
	if columns[1].Header != "sess" || columns[2].Header != "msgs" {
		t.Fatalf("expected sessions before messages, got %#v", columns)
	}
}

func TestRenderMonthDailyLines_FormatsHeaderAndDays(t *testing.T) {
	m := newStatsTestModel()
	m.width = 80
	m.height = 24
	report := stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), ActiveDays: 3, TotalMessages: 10, TotalSessions: 2, TotalTokens: 5000, TotalCost: 15.5, Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Messages: 2, Sessions: 1, Tokens: 1000, Cost: 3.0, FocusTag: "spike"}, {Date: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.Local), Messages: 5, Sessions: 1, Tokens: 2500, Cost: 7.5, FocusTag: "heavy"}, {Date: time.Date(2026, time.March, 20, 0, 0, 0, 0, time.Local), Messages: 3, Sessions: 1, Tokens: 1500, Cost: 5.0, FocusTag: "--"}}}
	lines := m.renderMonthDailyLines(report)
	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}
	plainLines := make([]string, len(lines))
	for i, line := range lines {
		plainLines[i] = stripANSI(line)
	}
	joined := strings.Join(plainLines, " ")
	if !strings.Contains(joined, "2026-03") || !strings.Contains(joined, "active 3/31d • streak 3d (best)") || !strings.Contains(joined, "Metrics") || !strings.Contains(joined, "Su Mo Tu We Th Fr Sa") {
		t.Fatalf("expected month daily header content, got %q", joined)
	}
	if !strings.Contains(lines[2], sundayTextStyle.Render("Su")) {
		t.Fatalf("expected Sunday weekday header to use Sunday style, got %q", lines[2])
	}
	if !strings.Contains(joined, "peak day") || !strings.Contains(joined, "total") || strings.Contains(joined, "messages 10 | sessions 2 | tokens") {
		t.Fatalf("expected metrics table header and no old summary row, got %q", joined)
	}
	if !strings.Contains(joined, "03-01") || !strings.Contains(joined, "03-15") || !strings.Contains(joined, "spike") || !strings.Contains(joined, "heavy") {
		t.Fatalf("expected day rows and focus tags, got %q", joined)
	}
}

func TestCalendarMonthDayCount_UsesCalendarDays(t *testing.T) {
	loc := time.FixedZone("DSTish", -7*60*60)
	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, loc)
	end := time.Date(2026, time.April, 1, 0, 0, 0, 0, loc)
	if got := calendarMonthDayCount(start, end); got != 31 {
		t.Fatalf("expected calendar month day count 31, got %d", got)
	}
}

func TestRenderMonthDailyDayLabel_StylesSundayOnly(t *testing.T) {
	sunday := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	wednesday := time.Date(2026, time.March, 4, 0, 0, 0, 0, time.Local)
	sundayLabel := renderMonthDailyDayLabel(sunday, false)
	if !strings.Contains(sundayLabel, sundayTextStyle.Render("Sun")) {
		t.Fatalf("expected Sunday label to use Sunday style, got %q", sundayLabel)
	}
	wednesdayLabel := renderMonthDailyDayLabel(wednesday, false)
	if strings.Contains(wednesdayLabel, sundayTextStyle.Render("Wed")) || !strings.Contains(stripANSI(wednesdayLabel), "03-04 Wed") {
		t.Fatalf("expected plain weekday label for non-Sunday, got %q", stripANSI(wednesdayLabel))
	}
}

func TestRenderMonthDailyDayLabel_UsesStrongerRedForSelectedSunday(t *testing.T) {
	sunday := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	selectedLabel := renderMonthDailyDayLabel(sunday, true)
	if !strings.Contains(selectedLabel, selectedSundayTextStyle.Render("Sun")) || strings.Contains(selectedLabel, sundayTextStyle.Render("Sun")) {
		t.Fatalf("expected selected Sunday to use stronger Sunday style only, got %q", selectedLabel)
	}
}

func TestRenderMonthDailyWeekdayHeader_UsesStrongerRedForSelectedSunday(t *testing.T) {
	m := newStatsTestModel()
	m.dailySelectedDate = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	header := m.renderMonthDailyWeekdayHeader()
	if !strings.Contains(header, selectedSundayTextStyle.Render("Su")) {
		t.Fatalf("expected selected Sunday header to use stronger Sunday style, got %q", header)
	}
}

func TestRenderMonthDailyHeatmapCell_UsesOrangeForSelectedDate(t *testing.T) {
	m := newStatsTestModel()
	selectedCell := m.renderMonthDailyHeatmapCell(3, true)
	normalCell := m.renderMonthDailyHeatmapCell(3, false)
	if selectedCell == normalCell {
		t.Fatalf("expected selected Daily heatmap cell to differ from normal cell")
	}
	if !strings.Contains(selectedCell, lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900")).Width(2).Align(lipgloss.Center).Render("██")) {
		t.Fatalf("expected selected Daily heatmap cell to use My Pulse orange emphasis, got %q", selectedCell)
	}
}

func TestMonthDailyBestStreak_ComputesBestContiguousActiveRun(t *testing.T) {
	report := stats.MonthDailyReport{Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 4, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 3, 0, 0, 0, 0, time.Local)}, {Date: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local), Tokens: 1}, {Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Tokens: 1}}}
	if got := formatMonthDailyBestStreak(report); got != "streak 2d (best)" {
		t.Fatalf("expected month best streak text, got %q", got)
	}
}

func TestRenderMonthDailyHeatmapCell_UsesTwoCharacterWidth(t *testing.T) {
	m := newStatsTestModel()
	cell := stripANSI(m.renderMonthDailyHeatmapCell(3, false))
	if lipgloss.Width(cell) != 2 || !strings.Contains(cell, "██") {
		t.Fatalf("expected two-character high-activity cell, got %q", cell)
	}
}

func TestRenderMonthDailyLines_UsesTokenBasedHeatmapNotCost(t *testing.T) {
	m := newStatsTestModel()
	m.statsConfig = config.NormalizeStatsConfig(config.StatsConfig{MediumTokens: 1000, HighTokens: 5000})
	m.width = 80
	m.height = 24
	report := stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), ActiveDays: 2, TotalMessages: 2, TotalSessions: 2, TotalTokens: 6000, TotalCost: 999.99, Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Messages: 1, Sessions: 1, Tokens: 0, Cost: 999.99, FocusTag: "heavy"}, {Date: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local), Messages: 1, Sessions: 1, Tokens: 6000, Cost: 0, FocusTag: "spike"}}}
	lines := m.renderMonthDailyHeatmapLines(report)
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "█") || strings.Count(plain, "██") != 1 {
		t.Fatalf("expected exactly one high heatmap cell from tokens, got %q", plain)
	}
}

func TestRenderMonthDailyLines_ResponsiveToNarrowLayout(t *testing.T) {
	m := newStatsTestModel()
	m.width = 60
	m.height = 24
	report := stats.MonthDailyReport{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local), ActiveDays: 2, TotalMessages: 5, TotalSessions: 1, TotalTokens: 3000, TotalCost: 10.0, Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Messages: 2, Sessions: 1, Tokens: 1500, Cost: 5.0, FocusTag: "heavy"}, {Date: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.Local), Messages: 3, Sessions: 1, Tokens: 1500, Cost: 5.0, FocusTag: "--"}}}
	lines := m.renderMonthDailyLines(report)
	for i, line := range lines {
		plain := stripANSI(line)
		if width := lipgloss.Width(plain); width > m.layoutWidth() {
			t.Fatalf("line %d exceeds layout width: %d > %d in %q", i, width, m.layoutWidth(), plain)
		}
	}
}

func TestMonthDailyColumnWidths_ResponsiveToLayout(t *testing.T) {
	for _, tt := range []struct {
		name           string
		width          int
		expectSessions bool
	}{{"wide layout (100 cols)", 100, true}, {"medium layout (72 cols)", 72, false}, {"narrow layout (60 cols)", 60, false}} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
			m.width = tt.width
			m.height = 24
			layout := m.monthDailyColumnWidths()
			if tt.expectSessions && layout.sessionsWidth == 0 {
				t.Fatalf("expected sessions column in %s", tt.name)
			}
			if !tt.expectSessions && layout.sessionsWidth > 0 {
				t.Fatalf("did not expect sessions column in %s", tt.name)
			}
			if layout.dateWidth <= 0 || layout.tokensWidth <= 0 || layout.costWidth <= 0 {
				t.Fatalf("expected positive column widths in %s, got %+v", tt.name, layout)
			}
		})
	}
}
