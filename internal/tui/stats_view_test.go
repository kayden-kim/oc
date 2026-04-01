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

func TestStatsTableColumnWidths_PrefersExpandableColumnsForRemainingWidth(t *testing.T) {
	columns := []statsTableColumn{
		{Header: "Model", MinWidth: 10, Expand: true, Style: defaultTextStyle},
		{Header: "Cost", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle},
	}
	rows := []statsTableRow{{Cells: []string{"gpt-5.4", "$1"}}}
	widths := statsTableColumnWidths(columns, rows, 20)

	if got, want := widths[0], 14; got != want {
		t.Fatalf("expected expandable first column width %d, got %d", want, got)
	}
	if got, want := widths[1], 4; got != want {
		t.Fatalf("expected non-expandable second column width %d, got %d", want, got)
	}
}

func TestMonthDailyColumns_PutsSessionsBeforeMessages(t *testing.T) {
	m := NewModel([]PluginItem{}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 100
	columns := m.monthDailyColumns()

	if len(columns) < 3 {
		t.Fatalf("expected daily columns to include date, sessions, and messages, got %#v", columns)
	}
	if columns[1].Header != "sess" || columns[2].Header != "msgs" {
		t.Fatalf("expected sessions before messages, got %#v", columns)
	}
}

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
	m := NewModel([]PluginItem{}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
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

	// Check that header contains month label
	plainLines := make([]string, len(lines))
	for i, line := range lines {
		plainLines[i] = stripANSI(line)
	}

	if !strings.Contains(strings.Join(plainLines, " "), "2026-03") {
		t.Fatalf("expected month label in header")
	}
	if !strings.Contains(strings.Join(plainLines, " "), "active 3/31d • streak 3d (best)") {
		t.Fatalf("expected streak meta in title row, got %q", strings.Join(plainLines, " "))
	}
	if !strings.Contains(strings.Join(plainLines, " "), "Metrics") {
		t.Fatalf("expected metrics section in output")
	}
	if !strings.Contains(strings.Join(plainLines, " "), "Su Mo Tu We Th Fr Sa") {
		t.Fatalf("expected weekday header in output")
	}
	if !strings.Contains(lines[2], sundayTextStyle.Render("Su")) {
		t.Fatalf("expected Sunday weekday header to use Sunday style, got %q", lines[2])
	}
	if !strings.Contains(strings.Join(plainLines, " "), "peak day") || !strings.Contains(strings.Join(plainLines, " "), "total") {
		t.Fatalf("expected metrics table headers in output")
	}
	if strings.Contains(strings.Join(plainLines, " "), "messages 10 | sessions 2 | tokens") {
		t.Fatalf("did not expect old summary row in output")
	}

	// Check that days are included
	foundMar01 := false
	foundMar15 := false
	for _, line := range plainLines {
		if strings.Contains(line, "03-01") {
			foundMar01 = true
		}
		if strings.Contains(line, "03-15") {
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
	if strings.Contains(wednesdayLabel, sundayTextStyle.Render("Wed")) {
		t.Fatalf("did not expect non-Sunday label to use Sunday style, got %q", wednesdayLabel)
	}
	if !strings.Contains(stripANSI(wednesdayLabel), "03-04 Wed") {
		t.Fatalf("expected plain weekday label for non-Sunday, got %q", stripANSI(wednesdayLabel))
	}
}

func TestRenderMonthDailyDayLabel_UsesStrongerRedForSelectedSunday(t *testing.T) {
	sunday := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	selectedLabel := renderMonthDailyDayLabel(sunday, true)
	if !strings.Contains(selectedLabel, selectedSundayTextStyle.Render("Sun")) {
		t.Fatalf("expected selected Sunday to use stronger Sunday style, got %q", selectedLabel)
	}
	if strings.Contains(selectedLabel, sundayTextStyle.Render("Sun")) {
		t.Fatalf("expected selected Sunday to avoid base Sunday style, got %q", selectedLabel)
	}
}

func TestRenderMonthDailyWeekdayHeader_UsesStrongerRedForSelectedSunday(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.dailySelectedDate = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	header := m.renderMonthDailyWeekdayHeader()
	if !strings.Contains(header, selectedSundayTextStyle.Render("Su")) {
		t.Fatalf("expected selected Sunday header to use stronger Sunday style, got %q", header)
	}
}

func TestRenderMonthDailyHeatmapCell_UsesOrangeForSelectedDate(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
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
	report := stats.MonthDailyReport{Days: []stats.DailySummary{
		{Date: time.Date(2026, time.March, 5, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 4, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 3, 0, 0, 0, 0, time.Local)},
		{Date: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Tokens: 1},
	}}
	if got := formatMonthDailyBestStreak(report); got != "streak 2d (best)" {
		t.Fatalf("expected month best streak text, got %q", got)
	}
}

func TestRenderMonthDailyHeatmapCell_UsesTwoCharacterWidth(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	cell := stripANSI(m.renderMonthDailyHeatmapCell(3, false))
	if lipgloss.Width(cell) != 2 {
		t.Fatalf("expected heatmap cell width 2, got %d in %q", lipgloss.Width(cell), cell)
	}
	if !strings.Contains(cell, "██") {
		t.Fatalf("expected two-character high-activity cell, got %q", cell)
	}
}

func TestRenderMonthDailyLines_UsesTokenBasedHeatmapNotCost(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{MediumTokens: 1000, HighTokens: 5000}, "v1.0", false)
	m.width = 80
	m.height = 24

	report := stats.MonthDailyReport{
		MonthStart:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		MonthEnd:      time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		ActiveDays:    2,
		TotalMessages: 2,
		TotalSessions: 2,
		TotalTokens:   6000,
		TotalCost:     999.99,
		Days: []stats.DailySummary{
			{Date: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), Messages: 1, Sessions: 1, Tokens: 0, Cost: 999.99, FocusTag: "heavy"},
			{Date: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local), Messages: 1, Sessions: 1, Tokens: 6000, Cost: 0, FocusTag: "spike"},
		},
	}

	lines := m.renderMonthDailyHeatmapLines(report)
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "█") {
		t.Fatalf("expected high-token day to appear as a high heatmap cell, got %q", plain)
	}
	if strings.Count(plain, "██") != 1 {
		t.Fatalf("expected exactly one high heatmap cell from tokens, got %q", plain)
	}
}

