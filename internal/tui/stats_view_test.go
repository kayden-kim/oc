package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(value string) string {
	return ansiRegexp.ReplaceAllString(value, "")
}

func TestRenderStatsTable_RespectsMaxWidth(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "name", MinWidth: 4, Style: defaultTextStyle}, {Header: "value", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}, {Header: "share", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}},
		[]statsTableRow{{Cells: []string{"very-long-entry-name", "123456", "99%"}}, {Divider: true}, {Cells: []string{"Total", "123456", "100%"}}},
		statsTableMaxWidth,
	)

	if len(lines) != 5 {
		t.Fatalf("expected 5 table lines, got %d", len(lines))
	}
	for i, line := range lines {
		plain := stripANSI(line)
		if width := utf8.RuneCountInString(strings.TrimPrefix(plain, "    ")); width > statsTableMaxWidth {
			t.Fatalf("expected line %d width <= %d, got %d in %q", i, statsTableMaxWidth, width, plain)
		}
	}
	if !strings.Contains(stripANSI(lines[0]), "name") || !strings.Contains(stripANSI(lines[0]), "value") || !strings.Contains(stripANSI(lines[0]), "share") {
		t.Fatalf("expected header row, got %q", stripANSI(lines[0]))
	}
	if !strings.Contains(stripANSI(lines[1]), strings.Repeat("┈", 10)) {
		t.Fatalf("expected header divider, got %q", stripANSI(lines[1]))
	}
}

func TestRenderStatsTable_UsesDisplayWidthForWideGlyphs(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "name", MinWidth: 4, Style: defaultTextStyle}, {Header: "value", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle}},
		[]statsTableRow{{Cells: []string{"모델이름이아주길어요테스트모델이름이아주길어요테스트모델이름이아주길어요테스트", "123456"}}, {Cells: []string{"🙂emoji-wide-name-with-extra-extra-extra-width", "42"}}},
		statsTableMaxWidth,
	)

	for i, line := range lines {
		plain := stripANSI(line)
		if width := lipgloss.Width(strings.TrimPrefix(plain, "    ")); width > statsTableMaxWidth {
			t.Fatalf("expected display width <= %d, got %d for line %d: %q", statsTableMaxWidth, width, i, plain)
		}
	}
	if !strings.Contains(stripANSI(lines[2]), "…") && !strings.Contains(stripANSI(lines[3]), "…") {
		t.Fatalf("expected truncation ellipsis for wide content, got %q", strings.Join([]string{stripANSI(lines[2]), stripANSI(lines[3])}, " | "))
	}
}

func TestRenderStatsTable_DoesNotPathTruncateModelNamesByDefault(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "Agent", MinWidth: 6, Style: defaultTextStyle}, {Header: "Model", MinWidth: 10, Style: defaultTextStyle}},
		[]statsTableRow{{Cells: []string{"explore", "qwen/qwen3-coder-super-long-model"}}},
		24,
	)

	plain := stripANSI(lines[2])
	if strings.Contains(plain, "..") {
		t.Fatalf("expected model names to use plain truncation, got %q", plain)
	}
	if !strings.Contains(plain, "qwen/") {
		t.Fatalf("expected model prefix to stay visible, got %q", plain)
	}
	if !strings.Contains(plain, "…") {
		t.Fatalf("expected truncated model ellipsis, got %q", plain)
	}
}

func TestRenderStatsTable_PathAwareColumnsUsePathTruncation(t *testing.T) {
	lines := renderStatsTable(
		[]statsTableColumn{{Header: "Project", MinWidth: 10, PathAware: true, Style: defaultTextStyle}},
		[]statsTableRow{{Cells: []string{"/Users/kayden/workspace/super-long-project-name"}}},
		18,
	)

	plain := stripANSI(lines[2])
	if !strings.Contains(plain, "..") {
		t.Fatalf("expected path-aware truncation, got %q", plain)
	}
}

