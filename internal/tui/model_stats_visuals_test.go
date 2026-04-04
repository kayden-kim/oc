package tui

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

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

func TestRenderWindowLines_UsesCompactLayoutOnNarrowWidth(t *testing.T) {
	report := stats.WindowReport{Label: "Daily", Start: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local), Messages: 12345, Sessions: 2345, Tokens: 987654, Cost: 1234.56, Models: []stats.ModelUsage{{Model: "gpt-5.4-with-a-long-name", TotalTokens: 123456, Cost: 12.34}}, TopSessions: []stats.SessionUsage{{ID: "ses_abcdefghijklmnopqrstuvwxyz", Title: "Very long session title", Messages: 123, Tokens: 456789, Cost: 45.67}}}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	model.width = 35
	content := strings.Join(model.renderWindowLines(report), "\n")
	if got := maxRenderedLineWidth(content); got > 35 {
		t.Fatalf("expected compact window lines width <= 35, got %d", got)
	}
	if strings.Contains(content, "| Window") {
		t.Fatalf("expected narrow window view to avoid wide tables, got %q", stripANSI(content))
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