func TestRenderDailyDetailLines_UsesRequestedDetailLayout(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 100
	m.height = 30
	m.dailySelectedDate = time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local)
	m.session = SessionItem{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유"}
	m.globalMonthDaily = stats.MonthDailyReport{Days: []stats.DailySummary{
		{Date: time.Date(2026, time.March, 17, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 18, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 19, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 20, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 21, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 22, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 23, 0, 0, 0, 0, time.Local), Tokens: 1},
		{Date: time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local), Tokens: 1},
	}}
	m.globalMonthDailyLoaded = true
	report := stats.WindowReport{
		Label:                "Daily",
		Start:                time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local),
		End:                  time.Date(2026, time.March, 25, 0, 0, 0, 0, time.Local),
		Messages:             1259,
		Sessions:             2,
		Tokens:               215900000,
		Cost:                 130.28,
		ActiveMinutes:        324,
		Models:               []stats.ModelUsage{{Model: "openai\x00gpt-5.4", TotalTokens: 1000, Cost: 1.23}},
		TopProjects:          []stats.UsageCount{{Name: "/tmp/work-a", Amount: 3000, Cost: 4.56}},
		TopAgents:            []stats.UsageCount{{Name: "explore", Count: 2}},
		TopAgentModels:       []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 2}},
		TopSkills:            []stats.UsageCount{{Name: "writing-plans", Count: 1}},
		TopTools:             []stats.UsageCount{{Name: "bash", Count: 3}},
		TotalProjectCost:     4.56,
		TotalSubtasks:        2,
		TotalAgentModelCalls: 2,
		TotalSkillCalls:      1,
		TotalToolCalls:       3,
		AllSessions:          []stats.SessionUsage{{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유", Messages: 12, Tokens: 3000, Cost: 4.56}, {ID: "ses_2", Title: "다른 세션", Messages: 8, Tokens: 2000, Cost: 2.34}},
		TopSessions:          []stats.SessionUsage{{ID: "ses_1", Title: "CLI 앱이 brew cask인 이유", Messages: 12, Tokens: 3000, Cost: 4.56}},
	}
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
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 100
	m.height = 30
	m.projectScope = true
	m.dailySelectedDate = time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local)
	m.globalMonthDaily = stats.MonthDailyReport{Days: []stats.DailySummary{{Date: time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local), Tokens: 1}}}
	m.globalMonthDailyLoaded = true
	report := stats.WindowReport{
		Label: "Daily", Start: time.Date(2026, time.March, 24, 0, 0, 0, 0, time.Local), End: time.Date(2026, time.March, 25, 0, 0, 0, 0, time.Local),
		TopProjects:          []stats.UsageCount{{Name: "/tmp/work-a", Amount: 3000, Cost: 4.56}},
		TopAgents:            []stats.UsageCount{{Name: "explore", Count: 2}},
		TopAgentModels:       []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 2}},
		TotalAgentModelCalls: 2,
		TotalSubtasks:        2,
	}
	plain := stripANSI(strings.Join(m.renderDailyDetailLines(report), "\n"))
	if strings.Contains(plain, "Projects (") {
		t.Fatalf("expected Projects table omitted in project scope, got %q", plain)
	}
	if !strings.Contains(plain, "Agents (1)") {
		t.Fatalf("expected other activity tables to remain, got %q", plain)
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

func TestRenderYearMonthlyLines_FormatsNumericGridAndMetrics(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 100
	m.height = 30
	m.statsTab = 2
	m.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	report := stats.YearMonthlyReport{
		Start:         time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local),
		End:           time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		ActiveMonths:  10,
		CurrentStreak: 4,
		BestStreak:    5,
	}
	for i := 0; i < 12; i++ {
		month := report.Start.AddDate(0, i, 0)
		tokens := int64((i + 1) * 1_000_000)
		if i == 3 {
			tokens = 0
		}
		item := stats.MonthlySummary{
			MonthStart:    month,
			MonthEnd:      month.AddDate(0, 1, 0),
			ActiveDays:    i + 1,
			TotalMessages: (i + 1) * 100,
			TotalSessions: (i + 1) * 10,
			TotalTokens:   tokens,
			TotalCost:     float64(i+1) * 9.25,
		}
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
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 60
	m.height = 24
	m.statsTab = 2
	report := stats.YearMonthlyReport{
		Start:         time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local),
		End:           time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		ActiveMonths:  6,
		CurrentStreak: 3,
		BestStreak:    4,
	}
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
	m.globalMonthDaily = stats.MonthDailyReport{
		MonthStart:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		MonthEnd:      time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		ActiveDays:    1,
		TotalMessages: 120,
		TotalSessions: 12,
		TotalTokens:   123456789,
		TotalCost:     42.42,
		Days: []stats.DailySummary{{
			Date:     time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
			Messages: 120,
			Sessions: 12,
			Tokens:   123456789,
			Cost:     42.42,
		}},
	}
	m.globalMonthDailyLoaded = true
	m.globalMonthDailyMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	m.dailySelectedDate = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	yearly := stats.YearMonthlyReport{
		Start:         time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local),
		End:           time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		ActiveMonths:  9,
		CurrentStreak: 3,
		BestStreak:    4,
		Months: []stats.MonthlySummary{{
			MonthStart:    time.Date(2025, time.April, 1, 0, 0, 0, 0, time.Local),
			MonthEnd:      time.Date(2025, time.May, 1, 0, 0, 0, 0, time.Local),
			TotalMessages: 10,
			TotalSessions: 2,
			TotalTokens:   1000,
			TotalCost:     1.25,
		}, {
			MonthStart:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
			MonthEnd:      time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
			TotalMessages: 120,
			TotalSessions: 12,
			TotalTokens:   123456789,
			TotalCost:     42.42,
		}},
		TotalMessages: 130,
		TotalSessions: 14,
		TotalTokens:   123457789,
		TotalCost:     43.67,
	}
	detail := stats.WindowReport{
		Label:                "Monthly",
		Start:                time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		End:                  time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		Messages:             120,
		Sessions:             12,
		Tokens:               123456789,
		Cost:                 42.42,
		Models:               []stats.ModelUsage{{Model: "openai\x00gpt-5.4", TotalTokens: 1000, Cost: 1.23, InputTokens: 500, OutputTokens: 200, CacheReadTokens: 250, ReasoningTokens: 50}},
		TopAgentModels:       []stats.UsageCount{{Name: "explore\x00gpt-5.4", Count: 3}},
		TopSkills:            []stats.UsageCount{{Name: "golang-patterns", Count: 2}},
		TopTools:             []stats.UsageCount{{Name: "read", Count: 5}},
		TotalAgentModelCalls: 3,
		TotalSkillCalls:      2,
		TotalToolCalls:       5,
		AllSessions:          []stats.SessionUsage{{ID: "ses_1", Title: "Current session", Messages: 7, Tokens: 700, Cost: 0.77}},
	}

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
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	m.width = 100
	m.height = 30
	m.statsTab = 2
	m.monthlySelectedMonth = time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	yearly := stats.YearMonthlyReport{Months: []stats.MonthlySummary{{MonthStart: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local), MonthEnd: time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local)}}}
	plain := stripANSI(strings.Join(m.renderYearMonthlyDetailLines(yearly, stats.WindowReport{}), "\n"))
	if !strings.Contains(plain, "2026-03") {
		t.Fatalf("expected loading detail header month label, got %q", plain)
	}
	if strings.Contains(plain, "selected") || strings.Contains(plain, "active ") || strings.Contains(plain, "streak") {
		t.Fatalf("expected loading detail header to omit right-side meta, got %q", plain)
	}
}