func TestShortenPathMiddle_PreservesPathEnds(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		width int
		want  []string
	}{
		{name: "unix path", path: "/Users/kayden/workspace/super-long-project-name", width: 24, want: []string{"/Users", "..", "project-name"}},
		{name: "windows path", path: `C:\Users\kayden\workspace\super-long-project-name`, width: 26, want: []string{`C:\Users`, "..", "project-name"}},
		{name: "non path fallback", path: "very-long-non-path-value", width: 10, want: []string{"…"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncatePathAware(tt.path, tt.width)
			if lipgloss.Width(got) > tt.width {
				t.Fatalf("expected width <= %d, got %d in %q", tt.width, lipgloss.Width(got), got)
			}
			for _, snippet := range tt.want {
				if !strings.Contains(got, snippet) {
					t.Fatalf("expected %q in %q", snippet, got)
				}
			}
		})
	}
}

func TestFormatSummaryTokensPerHour(t *testing.T) {
	tests := []struct {
		name           string
		tokens         int64
		sessionMinutes int
		want           string
	}{
		{name: "zero tokens", tokens: 0, sessionMinutes: 60, want: "--"},
		{name: "zero minutes", tokens: 1000, sessionMinutes: 0, want: "--"},
		{name: "one hour", tokens: 50000, sessionMinutes: 60, want: "50k"},
		{name: "half hour", tokens: 120000, sessionMinutes: 30, want: "240k"},
		{name: "two hours", tokens: 200, sessionMinutes: 120, want: "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryTokensPerHour(tt.tokens, tt.sessionMinutes); got != tt.want {
				t.Fatalf("formatSummaryTokensPerHour(%d, %d) = %q, want %q", tt.tokens, tt.sessionMinutes, got, tt.want)
			}
		})
	}
}

func TestFormatSummaryCodeLinesPerHour(t *testing.T) {
	tests := []struct {
		name           string
		lines          int
		sessionMinutes int
		want           string
	}{
		{name: "zero lines", lines: 0, sessionMinutes: 60, want: "--"},
		{name: "zero minutes", lines: 100, sessionMinutes: 0, want: "--"},
		{name: "one hour", lines: 300, sessionMinutes: 60, want: "300"},
		{name: "two hours", lines: 5000, sessionMinutes: 120, want: "2.5k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryCodeLinesPerHour(tt.lines, tt.sessionMinutes); got != tt.want {
				t.Fatalf("formatSummaryCodeLinesPerHour(%d, %d) = %q, want %q", tt.lines, tt.sessionMinutes, got, tt.want)
			}
		})
	}
}

func TestFormatSummaryChangedFiles(t *testing.T) {
	tests := []struct {
		name  string
		files int
		want  string
	}{
		{name: "zero files", files: 0, want: "--"},
		{name: "small count", files: 42, want: "42"},
		{name: "compact count", files: 1250, want: "1.2k"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryChangedFiles(tt.files); got != tt.want {
				t.Fatalf("formatSummaryChangedFiles(%d) = %q, want %q", tt.files, got, tt.want)
			}
		})
	}
}

func TestFormatSummaryHours(t *testing.T) {
	tests := []struct {
		name    string
		minutes int
		want    string
	}{
		{name: "zero minutes", minutes: 0, want: "--"},
		{name: "negative minutes", minutes: -5, want: "--"},
		{name: "ninety minutes", minutes: 90, want: "1.5h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSummaryHours(tt.minutes); got != tt.want {
				t.Fatalf("formatSummaryHours(%d) = %q, want %q", tt.minutes, got, tt.want)
			}
		})
	}
}

func TestFormatPerHourWithTop(t *testing.T) {
	days := []stats.Day{
		{Date: time.Date(2026, time.March, 27, 0, 0, 0, 0, time.Local), Tokens: 1000, SessionMinutes: 60, CodeLines: 100},
		{Date: time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local), Tokens: 2000, SessionMinutes: 60, CodeLines: 240},
	}

	if got := formatTokensPerHourWithTop(1000, 60, days); got != "1k (50%)" {
		t.Fatalf("formatTokensPerHourWithTop() = %q, want %q", got, "1k (50%)")
	}
	if got := formatCodeLinesPerHourWithTop(100, 60, days); got != "100 (42%)" {
		t.Fatalf("formatCodeLinesPerHourWithTop() = %q, want %q", got, "100 (42%)")
	}
	if got := formatTokensPerHourWithTop(1000, 0, days); got != "-- (--)" {
		t.Fatalf("formatTokensPerHourWithTop zero minutes = %q, want %q", got, "-- (--)")
	}
	if got := formatCodeLinesPerHourWithTop(100, 0, days); got != "-- (--)" {
		t.Fatalf("formatCodeLinesPerHourWithTop zero minutes = %q, want %q", got, "-- (--)")
	}
}

func TestRenderMonthDailyLines_FormatsHeaderAndDays(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 80
	m.height = 24

	report := stats.MonthDailyReport{
		MonthStart:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		MonthEnd:      time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		ActiveDays:    3,
		TotalMessages: 10,
		TotalSessions: 2,
		TotalTokens:   5000,
		TotalCost:     15.5,
		Days: []stats.DailySummary{
			{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Messages: 2, Sessions: 1, Tokens: 1000, Cost: 3.0, FocusTag: "spike"},
			{Date: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.Local), Messages: 5, Sessions: 1, Tokens: 2500, Cost: 7.5, FocusTag: "heavy"},
			{Date: time.Date(2026, time.March, 20, 0, 0, 0, 0, time.Local), Messages: 3, Sessions: 1, Tokens: 1500, Cost: 5.0, FocusTag: "--"},
		},
	}

	lines := m.renderMonthDailyLines(report)

	if len(lines) == 0 {
		t.Fatalf("expected non-empty lines")
	}

	// Check that header contains month name
	plainLines := make([]string, len(lines))
	for i, line := range lines {
		plainLines[i] = stripANSI(line)
	}

	if !strings.Contains(strings.Join(plainLines, " "), "March 2026") {
		t.Fatalf("expected month name in header")
	}

	// Check that days are included
	foundMar01 := false
	foundMar15 := false
	for _, line := range plainLines {
		if strings.Contains(line, "Mar 01") {
			foundMar01 = true
		}
		if strings.Contains(line, "Mar 15") {
			foundMar15 = true
		}
	}

	if !foundMar01 {
		t.Fatalf("expected Mar 01 in output")
	}
	if !foundMar15 {
		t.Fatalf("expected Mar 15 in output")
	}

	// Check that focus tags are present
	if !strings.Contains(strings.Join(plainLines, " "), "spike") {
		t.Fatalf("expected 'spike' focus tag in output")
	}
	if !strings.Contains(strings.Join(plainLines, " "), "heavy") {
		t.Fatalf("expected 'heavy' focus tag in output")
	}
}

func TestRenderMonthDailyLines_ResponsiveToNarrowLayout(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 60 // Narrow layout
	m.height = 24

	report := stats.MonthDailyReport{
		MonthStart:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		MonthEnd:      time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		ActiveDays:    2,
		TotalMessages: 5,
		TotalSessions: 1,
		TotalTokens:   3000,
		TotalCost:     10.0,
		Days: []stats.DailySummary{
			{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Messages: 2, Sessions: 1, Tokens: 1500, Cost: 5.0, FocusTag: "heavy"},
			{Date: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.Local), Messages: 3, Sessions: 1, Tokens: 1500, Cost: 5.0, FocusTag: "--"},
		},
	}

	lines := m.renderMonthDailyLines(report)

	// Check that all lines fit within width
	for i, line := range lines {
		plain := stripANSI(line)
		width := lipgloss.Width(plain)
		if width > m.layoutWidth() {
			t.Fatalf("line %d exceeds layout width: %d > %d in %q", i, width, m.layoutWidth(), plain)
		}
	}
}

func TestMonthDailyColumnWidths_ResponsiveToLayout(t *testing.T) {
	tests := []struct {
		name           string
		width          int
		expectSessions bool
	}{
		{name: "wide layout (100 cols)", width: 100, expectSessions: true},  // 76 available → sessions included
		{name: "medium layout (72 cols)", width: 72, expectSessions: false}, // 68 available → no sessions
		{name: "narrow layout (60 cols)", width: 60, expectSessions: false}, // 56 available → compact
	}

	for _, tt := range tests {
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

func TestFocusTagStyle_ReturnsConsistentStyle(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)

	tests := []struct {
		tag string
	}{
		{tag: "spike"},
		{tag: "heavy"},
		{tag: "quiet"},
		{tag: "--"},
	}

	for _, tt := range tests {
		style := m.focusTagStyle(tt.tag)
		// Check that style rendering doesn't panic or return empty
		rendered := style.Render("test")
		if len(rendered) == 0 {
			t.Fatalf("expected non-empty rendered output for tag %q", tt.tag)
		}
	}
}