func TestRenderYearMonthlyDetailLines_ShowsProjectsBeforeModelsInProjectScope(t *testing.T) {
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
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
	detail := stats.WindowReport{
		Label:            "Monthly",
		Start:            time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		End:              time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
		Tokens:           76200000,
		TotalProjectCost: 33.45,
		TopProjects:      []stats.UsageCount{{Name: "d:/workspace/opencode-workspace/oc", Amount: 76200000, Cost: 33.45}},
		Models:           []stats.ModelUsage{{Model: "openai\x00gpt-5.4", TotalTokens: 1000, Cost: 1.23}},
	}
	plain := stripANSI(strings.Join(m.renderYearMonthlyDetailLines(yearly, detail), "\n"))
	if !strings.Contains(plain, "Projects (1)") {
		t.Fatalf("expected project-scope monthly detail to include projects table, got %q", plain)
	}
	if strings.Index(plain, "Projects (1)") > strings.Index(plain, "Models (1)") {
		t.Fatalf("expected projects table before models table, got %q", plain)
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
	m := NewModel([]PluginItem{}, []EditChoice{}, []SessionItem{}, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, "v1.0", false)
	report := stats.WindowReport{}
	axis := stripANSI(m.renderDailyDetailAxisLine())
	spark := stripANSI(m.renderDailyDetailSparkline(report))
	if !strings.HasPrefix(axis, "    00") {
		t.Fatalf("expected axis line to align with the surrounding bullet indent, got %q", axis)
	}
	if !strings.HasPrefix(spark, "    ") {
		t.Fatalf("expected sparkline to align with the surrounding bullet indent, got %q", spark)
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
