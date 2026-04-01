package tui

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

const (
	rhythmFirstColumnWidth  = 44
	metricColumnWidth       = 24
	overviewPairColumnWidth = 28
	trendLabelColumnWidth   = 10
	maxActivityItems        = 15
	statsTabWidth           = 14
	statsTableMaxWidth      = 76
	statsTableColumnGap     = 2
	usageBarWidth           = 8
)

type statsTableColumn struct {
	Header     string
	MinWidth   int
	AlignRight bool
	PathAware  bool
	Style      lipgloss.Style
}

type statsTableRow struct {
	Cells   []string
	Divider bool
}

func statsTabTitles() []string {
	return []string{"Overview", "Daily", "Monthly"}
}

func filterNonEmpty(parts []string) []string {
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			result = append(result, part)
		}
	}
	return result
}

func (m Model) renderLauncherAnalytics() string {
	report := m.currentReport()
	sections := []string{renderSubSectionHeader("My Pulse", habitSectionTitleStyle), ""}
	minimap := m.renderLauncherMinimap(report)
	if m.isNarrowLayout() {
		sections = append(sections, defaultTextStyle.Render("    ")+truncateDisplayWidth("• daily  "+formatDailyPulseValue(report), max(0, m.layoutWidth()-4)))
	} else {
		habitLine := styledMetricLead("• daily  ", formatDailyPulseValue(report))
		if minimap != "" {
			habitLine += minimap
		}
		sections = append(sections, bulletLine(habitLine))
	}
	sparkline := m.render24hSparkline(report)
	if m.isNarrowLayout() {
		sections = append(sections, defaultTextStyle.Render("    ")+truncateDisplayWidth("• hourly "+formatHourlyPulseValue(report), max(0, m.layoutWidth()-4)))
	} else {
		todayLine := styledMetricLead("• hourly ", formatHourlyPulseValue(report))
		if sparkline != "" {
			todayLine += sparkline
		}
		sections = append(sections, bulletLine(todayLine))
	}
	sections = append(sections, "", renderSubSectionHeader("Metrics", todaySectionTitleStyle))
	sections = append(sections, m.renderMetricsTable(report)...)
	return strings.Join(sections, "\n")
}

func (m Model) currentReport() stats.Report {
	if m.projectScope {
		return m.projectStats
	}
	return m.globalStats
}

func maxTokens(days []stats.Day) int64 {
	var max int64
	for _, day := range days {
		if day.Tokens > max {
			max = day.Tokens
		}
	}
	return max
}

func maxCost(days []stats.Day) float64 {
	var max float64
	for _, day := range days {
		if day.Cost > max {
			max = day.Cost
		}
	}
	return max
}

func maxSessionMinutes(days []stats.Day) int {
	var max int
	for _, day := range days {
		if day.SessionMinutes > max {
			max = day.SessionMinutes
		}
	}
	return max
}

func maxCodeLines(days []stats.Day) int {
	var max int
	for _, day := range days {
		if day.CodeLines > max {
			max = day.CodeLines
		}
	}
	return max
}

func maxChangedFiles(days []stats.Day) int {
	var max int
	for _, day := range days {
		if day.ChangedFiles > max {
			max = day.ChangedFiles
		}
	}
	return max
}

func displayBestStreak(report stats.Report) int {
	if report.BestStreak > report.CurrentStreak {
		return report.BestStreak
	}
	return report.CurrentStreak
}

func formatActiveDaysSummary(report stats.Report) string {
	if len(report.Days) == 0 {
		return "--"
	}
	return fmt.Sprintf("%d/30d", report.ActiveDays)
}

func formatDailyPulseValue(report stats.Report) string {
	return formatPulseValue(
		formatActiveDaysSummary(report),
		formatInlineStreakSummary(formatDayStreakValue(report), formatDayStreakValueWithBest(report)),
	)
}

func formatHourlyPulseValue(report stats.Report) string {
	hourlyValue := "--"
	if len(report.Days) > 0 {
		hourlyValue = formatRolling24hHours(report.Rolling24hSessionMinutes)
	}
	return formatPulseValue(
		hourlyValue,
		formatInlineStreakSummary(formatHourlyStreakValue(report), formatHourlyBestStreakValue(report)),
	)
}

func formatPulseValue(primary string, summary string) string {
	if summary == "" {
		return primary
	}
	return primary + " " + summary
}

func formatInlineStreakSummary(current string, best string) string {
	if current == "--" && best == "--" {
		return ""
	}
	if current == best && current != "--" {
		return fmt.Sprintf("(streak %s)", current)
	}
	return fmt.Sprintf("(streak %s, best %s)", current, best)
}

func formatDayStreakValue(report stats.Report) string {
	if len(report.Days) == 0 {
		return "--"
	}
	return fmt.Sprintf("%dd", report.CurrentStreak)
}

func formatDayStreakValueWithBest(report stats.Report) string {
	if len(report.Days) == 0 {
		return "--"
	}
	return fmt.Sprintf("%dd", displayBestStreak(report))
}

func formatHourlyStreakValue(report stats.Report) string {
	if len(report.Days) == 0 {
		return "--"
	}
	return formatHourlyStreakDuration(report.CurrentHourlyStreakSlots)
}

func formatHourlyBestStreakValue(report stats.Report) string {
	if len(report.Days) == 0 {
		return "--"
	}
	best := report.BestHourlyStreakSlots
	if best < report.CurrentHourlyStreakSlots {
		best = report.CurrentHourlyStreakSlots
	}
	return formatHourlyStreakDuration(best)
}

func formatHourlyStreakDuration(slots int) string {
	if slots <= 0 {
		return "0h"
	}
	if slots%2 == 0 {
		return fmt.Sprintf("%dh", slots/2)
	}
	return fmt.Sprintf("%.1fh", float64(slots)/2)
}

func maxTokenDay(days []stats.Day) stats.Day {
	var max stats.Day
	for _, day := range days {
		if day.Tokens > max.Tokens {
			max = day
		}
	}
	return max
}

func maxSessionDay(days []stats.Day) stats.Day {
	var max stats.Day
	for _, day := range days {
		if day.SessionMinutes > max.SessionMinutes {
			max = day
		}
	}
	return max
}

func formatPeakValue(value string, date time.Time) string {
	if date.IsZero() {
		return value
	}
	return fmt.Sprintf("%s (%s)", value, date.Format("2006-01-02"))
}

func joinTitleAndMeta(left, right string, width int) string {
	padding := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return truncateDisplayWidth(left+strings.Repeat(" ", padding)+right, width)
}

func (m Model) renderLauncherMinimap(report stats.Report) string {
	if m.isNarrowLayout() {
		return ""
	}
	if len(report.Days) == 0 {
		return ""
	}
	count := minMinimapDayCountWide
	requiredWidth := count + (count-1)/7
	if available := m.launcherVisualWidth(); available < requiredWidth {
		if available < minMinimapDayCountSlim+(minMinimapDayCountSlim-1)/7 {
			return ""
		}
		count = minMinimapDayCountSlim
	}
	days := report.Days
	if len(days) > count {
		days = days[len(days)-count:]
	}
	return fmt.Sprintf("%s", m.renderHeatmapLine(days))
}

func bulletLine(line string) string {
	return defaultTextStyle.Render("    ") + line
}

func todayMetricLine(line string) string {
	return defaultTextStyle.Render("    ") + line
}

func padStyledText(rendered string, visibleWidth int, targetWidth int) string {
	if visibleWidth >= targetWidth {
		return rendered
	}
	return rendered + defaultTextStyle.Render(strings.Repeat(" ", targetWidth-visibleWidth))
}

func styledTodayMetrics(labelA string, valueA string, labelB string, valueB string) string {
	return renderTwoColumns(labelA, valueA, metricColumnWidth, labelB, valueB, metricColumnWidth)
}

func styledTodayMetricTriple(labelA string, valueA string, labelB string, valueB string, labelC string, valueC string) string {
	return renderThreeColumns(labelA, valueA, metricColumnWidth, labelB, valueB, metricColumnWidth, labelC, valueC, metricColumnWidth)
}

func valueText(value string) string {
	return statsValueTextStyle.Render(value)
}

func styledMetricLine(label string, value string) string {
	return defaultTextStyle.Render(label) + valueText(value)
}

func styledTrendLine(label string, value string) string {
	return padStyledText(defaultTextStyle.Render(label), lipgloss.Width(label), trendLabelColumnWidth) + valueText(value)
}

func styledMetricLead(label string, value string) string {
	return renderColumn(label, value, rhythmFirstColumnWidth)
}

func styledMetricFixedPair(labelA string, valueA string, labelB string, valueB string) string {
	return renderTwoColumns(labelA, valueA, metricColumnWidth, labelB, valueB, metricColumnWidth)
}

func activitySectionHeader(title string, unique int) string {
	title = strings.TrimPrefix(title, "Activity - ")
	return renderSubSectionHeader(fmt.Sprintf("%s (%s)", title, formatGroupedInt(unique)), habitSectionTitleStyle)
}

func styledOverviewPair(labelA string, valueA string, labelB string, valueB string) string {
	return renderTwoColumns(labelA, valueA, overviewPairColumnWidth, labelB, valueB, overviewPairColumnWidth)
}

func (m Model) renderMetricsTable(report stats.Report) []string {
	todayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	columns := []statsTableColumn{
		{Header: "", MinWidth: 8, Style: defaultTextStyle},
		{Header: "today", MinWidth: 10, Style: todayStyle},
		{Header: "peak day", MinWidth: 12, Style: statsValueTextStyle},
		{Header: "30d total", MinWidth: 10, Style: statsValueTextStyle},
	}
	rows := []statsTableRow{
		{Cells: []string{"tokens", formatTokensWithTop(report.TodayTokens, report.Days), formatPeakValue(formatSummaryTokens(maxTokens(report.Days)), maxTokenDay(report.Days).Date), formatSummaryTokens(report.ThirtyDayTokens)}},
		{Cells: []string{"cost", formatCurrencyWithTop(report.TodayCost, report.Days), formatPeakValue(formatSummaryCurrency(report.HighestBurnDay.Cost), report.HighestBurnDay.Date), formatSummaryCurrency(report.ThirtyDayCost)}},
		{Cells: []string{"hours", formatHoursWithTop(report.TodaySessionMinutes, report.Days), formatPeakValue(formatSummaryHours(maxSessionMinutes(report.Days)), maxSessionDay(report.Days).Date), formatSummaryHours(report.ThirtyDaySessionMinutes)}},
		{Cells: []string{"lines", formatCodeLinesWithTop(report.TodayCodeLines, report.Days), formatPeakValue(formatSummaryCodeLines(report.HighestCodeDay.CodeLines), report.HighestCodeDay.Date), formatSummaryCodeLines(report.ThirtyDayCodeLines)}},
		{Cells: []string{"files", formatChangedFilesWithTop(report.TodayChangedFiles, report.Days), formatPeakValue(formatSummaryChangedFiles(report.HighestChangedFilesDay.ChangedFiles), report.HighestChangedFilesDay.Date), formatSummaryChangedFiles(report.ThirtyDayChangedFiles)}},
		{Divider: true},
		{Cells: []string{"tok/h", formatTokensPerHourWithTop(report.TodayTokens, report.TodaySessionMinutes, report.Days), formatPeakValue(formatSummaryTokensPerHour(maxTokensPerHourDay(report.Days).Tokens, maxTokensPerHourDay(report.Days).SessionMinutes), maxTokensPerHourDay(report.Days).Date), formatSummaryTokensPerHour(report.ThirtyDayTokens, report.ThirtyDaySessionMinutes)}},
		{Cells: []string{"line/h", formatCodeLinesPerHourWithTop(report.TodayCodeLines, report.TodaySessionMinutes, report.Days), formatPeakValue(formatSummaryCodeLinesPerHour(maxCodeLinesPerHourDay(report.Days).CodeLines, maxCodeLinesPerHourDay(report.Days).SessionMinutes), maxCodeLinesPerHourDay(report.Days).Date), formatSummaryCodeLinesPerHour(report.ThirtyDayCodeLines, report.ThirtyDaySessionMinutes)}},
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func renderStatsTable(columns []statsTableColumn, rows []statsTableRow, maxWidth int) []string {
	if len(columns) == 0 {
		return nil
	}
	widths := statsTableColumnWidths(columns, rows, maxWidth)
	contentWidth := statsTableContentWidth(widths)
	lines := make([]string, 0, len(rows)+2)
	lines = append(lines, renderStatsTableHeader(columns, widths))
	lines = append(lines, statsTableDividerLine(contentWidth))
	for _, row := range rows {
		if row.Divider {
			lines = append(lines, statsTableDividerLine(contentWidth))
			continue
		}
		lines = append(lines, renderStatsTableLine(columns, widths, row.Cells))
	}
	return lines
}

func headerCells(columns []statsTableColumn) []string {
	cells := make([]string, len(columns))
	for i, column := range columns {
		cells[i] = column.Header
	}
	return cells
}

func renderStatsTableHeader(columns []statsTableColumn, widths []int) string {
	parts := make([]string, len(columns))
	for i, column := range columns {
		parts[i] = renderStatsTableCell(column.Header, widths[i], column.AlignRight, column.PathAware, defaultTextStyle)
	}
	return defaultTextStyle.Render("    ") + strings.Join(parts, defaultTextStyle.Render(strings.Repeat(" ", statsTableColumnGap)))
}

func statsTableColumnWidths(columns []statsTableColumn, rows []statsTableRow, maxWidth int) []int {
	widths := make([]int, len(columns))
	minWidths := make([]int, len(columns))
	for i, column := range columns {
		minWidth := column.MinWidth
		if minWidth <= 0 {
			minWidth = 1
		}
		minWidths[i] = minWidth
		widths[i] = maxInt(minWidth, lipgloss.Width(column.Header))
	}
	for _, row := range rows {
		if row.Divider {
			continue
		}
		for i := range columns {
			cell := ""
			if i < len(row.Cells) {
				cell = row.Cells[i]
			}
			widths[i] = maxInt(widths[i], lipgloss.Width(cell))
		}
	}
	available := maxWidth - statsTableColumnGap*(len(columns)-1)
	if available <= 0 {
		return widths
	}
	zeroMinWidths := make([]int, len(minWidths))
	for sumInts(minWidths) > available {
		index := widestShrinkableColumn(minWidths, zeroMinWidths)
		if index < 0 {
			break
		}
		minWidths[index]--
	}
	for sumInts(widths) > available {
		index := widestShrinkableColumn(widths, minWidths)
		if index < 0 {
			break
		}
		widths[index]--
	}
	for sumInts(widths) < available {
		for i := range widths {
			if sumInts(widths) >= available {
				break
			}
			widths[i]++
		}
	}
	return widths
}

func widestShrinkableColumn(widths []int, minWidths []int) int {
	index := -1
	maxWidth := 0
	for i, width := range widths {
		if width <= minWidths[i] {
			continue
		}
		if width > maxWidth {
			maxWidth = width
			index = i
		}
	}
	return index
}

func statsTableContentWidth(widths []int) int {
	if len(widths) == 0 {
		return 0
	}
	return sumInts(widths) + statsTableColumnGap*(len(widths)-1)
}

func renderStatsTableLine(columns []statsTableColumn, widths []int, cells []string) string {
	parts := make([]string, len(columns))
	for i, column := range columns {
		cell := ""
		if i < len(cells) {
			cell = cells[i]
		}
		parts[i] = renderStatsTableCell(cell, widths[i], column.AlignRight, column.PathAware, column.Style)
	}
	return defaultTextStyle.Render("    ") + strings.Join(parts, defaultTextStyle.Render(strings.Repeat(" ", statsTableColumnGap)))
}

func renderStatsTableCell(value string, width int, alignRight bool, pathAware bool, style lipgloss.Style) string {
	truncated := truncateDisplayWidth(value, width)
	if pathAware {
		truncated = truncatePathAware(value, width)
	}
	visible := lipgloss.Width(truncated)
	padding := width - visible
	if padding < 0 {
		padding = 0
	}
	pad := defaultTextStyle.Render(strings.Repeat(" ", padding))
	if alignRight {
		return pad + style.Render(truncated)
	}
	return style.Render(truncated) + pad
}

func statsTableDividerLine(width int) string {
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#191919"))
	return defaultTextStyle.Render("    ") + dividerStyle.Render(strings.Repeat("┈", width))
}

func metricsDividerLine() string {
	return statsTableDividerLine(statsTableMaxWidth)
}

func truncateDisplayWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		for _, r := range value {
			return string(r)
		}
		return ""
	}
	var b strings.Builder
	currentWidth := 0
	for _, r := range value {
		runeWidth := lipgloss.Width(string(r))
		if currentWidth+runeWidth > width-1 {
			break
		}
		b.WriteRune(r)
		currentWidth += runeWidth
	}
	if b.Len() == 0 {
		return "…"
	}
	return b.String() + "…"
}

func sumInts(values []int) int {
	total := 0
	for _, value := range values {
		total += value
	}
	return total
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func styledMetricTriple(labelA string, valueA string, labelB string, valueB string, labelC string, valueC string) string {
	return renderThreeColumns(labelA, valueA, metricColumnWidth, labelB, valueB, metricColumnWidth, labelC, valueC, metricColumnWidth)
}

func renderTwoColumns(labelA string, valueA string, widthA int, labelB string, valueB string, widthB int) string {
	return renderColumn(labelA, valueA, widthA) + renderColumn(labelB, valueB, widthB)
}

func renderThreeColumns(labelA string, valueA string, widthA int, labelB string, valueB string, widthB int, labelC string, valueC string, widthC int) string {
	return renderColumn(labelA, valueA, widthA) + renderColumn(labelB, valueB, widthB) + renderColumn(labelC, valueC, widthC)
}

func renderColumn(label string, value string, width int) string {
	rendered := defaultTextStyle.Render(label) + valueText(value)
	return padStyledText(rendered, lipgloss.Width(label)+lipgloss.Width(value), width)
}

func (m Model) renderHeatmapLine(days []stats.Day) string {
	var b strings.Builder
	todayKey := heatmapDayKey(time.Now())
	for i, day := range days {
		if i > 0 {
			if i%7 == 0 {
				b.WriteByte(' ')
			}
		}
		b.WriteString(m.renderHeatmapCell(day, heatmapDayKey(day.Date) == todayKey))
	}
	return b.String()
}

func heatmapDayKey(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02")
}

func (m Model) renderHeatmapCell(day stats.Day, isToday bool) string {
	level := m.activityLevel(day)
	return m.renderHeatmapLevelCell(level, isToday)
}

func (m Model) renderHeatmapLevelCell(level int, isToday bool) string {
	char := '·'
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#303030"))
	switch level {
	case 1:
		char = '░'
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#505050"))
	case 2:
		char = '▓'
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#787878"))
	case 3:
		char = '█'
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#B8B8B8"))
	}
	if isToday {
		switch level {
		case 0:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#5A3A00"))
		case 1:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A5400"))
		case 2:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#C97300"))
		case 3:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
		}
	}
	return style.Render(string(char))
}

func (m Model) renderMonthDailyHeatmapCell(level int, isSelected bool) string {
	char := '·'
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#303030"))
	switch level {
	case 1:
		char = '░'
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#505050"))
	case 2:
		char = '▓'
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#787878"))
	case 3:
		char = '█'
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#B8B8B8"))
	}
	if isSelected {
		switch level {
		case 0:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#5A3A00"))
		case 1:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#8A5400"))
		case 2:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#C97300"))
		case 3:
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
		}
	}
	return style.Width(2).Align(lipgloss.Center).Render(strings.Repeat(string(char), 2))
}

func (m Model) monthDailyHeatmapLevel(day stats.DailySummary) int {
	if day.Tokens >= m.statsConfig.HighTokens {
		return 3
	}
	if day.Tokens >= m.statsConfig.MediumTokens {
		return 2
	}
	if day.Tokens > 0 {
		return 1
	}
	return 0
}

func (m Model) activityLevel(day stats.Day) int {
	if day.Tokens >= m.statsConfig.HighTokens {
		return 3
	}
	if day.Tokens >= m.statsConfig.MediumTokens {
		return 2
	}
	if isActive(day) {
		return 1
	}
	return 0
}

func sparklineLevel(tokens int64, step int64) int {
	if tokens <= 0 {
		return 0
	}
	if step <= 0 {
		return 7
	}
	level := int((tokens-1)/step) + 1
	if level > 7 {
		return 7
	}
	return level
}

var sparklineChars = [8]rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

var sparklineTodayColors = [8]string{
	"#3F2800", // level 0: inactive baseline for today
	"#3F2800", // level 1
	"#563600", // level 2
	"#6C4400", // level 3
	"#966400", // level 4
	"#AA7200", // level 5
	"#D48600", // level 6
	"#FF9900", // level 7
}

var sparklineYesterdayColors = [8]string{
	"#303030", // level 0: inactive
	"#404040", // level 1
	"#505050", // level 2
	"#606060", // level 3
	"#707070", // level 4
	"#808080", // level 5
	"#989898", // level 6
	"#B8B8B8", // level 7
}

const sparklineHighlightColor = "#FFAA33"

func sparklineCell(level int, isCurrentSlot bool, isToday bool) string {
	char := sparklineChars[level]
	colors := sparklineYesterdayColors
	if isToday {
		colors = sparklineTodayColors
	}
	color := colors[level]
	if isToday && isCurrentSlot && level > 0 {
		color = sparklineHighlightColor
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(string(char))
}

func (m Model) render24hSparkline(report stats.Report) string {
	return m.render24hSparklineAt(report, time.Now())
}

func rolling24hCompressedTodayStartIndex(now time.Time) int {
	return 23 - now.Hour()
}

func (m Model) render24hSparklineAt(report stats.Report, now time.Time) string {
	if m.isNarrowLayout() || m.launcherVisualWidth() < minSparklineWidth {
		return ""
	}

	slots := report.Rolling24hSlots
	// Hourly heatmap cells aggregate two half-hour slots, so calibrate against
	// a focused 2-hour block represented as 2 hourly cells.
	slotHigh := m.statsConfig.HighTokens / 2
	if slotHigh <= 0 {
		slotHigh = DefaultActivityHighTokens / 2
	}
	step := slotHigh / 7
	if step <= 0 {
		step = 1
	}

	var b strings.Builder
	compressedTodayStart := rolling24hCompressedTodayStartIndex(now)
	for i := 0; i < 24; i++ {
		if i > 0 && i%6 == 0 {
			b.WriteByte(' ')
		}
		merged := slots[i*2] + slots[i*2+1]
		level := sparklineLevel(merged, step)
		b.WriteString(sparklineCell(level, i == 23, i >= compressedTodayStart))
	}
	return b.String()
}

func isActive(day stats.Day) bool {
	return day.AssistantMessages > 0 || day.ToolCalls > 0 || day.StepFinishes > 0
}

func isAgent(day stats.Day) bool {
	return day.Subtasks >= 1
}

func isAgentHeavy(day stats.Day) bool {
	return day.Subtasks >= 2
}

func (m Model) renderStatsView() string {
	lines := m.statsContentLines()
	start, end := m.visibleStatsRange(len(lines))
	help := renderStatsHelpLine(m.layoutWidth())
	if m.statsTab == 1 {
		if m.dailyDetailMode {
			help = renderDailyDetailHelpLine(m.layoutWidth())
		} else {
			help = renderDailyMonthListHelpLine(m.layoutWidth())
		}
	} else if m.statsTab == 2 {
		if m.monthlyDetailMode {
			help = renderMonthlyDetailHelpLine(m.layoutWidth())
		} else {
			help = renderMonthlyListHelpLine(m.layoutWidth())
		}
	}
	parts := []string{
		m.renderTopBadge(),
		m.renderStatsTabs() + "\n" + strings.Join(lines[start:end], "\n"),
		help,
	}
	return strings.Join(filterNonEmpty(parts), "\n\n")
}

func (m Model) statsContentLines() []string {
	if m.statsTab == 0 && m.currentStatsLoading() && len(m.currentReport().Days) == 0 {
		return []string{"Loading stats..."}
	}
	switch m.statsTab {
	case 0:
		return m.renderOverviewLines()
	case 1:
		if m.dailyDetailMode {
			if m.currentWindowLoading() && m.currentWindowReport().Label == "" {
				return []string{"Loading stats..."}
			}
			return m.renderDailyDetailLines(m.currentWindowReport())
		}
		report := m.currentMonthDaily()
		if m.currentMonthDailyLoading() && report.MonthStart.IsZero() {
			return []string{renderSubSectionHeader(m.currentDailyMonth().Format("2006-01"), todaySectionTitleStyle)}
		}
		return m.renderMonthDailyLines(report)
	case 2:
		report := m.currentYearMonthly()
		if m.monthlyDetailMode {
			if m.currentWindowLoading() && m.currentWindowReport().Label == "" {
				return []string{"Loading stats..."}
			}
			return m.renderYearMonthlyDetailLines(report, m.currentWindowReport())
		}
		if m.currentYearMonthlyLoading() && len(report.Months) == 0 {
			return []string{renderSubSectionHeader(m.currentMonthlySelection().Format("2006-01"), todaySectionTitleStyle)}
		}
		return m.renderYearMonthlyLines(report)
	default:
		if m.currentWindowLoading() && m.currentWindowReport().Label == "" {
			return []string{"Loading stats..."}
		}
		return m.renderWindowLines(m.currentWindowReport())
	}
}

func (m Model) currentStatsLoading() bool {
	if m.projectScope {
		return m.projectStatsLoading
	}
	return m.globalStatsLoading
}

func (m Model) currentWindowReport() stats.WindowReport {
	if m.projectScope {
		if m.statsTab == 1 {
			return m.projectDaily
		}
		return m.projectMonthly
	}
	if m.statsTab == 1 {
		return m.globalDaily
	}
	return m.globalMonthly
}

func (m Model) currentWindowLoading() bool {
	if m.projectScope {
		if m.statsTab == 1 {
			return m.projectDailyLoading
		}
		return m.projectMonthlyLoading
	}
	if m.statsTab == 1 {
		return m.globalDailyLoading
	}
	return m.globalMonthlyLoading
}

func (m Model) currentMonthDaily() stats.MonthDailyReport {
	if m.projectScope {
		return m.projectMonthDaily
	}
	return m.globalMonthDaily
}

func (m Model) currentMonthDailyLoading() bool {
	if m.projectScope {
		return m.projectMonthDailyLoading
	}
	return m.globalMonthDailyLoading
}

func (m Model) renderStatsTabs() string {
	titles := statsTabTitles()
	if len(titles) == 0 {
		return ""
	}
	targetWidth := m.layoutWidth()
	labels := make([]string, 0, len(titles))
	indicators := make([]string, 0, len(titles))
	for i, title := range titles {
		labels = append(labels, renderStatsTabLabel(title, i == m.statsTab))
		if i == m.statsTab {
			indicators = append(indicators, statsTabIndicatorStyle.Render(strings.Repeat("▔", statsTabWidth)))
			continue
		}
		indicators = append(indicators, strings.Repeat(" ", statsTabWidth))
	}
	left := strings.Join(labels, "")
	if m.isNarrowLayout() {
		meta := statsTabMetaStyle.Width(targetWidth).Align(lipgloss.Right).Render(m.statsTabMeta())
		return left + "\n" + meta
	}
	metaWidth := max(0, targetWidth-lipgloss.Width(left))
	meta := statsTabMetaStyle.Width(metaWidth).Align(lipgloss.Right).Render(m.statsTabMeta())
	indicatorRow := strings.Join(indicators, "") + strings.Repeat(" ", metaWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, meta) + "\n" + indicatorRow
}

func renderStatsTabLabel(title string, active bool) string {
	style := statsTabStyle
	if active {
		style = statsTabActiveStyle
	}
	return style.Padding(0, 0).Width(statsTabWidth).Align(lipgloss.Center).Render("   " + title + "   ")
}

func (m Model) statsTabMeta() string {
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("#202020")).Render("|")
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render(strings.ToUpper(m.statsScopeLabel()))
	return " " + divider + " " + label + " "
}

func (m Model) statsScopeLabel() string {
	if m.projectScope {
		return "project"
	}
	return "global"
}

func (m Model) statsTabDateRange() string {
	report := m.currentReport()
	if m.statsTab == 0 {
		if len(report.Days) == 0 {
			return "-- ~"
		}
		return report.Days[0].Date.Format("2006-01-02") + " ~"
	}
	if m.statsTab != 0 {
		if m.statsTab == 1 {
			if m.dailyDetailMode {
				return m.currentDailyDate().Format("2006-01-02")
			}
			return m.currentDailyMonth().Format("2006-01")
		}
		if m.statsTab == 2 {
			return m.currentMonthlySelection().Format("2006-01")
		}
		window := m.currentWindowReport()
		if !window.Start.IsZero() && !window.End.IsZero() {
			if window.Label == "Monthly" {
				return window.Start.Format("2006-01")
			}
			if window.Label == "Daily" {
				return window.Start.Format("2006-01-02")
			}
			return formatStatsDateRange(window.Start, window.End.Add(-time.Second))
		}
	}
	if len(report.Days) == 0 {
		return "--~--"
	}
	return formatStatsDateRange(report.Days[0].Date, report.Days[len(report.Days)-1].Date)
}

func formatStatsDateRange(start time.Time, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return "--~--"
	}
	return start.Format("2006-01-02") + "~" + end.Format("2006-01-02")
}

func (m Model) renderOverviewLines() []string {
	report := m.currentReport()
	lines := strings.Split(m.renderLauncherAnalytics(), "\n")
	if !m.isNarrowLayout() {
		lines = append(lines,
			"",
			renderSubSectionHeader("Trends", habitSectionTitleStyle),
			"",
			bulletLine(styledTrendLine("• tokens ", renderValueTrend(report.Days, func(day stats.Day) float64 { return float64(day.Tokens) }))),
			bulletLine(styledTrendLine("• cost ", renderValueTrend(report.Days, func(day stats.Day) float64 { return day.Cost }))),
			bulletLine(styledTrendLine("• hours ", renderValueTrend(report.Days, func(day stats.Day) float64 { return float64(day.SessionMinutes) }))),
			bulletLine(styledTrendLine("• lines ", renderValueTrend(report.Days, func(day stats.Day) float64 { return float64(day.CodeLines) }))),
			bulletLine(styledTrendLine("• files ", renderValueTrend(report.Days, func(day stats.Day) float64 { return float64(day.ChangedFiles) }))),
			"",
			bulletLine(styledMetricLine("• reasoning ", fmt.Sprintf("%s today | %s baseline", formatPercent(report.TodayReasoningShare), formatPercent(report.RecentReasoningShare)))),
		)
	}
	lines = append(lines,
		"",
		activitySectionHeader("Activity - Models", report.UniqueModelCount),
	)
	lines = append(lines, m.renderModelUsageLines(report.TopModels, report.TotalModelTokens, report.TotalModelCost)...)
	if !m.projectScope {
		lines = append(lines,
			"",
			activitySectionHeader("Activity - Projects", report.UniqueProjectCount),
		)
		lines = append(lines, m.renderProjectUsageLines(report.TopProjects, report.ThirtyDayTokens, report.TotalProjectCost)...)
	}
	lines = append(lines,
		"",
		activitySectionHeader("Activity - Agents", report.UniqueAgentModelCount),
	)
	lines = append(lines, m.renderAgentModelUsageLines(report.TopAgentModels, int64(report.TotalAgentModelCalls))...)
	lines = append(lines,
		"",
		activitySectionHeader("Activity - Skills", report.UniqueSkillCount),
	)
	lines = append(lines, m.renderUsageLines("count", report.TopSkills, int64(report.TotalSkillCalls))...)
	lines = append(lines,
		"",
		activitySectionHeader("Activity - Tools", report.UniqueToolCount),
	)
	lines = append(lines, m.renderUsageLines("count", report.TopTools, int64(report.TotalToolCalls))...)
	return lines
}

func (m Model) renderUsageLines(metricHeader string, items []stats.UsageCount, total int64) []string {
	metricFormatter := formatUsageMetric
	if usageItemsUseAmounts(items) {
		metricFormatter = formatSummaryTokens
	}
	compactPathColumns := m.isNarrowLayout() && usageItemsLookLikePaths(items)
	metricLabel := metricHeader
	shareLabel := "share"
	metricMinWidth := 7
	shareMinWidth := 5
	if compactPathColumns {
		if metricHeader == "tokens" {
			metricLabel = "tok"
		} else if metricHeader == "count" {
			metricLabel = "cnt"
		}
		shareLabel = "%"
		metricMinWidth = 3
		shareMinWidth = 4
	}
	shareCell := func(value int64, top int64) string {
		share := formatUsageShare(value, total)
		if m.isNarrowLayout() {
			return share
		}
		return renderUsageBar(value, top, usageBarWidth) + " " + share
	}
	placeholderShare := func(value string) string {
		if m.isNarrowLayout() {
			return value
		}
		return strings.Repeat("·", usageBarWidth) + " " + value
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 12, PathAware: compactPathColumns, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, Style: statsValueTextStyle},
	}
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", placeholderShare("--")}}}
		if total > 0 {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", metricFormatter(total), placeholderShare("100%")}})
		}
		return renderStatsTable(columns, rows, m.statsTableMaxWidth())
	}
	visibleItems := items
	if len(visibleItems) > maxActivityItems {
		visibleItems = visibleItems[:maxActivityItems]
	}
	top := usageMetric(visibleItems[0])
	showOthers := len(items) > maxActivityItems && total > 0
	othersMetric := total
	rows := make([]statsTableRow, 0, len(visibleItems)+2)
	for _, item := range visibleItems {
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		rows = append(rows, statsTableRow{Cells: []string{item.Name, metricFormatter(itemMetric), shareCell(itemMetric, top)}})
	}
	if showOthers && othersMetric > 0 {
		rows = append(rows, statsTableRow{Cells: []string{"others", metricFormatter(othersMetric), shareCell(othersMetric, top)}})
	}
	if total > 0 {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", metricFormatter(total), placeholderShare("100%")}})
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func (m Model) renderProjectUsageLines(items []stats.UsageCount, total int64, totalCost float64) []string {
	compactPathColumns := m.isNarrowLayout() && usageItemsLookLikePaths(items)
	metricLabel := "tokens"
	costLabel := "cost"
	shareLabel := "share"
	metricMinWidth := 7
	costMinWidth := 6
	shareMinWidth := 5
	if compactPathColumns {
		metricLabel = "tok"
		costLabel = "$"
		shareLabel = "%"
		metricMinWidth = 3
		costMinWidth = 4
		shareMinWidth = 4
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 12, PathAware: compactPathColumns, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: costLabel, MinWidth: costMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, AlignRight: true, Style: statsValueTextStyle},
	}
	showTotal := total > 0 || totalCost > 0
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", "--", "--"}}}
		if showTotal {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", formatSummaryTokens(total), formatSummaryCurrency(totalCost), formatUsageShare(total, total)}})
		}
		return renderStatsTable(columns, rows, m.statsTableMaxWidth())
	}
	visibleItems := items
	if len(visibleItems) > maxActivityItems {
		visibleItems = visibleItems[:maxActivityItems]
	}
	showOthers := len(items) > maxActivityItems && (total > 0 || totalCost > 0)
	othersMetric := total
	othersCost := totalCost
	rows := make([]statsTableRow, 0, len(visibleItems)+2)
	for _, item := range visibleItems {
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		othersCost -= item.Cost
		rows = append(rows, statsTableRow{Cells: []string{item.Name, formatSummaryTokens(itemMetric), formatSummaryCurrency(item.Cost), formatUsageShare(itemMetric, total)}})
	}
	if showOthers && (othersMetric > 0 || othersCost > 0) {
		rows = append(rows, statsTableRow{Cells: []string{"others", formatSummaryTokens(othersMetric), formatSummaryCurrency(othersCost), formatUsageShare(othersMetric, total)}})
	}
	if showTotal {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", formatSummaryTokens(total), formatSummaryCurrency(totalCost), formatUsageShare(total, total)}})
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func (m Model) renderAgentModelUsageLines(items []stats.UsageCount, total int64) []string {
	metricLabel := "count"
	shareLabel := "share"
	metricMinWidth := 7
	shareMinWidth := 5
	if m.isNarrowLayout() {
		metricLabel = "cnt"
		shareLabel = "%"
		metricMinWidth = 3
		shareMinWidth = 4
	}
	shareCell := func(value int64, top int64) string {
		share := formatUsageShare(value, total)
		if m.isNarrowLayout() {
			return share
		}
		return renderUsageBar(value, top, usageBarWidth) + " " + share
	}
	placeholderShare := func(value string) string {
		if m.isNarrowLayout() {
			return value
		}
		return strings.Repeat("·", usageBarWidth) + " " + value
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 10, Style: defaultTextStyle},
		{Header: "model", MinWidth: 18, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, Style: statsValueTextStyle},
	}
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", "--", placeholderShare("--")}}}
		if total > 0 {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatUsageMetric(total), placeholderShare("100%")}})
		}
		return renderStatsTable(columns, rows, m.statsTableMaxWidth())
	}
	visibleItems := items
	if len(visibleItems) > maxActivityItems {
		visibleItems = visibleItems[:maxActivityItems]
	}
	top := usageMetric(visibleItems[0])
	showOthers := len(items) > maxActivityItems && total > 0
	othersMetric := total
	rows := make([]statsTableRow, 0, len(visibleItems)+2)
	for _, item := range visibleItems {
		agent, model := splitAgentModelUsageKey(item.Name)
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		rows = append(rows, statsTableRow{Cells: []string{agent, model, formatUsageMetric(itemMetric), shareCell(itemMetric, top)}})
	}
	if showOthers && othersMetric > 0 {
		rows = append(rows, statsTableRow{Cells: []string{"others", "", formatUsageMetric(othersMetric), shareCell(othersMetric, top)}})
	}
	if total > 0 {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatUsageMetric(total), placeholderShare("100%")}})
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func (m Model) renderModelUsageLines(items []stats.UsageCount, total int64, totalCost float64) []string {
	metricLabel := "tokens"
	costLabel := "cost"
	shareLabel := "share"
	metricMinWidth := 7
	costMinWidth := 6
	shareMinWidth := 5
	if m.isNarrowLayout() {
		metricLabel = "tok"
		costLabel = "$"
		shareLabel = "%"
		metricMinWidth = 3
		costMinWidth = 4
		shareMinWidth = 4
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 18, Style: defaultTextStyle},
		{Header: "provider", MinWidth: 10, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: costLabel, MinWidth: costMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, AlignRight: true, Style: statsValueTextStyle},
	}
	showTotal := total > 0 || totalCost > 0
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"-", "--", "--", "--", "--"}}}
		if showTotal {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatSummaryTokens(total), formatSummaryCurrency(totalCost), formatUsageShare(total, total)}})
		}
		return renderStatsTable(columns, rows, m.statsTableMaxWidth())
	}
	visibleItems := items
	if len(visibleItems) > maxActivityItems {
		visibleItems = visibleItems[:maxActivityItems]
	}
	showOthers := len(items) > maxActivityItems && (total > 0 || totalCost > 0)
	othersMetric := total
	othersCost := totalCost
	rows := make([]statsTableRow, 0, len(visibleItems)+2)
	for _, item := range visibleItems {
		provider, model := splitProviderModelUsageKey(item.Name)
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		othersCost -= item.Cost
		rows = append(rows, statsTableRow{Cells: []string{model, provider, formatSummaryTokens(itemMetric), formatSummaryCurrency(item.Cost), formatUsageShare(itemMetric, total)}})
	}
	if showOthers && (othersMetric > 0 || othersCost > 0) {
		rows = append(rows, statsTableRow{Cells: []string{"others", "", formatSummaryTokens(othersMetric), formatSummaryCurrency(othersCost), formatUsageShare(othersMetric, total)}})
	}
	if showTotal {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatSummaryTokens(total), formatSummaryCurrency(totalCost), formatUsageShare(total, total)}})
	}
	return renderStatsTable(columns, rows, m.statsTableMaxWidth())
}

func splitAgentModelUsageKey(value string) (string, string) {
	parts := strings.SplitN(value, "\x00", 2)
	if len(parts) != 2 {
		if strings.TrimSpace(value) == "" {
			return "--", "--"
		}
		return value, "--"
	}
	agent := strings.TrimSpace(parts[0])
	model := strings.TrimSpace(parts[1])
	if agent == "" {
		agent = "--"
	}
	if model == "" {
		model = "--"
	}
	return agent, model
}

func splitProviderModelUsageKey(value string) (string, string) {
	parts := strings.SplitN(value, "\x00", 2)
	if len(parts) != 2 {
		if strings.TrimSpace(value) == "" {
			return "--", "--"
		}
		return "--", value
	}
	provider := strings.TrimSpace(parts[0])
	model := strings.TrimSpace(parts[1])
	if provider == "" {
		provider = "--"
	}
	if model == "" {
		model = "--"
	}
	return provider, model
}

func usageItemsUseAmounts(items []stats.UsageCount) bool {
	for _, item := range items {
		if item.Amount > 0 {
			return true
		}
	}
	return false
}

func usageItemsLookLikePaths(items []stats.UsageCount) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Name == "" || !isPathLike(item.Name) {
			return false
		}
	}
	return true
}

func formatUsageMetric(value int64) string {
	return formatGroupedNumber(value)
}

func renderUsageBar(count int64, maxCount int64, width int) string {
	if width <= 0 {
		return ""
	}
	if count <= 0 || maxCount <= 0 {
		return strings.Repeat("·", width)
	}
	filled := int(math.Round((float64(count) / float64(maxCount)) * float64(width)))
	if filled < 1 {
		filled = 1
	}
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("·", width-filled)
}

func formatUsageShare(count int64, total int64) string {
	if count <= 0 || total <= 0 {
		return "--"
	}
	return fmt.Sprintf("%d%%", int(math.Round((float64(count)/float64(total))*100)))
}

func usageMetric(item stats.UsageCount) int64 {
	if item.Amount > 0 {
		return item.Amount
	}
	return int64(item.Count)
}

func (m Model) renderWindowLines(report stats.WindowReport) []string {
	title := renderSubSectionHeader(formatWindowTitle(report), todaySectionTitleStyle)
	if m.isNarrowLayout() {
		return m.renderCompactWindowLines(report)
	}
	lines := []string{title}
	lines = append(lines, renderStatsTable(windowSummaryColumns(), windowSummaryRows(report), m.statsTableMaxWidth())...)
	lines = append(lines, "")
	lines = append(lines, renderStatsTable(windowModelColumns(), windowModelTableRows(report.Models), m.statsTableMaxWidth())...)
	lines = append(lines,
		"",
		renderSubSectionHeader("Top Sessions", habitSectionTitleStyle),
	)
	lines = append(lines, renderStatsTable(windowSessionColumns(), m.windowSessionTableRows(report), m.statsTableMaxWidth())...)
	return lines
}

func (m Model) renderCompactWindowLines(report stats.WindowReport) []string {
	clamp := func(text string) string {
		return truncateDisplayWidth(text, m.layoutWidth())
	}
	bullet := func(text string) string {
		return defaultTextStyle.Render("    ") + truncateDisplayWidth(text, max(0, m.layoutWidth()-4))
	}
	lines := []string{
		clamp(renderSubSectionHeader(formatWindowTitle(report), todaySectionTitleStyle)),
		bullet("• window " + formatWindowRange(report.Start, report.End)),
		bullet("• messages " + formatGroupedInt(report.Messages)),
		bullet("• sessions " + formatGroupedInt(report.Sessions)),
		bullet("• tokens " + formatSummaryTokens(report.Tokens)),
		bullet("• cost " + formatSummaryCurrency(report.Cost)),
		"",
	}
	if len(report.Models) == 0 {
		lines = append(lines, bullet("• -"))
	} else {
		for _, model := range report.Models {
			lines = append(lines, bullet(fmt.Sprintf("• %s %s %s", blankDash(model.Model), formatSummaryTokens(model.TotalTokens), formatSummaryCurrency(model.Cost))))
		}
	}
	lines = append(lines, "", clamp(renderSubSectionHeader("Top Sessions", habitSectionTitleStyle)))
	for _, row := range m.windowSessionRows(report) {
		lines = append(lines, bullet("• "+strings.TrimSpace(strings.Join(row, " "))))
	}
	return lines
}

func formatWindowTitle(report stats.WindowReport) string {
	if report.Label == "Monthly" && !report.Start.IsZero() {
		return "Token Used"
	}
	end := report.End.Add(-time.Second)
	if !end.IsZero() {
		return "Token Used"
	}
	return "Token Used"
}

func formatWindowRange(start time.Time, end time.Time) string {
	if start.IsZero() || end.IsZero() {
		return "--"
	}
	return fmt.Sprintf("%s .. %s", start.Format("2006-01-02 15:04"), end.Add(-time.Second).Format("2006-01-02 15:04"))
}

func renderValueTrend(days []stats.Day, extract func(stats.Day) float64) string {
	if len(days) == 0 {
		return "--"
	}
	values := make([]float64, len(days))
	maxValue := 0.0
	for i, day := range days {
		values[i] = extract(day)
		if values[i] > maxValue {
			maxValue = values[i]
		}
	}
	levels := []rune{'·', '░', '▓', '█'}
	colors := []string{"#303030", "#505050", "#787878", "#B8B8B8"}
	todayColors := []string{"#5A3A00", "#8A5400", "#C97300", "#FF9900"}
	var b strings.Builder
	for i, value := range values {
		if maxValue == 0 {
			palette := colors
			if i == len(values)-1 {
				palette = todayColors
			}
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(palette[0])).Render("·"))
			continue
		}
		index := int(math.Round((value / maxValue) * float64(len(levels)-1)))
		if index < 0 {
			index = 0
		}
		if index >= len(levels) {
			index = len(levels) - 1
		}
		palette := colors
		if i == len(values)-1 {
			palette = todayColors
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(palette[index])).Render(string(levels[index])))
	}
	return b.String()
}

func formatCurrencyWithTop(today float64, days []stats.Day) string {
	return fmt.Sprintf("%s (%s)", formatSummaryCurrency(today), formatRatioToTop(today, maxCost(days)))
}

func formatTokensWithTop(today int64, days []stats.Day) string {
	return fmt.Sprintf("%s (%s)", formatSummaryTokens(today), formatRatioToTop(float64(today), float64(maxTokens(days))))
}

func formatHoursWithTop(today int, days []stats.Day) string {
	return fmt.Sprintf("%s (%s)", formatSummaryHours(today), formatRatioToTop(float64(today), float64(maxSessionMinutes(days))))
}

func formatRolling24hHours(minutes int) string {
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("%.1f/24h", float64(minutes)/60)
}

func formatCodeLinesWithTop(today int, days []stats.Day) string {
	return fmt.Sprintf("%s (%s)", formatSummaryCodeLines(today), formatRatioToTop(float64(today), float64(maxCodeLines(days))))
}

func formatChangedFilesWithTop(today int, days []stats.Day) string {
	return fmt.Sprintf("%s (%s)", formatSummaryChangedFiles(today), formatRatioToTop(float64(today), float64(maxChangedFiles(days))))
}

func perHourRate(value float64, sessionMinutes int) float64 {
	if value <= 0 || sessionMinutes <= 0 {
		return 0
	}
	return value / (float64(sessionMinutes) / 60)
}

func maxTokensPerHour(days []stats.Day) float64 {
	maxRate := 0.0
	for _, day := range days {
		rate := perHourRate(float64(day.Tokens), day.SessionMinutes)
		if rate > maxRate {
			maxRate = rate
		}
	}
	return maxRate
}

func maxTokensPerHourDay(days []stats.Day) stats.Day {
	var maxDay stats.Day
	maxRate := 0.0
	for _, day := range days {
		rate := perHourRate(float64(day.Tokens), day.SessionMinutes)
		if rate > maxRate {
			maxRate = rate
			maxDay = day
		}
	}
	return maxDay
}

func maxCodeLinesPerHour(days []stats.Day) float64 {
	maxRate := 0.0
	for _, day := range days {
		rate := perHourRate(float64(day.CodeLines), day.SessionMinutes)
		if rate > maxRate {
			maxRate = rate
		}
	}
	return maxRate
}

func maxCodeLinesPerHourDay(days []stats.Day) stats.Day {
	var maxDay stats.Day
	maxRate := 0.0
	for _, day := range days {
		rate := perHourRate(float64(day.CodeLines), day.SessionMinutes)
		if rate > maxRate {
			maxRate = rate
			maxDay = day
		}
	}
	return maxDay
}

func formatTokensPerHourWithTop(todayTokens int64, todayMinutes int, days []stats.Day) string {
	return fmt.Sprintf("%s (%s)", formatSummaryTokensPerHour(todayTokens, todayMinutes), formatRatioToTop(perHourRate(float64(todayTokens), todayMinutes), maxTokensPerHour(days)))
}

func formatCodeLinesPerHourWithTop(today int, todayMinutes int, days []stats.Day) string {
	return fmt.Sprintf("%s (%s)", formatSummaryCodeLinesPerHour(today, todayMinutes), formatRatioToTop(perHourRate(float64(today), todayMinutes), maxCodeLinesPerHour(days)))
}

func formatSummaryCurrency(value float64) string {
	if value <= 0 {
		return "--"
	}
	return formatCurrency(value)
}

func formatSummaryTokens(value int64) string {
	if value <= 0 {
		return "--"
	}
	return formatCompactTokens(value)
}

func formatSummaryHours(minutes int) string {
	if minutes <= 0 {
		return "--"
	}
	return formatGroupedFloat(float64(minutes)/60, 1) + "h"
}

func formatSummaryCodeLines(value int) string {
	if value <= 0 {
		return "--"
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", float64(value)/1000)
	}
	return formatGroupedInt(value)
}

func formatSummaryChangedFiles(value int) string {
	if value <= 0 {
		return "--"
	}
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", float64(value)/1000)
	}
	return formatGroupedInt(value)
}

func formatSummaryTokensPerHour(value int64, sessionMinutes int) string {
	rate := perHourRate(float64(value), sessionMinutes)
	if rate <= 0 {
		return "--"
	}
	return formatCompactTokens(int64(math.Round(rate)))
}

func formatSummaryCodeLinesPerHour(value int, sessionMinutes int) string {
	rate := perHourRate(float64(value), sessionMinutes)
	if rate <= 0 {
		return "--"
	}
	return formatSummaryCodeLines(int(math.Round(rate)))
}

func formatCurrency(value float64) string {
	return "$" + formatGroupedFloat(value, 2)
}

func formatCompactTokens(value int64) string {
	if value >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(value)/1_000_000)
	}
	if value >= 1000 {
		return fmt.Sprintf("%dk", int(math.Round(float64(value)/1000)))
	}
	return formatGroupedNumber(value)
}

func formatGroupedInt(value int) string {
	return formatGroupedNumber(int64(value))
}

func formatGroupedNumber(value int64) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	s := strconv.FormatInt(value, 10)
	if len(s) <= 3 {
		if negative {
			return "-" + s
		}
		return s
	}
	var b strings.Builder
	if negative {
		b.WriteByte('-')
	}
	firstGroupLen := len(s) % 3
	if firstGroupLen == 0 {
		firstGroupLen = 3
	}
	b.WriteString(s[:firstGroupLen])
	for i := firstGroupLen; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func formatGroupedFloat(value float64, decimals int) string {
	negative := value < 0
	if negative {
		value = -value
	}
	raw := strconv.FormatFloat(value, 'f', decimals, 64)
	parts := strings.SplitN(raw, ".", 2)
	result := formatGroupedNumber(mustParseInt64(parts[0]))
	if len(parts) == 2 {
		result += "." + parts[1]
	}
	if negative {
		return "-" + result
	}
	return result
}

func mustParseInt64(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func formatDelta(today float64, yesterday float64) string {
	if yesterday == 0 {
		if today == 0 {
			return "--"
		}
		return "new"
	}
	change := ((today - yesterday) / yesterday) * 100
	if change > 0 {
		return fmt.Sprintf("↑%.0f%%", math.Abs(change))
	}
	if change < 0 {
		return fmt.Sprintf("↓%.0f%%", math.Abs(change))
	}
	return "--"
}

func formatRatioToTop(today float64, maxValue float64) string {
	if today <= 0 || maxValue <= 0 {
		return "--"
	}
	if maxValue <= 0 {
		return "--"
	}
	if today >= maxValue {
		return "max"
	}
	ratio := (today / maxValue) * 100
	return fmt.Sprintf("%.0f%%", math.Abs(ratio))
}

func formatPercent(value float64) string {
	return fmt.Sprintf("%.0f%%", value*100)
}

func formatDay(day time.Time) string {
	if day.IsZero() {
		return "--"
	}
	return day.Format("2006-01-02")
}

func windowSummaryColumns() []statsTableColumn {
	return []statsTableColumn{
		{Header: "window", MinWidth: 16, Style: defaultTextStyle},
		{Header: "messages", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "sessions", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "tokens", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "cost", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
	}
}

func windowSummaryRows(report stats.WindowReport) []statsTableRow {
	return []statsTableRow{{Cells: []string{
		formatWindowRange(report.Start, report.End),
		formatGroupedInt(report.Messages),
		formatGroupedInt(report.Sessions),
		formatSummaryTokens(report.Tokens),
		formatSummaryCurrency(report.Cost),
	}}}
}

func windowModelColumns() []statsTableColumn {
	return []statsTableColumn{
		{Header: "", MinWidth: 12, Style: defaultTextStyle},
		{Header: "input", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "output", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "c.read", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "c.write", MinWidth: 7, AlignRight: true, Style: statsValueTextStyle},
		{Header: "reasoning", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "total", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "cost", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
	}
}

func windowModelTableRows(models []stats.ModelUsage) []statsTableRow {
	rows := make([]statsTableRow, 0, len(models))
	total := stats.ModelUsage{}
	for _, model := range models {
		total.InputTokens += model.InputTokens
		total.OutputTokens += model.OutputTokens
		total.CacheReadTokens += model.CacheReadTokens
		total.CacheWriteTokens += model.CacheWriteTokens
		total.ReasoningTokens += model.ReasoningTokens
		total.TotalTokens += model.TotalTokens
		total.Cost += model.Cost
		rows = append(rows, statsTableRow{Cells: []string{
			windowModelDisplayName(model.Model),
			formatSummaryTokens(model.InputTokens),
			formatSummaryTokens(model.OutputTokens),
			formatSummaryTokens(model.CacheReadTokens),
			formatSummaryTokens(model.CacheWriteTokens),
			formatSummaryTokens(model.ReasoningTokens),
			formatSummaryTokens(model.TotalTokens),
			formatSummaryCurrency(model.Cost),
		}})
	}
	if len(rows) == 0 {
		return []statsTableRow{{Cells: []string{"-", "-", "-", "-", "-", "-", "-", "-"}}}
	}
	rows = append(rows,
		statsTableRow{Divider: true},
		statsTableRow{Cells: []string{
			"Total",
			formatSummaryTokens(total.InputTokens),
			formatSummaryTokens(total.OutputTokens),
			formatSummaryTokens(total.CacheReadTokens),
			formatSummaryTokens(total.CacheWriteTokens),
			formatSummaryTokens(total.ReasoningTokens),
			formatSummaryTokens(total.TotalTokens),
			formatSummaryCurrency(total.Cost),
		}},
	)
	return rows
}

func windowModelDisplayName(value string) string {
	provider, model := splitProviderModelUsageKey(value)
	if provider == "--" || strings.TrimSpace(provider) == "" {
		return model
	}
	return provider + "/" + model
}

func windowSessionColumns() []statsTableColumn {
	return []statsTableColumn{
		{Header: "", MinWidth: 1, Style: defaultTextStyle},
		{Header: "", MinWidth: 12, Style: defaultTextStyle},
		{Header: "msgs", MinWidth: 5, AlignRight: true, Style: statsValueTextStyle},
		{Header: "tokens", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "cost", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "title", MinWidth: 10, Style: defaultTextStyle},
	}
}

func windowModelHeaders() []string {
	return []string{"Model", "Input", "Output", "C.Read", "C.Write", "Reasoning", "Total", "Cost"}
}

func windowModelRows(models []stats.ModelUsage) [][]string {
	rows := make([][]string, 0, len(models))
	for _, model := range models {
		rows = append(rows, []string{
			model.Model,
			formatSummaryTokens(model.InputTokens),
			formatSummaryTokens(model.OutputTokens),
			formatSummaryTokens(model.CacheReadTokens),
			formatSummaryTokens(model.CacheWriteTokens),
			formatSummaryTokens(model.ReasoningTokens),
			formatSummaryTokens(model.TotalTokens),
			formatSummaryCurrency(model.Cost),
		})
	}
	if len(rows) == 0 {
		return [][]string{{"-", "-", "-", "-", "-", "-", "-", "-"}}
	}
	return rows
}

func windowSessionHeaders() []string {
	return []string{"Current", "Session", "Msgs", "Tokens", "Cost", "Title"}
}

func (m Model) windowSessionRows(report stats.WindowReport) [][]string {
	rows := make([][]string, 0, len(report.TopSessions))
	for _, session := range report.TopSessions {
		currentMark := ""
		if m.session.ID != "" && session.ID == m.session.ID {
			currentMark = "*"
		}
		rows = append(rows, []string{currentMark, session.ID, formatGroupedInt(session.Messages), formatSummaryTokens(session.Tokens), formatSummaryCurrency(session.Cost), blankDash(session.Title)})
	}
	if len(rows) == 0 {
		rows = [][]string{{"", "-", "-", "-", "-", "-"}}
	}
	return rows
}

func (m Model) windowSessionTableRows(report stats.WindowReport) []statsTableRow {
	stringRows := m.windowSessionRows(report)
	rows := make([]statsTableRow, 0, len(stringRows))
	for _, row := range stringRows {
		rows = append(rows, statsTableRow{Cells: row})
	}
	return rows
}

func blankDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func tableString(headers []string, rows [][]string, rightAligned map[int]bool) string {
	return tableStringWithMaxWidth(headers, rows, rightAligned, maxLayoutWidth)
}

func tableStringWithMaxWidth(headers []string, rows [][]string, rightAligned map[int]bool, maxWidth int) string {
	widths := make([]int, len(headers))
	minWidths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = lipgloss.Width(header)
		minWidths[i] = maxInt(3, lipgloss.Width(truncateDisplayWidth(header, 3)))
	}
	for _, row := range rows {
		for i, cell := range row {
			cellWidth := lipgloss.Width(cell)
			if cellWidth > widths[i] {
				widths[i] = cellWidth
			}
		}
	}
	totalWidth := func(widths []int) int {
		if len(widths) == 0 {
			return 0
		}
		return 4 + sumInts(widths) + (len(widths)-1)*3
	}
	if maxWidth > 0 {
		zeroMinWidths := make([]int, len(minWidths))
		availableContentWidth := maxWidth - (4 + (len(widths)-1)*3)
		for sumInts(minWidths) > availableContentWidth {
			index := widestShrinkableColumn(minWidths, zeroMinWidths)
			if index < 0 {
				break
			}
			minWidths[index]--
		}
		for totalWidth(widths) > maxWidth {
			index := widestShrinkableColumn(widths, minWidths)
			if index < 0 {
				break
			}
			widths[index]--
		}
	}
	formatRow := func(row []string) string {
		parts := make([]string, len(row))
		for i, cell := range row {
			cell = truncatePathAware(cell, widths[i])
			if rightAligned[i] {
				padding := max(0, widths[i]-lipgloss.Width(cell))
				parts[i] = strings.Repeat(" ", padding) + cell
			} else {
				padding := max(0, widths[i]-lipgloss.Width(cell))
				parts[i] = cell + strings.Repeat(" ", padding)
			}
		}
		return "| " + strings.Join(parts, " | ") + " |"
	}
	sep := make([]string, len(headers))
	for i := range headers {
		sep[i] = strings.Repeat("-", widths[i])
	}
	lines := []string{formatRow(headers), "| " + strings.Join(sep, " | ") + " |"}
	for _, row := range rows {
		lines = append(lines, formatRow(row))
	}
	return strings.Join(lines, "\n")
}

// renderMonthDailyLines renders month-list view with daily summaries and focus tags
func (m Model) renderMonthDailyLines(report stats.MonthDailyReport) []string {
	if report.MonthStart.IsZero() {
		return []string{renderSubSectionHeader(m.currentDailyMonth().Format("2006-01"), todaySectionTitleStyle)}
	}

	if m.isNarrowLayout() {
		return m.renderCompactMonthDailyLines(report)
	}

	lines := []string{
		renderSubSectionHeader(m.renderMonthDailyTitle(report), todaySectionTitleStyle),
		"",
	}
	lines = append(lines, m.renderMonthDailyHeatmapLines(report)...)
	lines = append(lines, "")
	lines = append(lines, m.renderMonthDailyMetricsLines(report)...)
	lines = append(lines, "")
	lines = append(lines, renderStatsTable(m.monthDailyColumns(), m.monthDailyTableRows(report), m.statsTableMaxWidth())...)

	return lines
}

func (m Model) renderMonthDailyHeatmapLines(report stats.MonthDailyReport) []string {
	if m.isNarrowLayout() {
		return nil
	}
	dayMap := make(map[string]stats.DailySummary, len(report.Days))
	for _, day := range report.Days {
		dayMap[heatmapDayKey(day.Date)] = day
	}
	lines := []string{defaultTextStyle.Render("  ") + m.renderMonthDailyWeekdayHeader()}
	monthStart := report.MonthStart
	monthEnd := report.MonthEnd
	firstWeekday := int(monthStart.Weekday())
	week := make([]string, 0, 7)
	for i := 0; i < firstWeekday; i++ {
		week = append(week, "  ")
	}
	selectedKey := heatmapDayKey(m.currentDailyDate())
	for day := monthStart; day.Before(monthEnd); day = day.AddDate(0, 0, 1) {
		summary := dayMap[heatmapDayKey(day)]
		cell := m.renderMonthDailyHeatmapCell(m.monthDailyHeatmapLevel(summary), heatmapDayKey(day) == selectedKey)
		week = append(week, cell)
		if len(week) == 7 {
			lines = append(lines, defaultTextStyle.Render("      ")+strings.Join(week, " "))
			week = week[:0]
		}
	}
	if len(week) > 0 {
		for len(week) < 7 {
			week = append(week, "  ")
		}
		lines = append(lines, defaultTextStyle.Render("      ")+strings.Join(week, " "))
	}
	return lines
}

func (m Model) renderMonthDailyWeekdayHeader() string {
	sundayHeaderStyle := sundayTextStyle
	if m.currentDailyDate().Weekday() == time.Sunday {
		sundayHeaderStyle = selectedSundayTextStyle
	}
	labels := []string{
		sundayHeaderStyle.Render("Su"),
		defaultTextStyle.Render("Mo"),
		defaultTextStyle.Render("Tu"),
		defaultTextStyle.Render("We"),
		defaultTextStyle.Render("Th"),
		defaultTextStyle.Render("Fr"),
		defaultTextStyle.Render("Sa"),
	}
	return defaultTextStyle.Render("    ") + strings.Join(labels, " ")
}

func renderMonthDailyDayLabel(day time.Time, selected bool) string {
	datePart := day.Format("01-02")
	weekday := day.Format("Mon")
	if day.Weekday() == time.Sunday {
		if selected {
			return datePart + " " + selectedSundayTextStyle.Render(weekday)
		}
		return datePart + " " + sundayTextStyle.Render(weekday)
	}
	return datePart + " " + weekday
}

func formatMonthDailyBestStreak(report stats.MonthDailyReport) string {
	return fmt.Sprintf("streak %dd (best)", monthDailyBestStreak(report.Days))
}

func monthDailyBestStreak(days []stats.DailySummary) int {
	best := 0
	current := 0
	for i := len(days) - 1; i >= 0; i-- {
		if isMonthDailyActive(days[i]) {
			current++
			if current > best {
				best = current
			}
			continue
		}
		current = 0
	}
	return best
}

func isMonthDailyActive(day stats.DailySummary) bool {
	return day.Messages > 0 || day.Sessions > 0 || day.Tokens > 0 || day.Cost > 0
}

func (m Model) renderMonthDailyMetricsLines(report stats.MonthDailyReport) []string {
	if m.isNarrowLayout() {
		return nil
	}
	selected := m.currentMonthDailySelectedSummary(report)
	selectedLabel := "--"
	if !selected.Date.IsZero() {
		selectedLabel = selected.Date.Format("01-02")
	}
	todayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	columns := []statsTableColumn{
		{Header: "", MinWidth: 8, Style: defaultTextStyle},
		{Header: selectedLabel, MinWidth: 14, Style: todayStyle},
		{Header: "peak day", MinWidth: 18, Style: statsValueTextStyle},
		{Header: "total", MinWidth: 12, Style: statsValueTextStyle},
	}
	peakMessages, peakMessagesDate := monthDailyPeakInt(report.Days, func(day stats.DailySummary) int { return day.Messages })
	peakSessions, peakSessionsDate := monthDailyPeakInt(report.Days, func(day stats.DailySummary) int { return day.Sessions })
	peakTokens, peakTokensDate := monthDailyPeakInt64(report.Days, func(day stats.DailySummary) int64 { return day.Tokens })
	peakCost, peakCostDate := monthDailyPeakFloat(report.Days, func(day stats.DailySummary) float64 { return day.Cost })
	rows := []statsTableRow{
		{Cells: []string{"messages", formatMonthDailyCompactIntMetricWithShare(selected.Messages, report.TotalMessages), formatPeakValue(formatCompactCount(peakMessages), peakMessagesDate), formatCompactCount(report.TotalMessages)}},
		{Cells: []string{"sessions", formatMonthDailyIntMetricWithShare(selected.Sessions, report.TotalSessions), formatPeakValue(formatGroupedInt(peakSessions), peakSessionsDate), formatGroupedInt(report.TotalSessions)}},
		{Cells: []string{"tokens", formatMonthDailyInt64MetricWithShare(selected.Tokens, report.TotalTokens), formatPeakValue(formatSummaryTokens(peakTokens), peakTokensDate), formatSummaryTokens(report.TotalTokens)}},
		{Cells: []string{"cost", formatMonthDailyFloatMetricWithShare(selected.Cost, report.TotalCost), formatPeakValue(formatSummaryCurrency(peakCost), peakCostDate), formatSummaryCurrency(report.TotalCost)}},
	}
	return append([]string{renderSubSectionHeader("Metrics", habitSectionTitleStyle)}, renderStatsTable(columns, rows, m.statsTableMaxWidth())...)
}

func (m Model) renderMonthDailySummaryMetricsLines(report stats.MonthDailyReport) []string {
	if m.isNarrowLayout() {
		return nil
	}
	columns := []statsTableColumn{
		{Header: "", MinWidth: 8, Style: defaultTextStyle},
		{Header: "peak day", MinWidth: 18, Style: statsValueTextStyle},
		{Header: "total", MinWidth: 12, Style: statsValueTextStyle},
	}
	peakMessages, peakMessagesDate := monthDailyPeakInt(report.Days, func(day stats.DailySummary) int { return day.Messages })
	peakSessions, peakSessionsDate := monthDailyPeakInt(report.Days, func(day stats.DailySummary) int { return day.Sessions })
	peakTokens, peakTokensDate := monthDailyPeakInt64(report.Days, func(day stats.DailySummary) int64 { return day.Tokens })
	peakCost, peakCostDate := monthDailyPeakFloat(report.Days, func(day stats.DailySummary) float64 { return day.Cost })
	rows := []statsTableRow{
		{Cells: []string{"messages", formatPeakValue(formatCompactCount(peakMessages), peakMessagesDate), formatCompactCount(report.TotalMessages)}},
		{Cells: []string{"sessions", formatPeakValue(formatGroupedInt(peakSessions), peakSessionsDate), formatGroupedInt(report.TotalSessions)}},
		{Cells: []string{"tokens", formatPeakValue(formatSummaryTokens(peakTokens), peakTokensDate), formatSummaryTokens(report.TotalTokens)}},
		{Cells: []string{"cost", formatPeakValue(formatSummaryCurrency(peakCost), peakCostDate), formatSummaryCurrency(report.TotalCost)}},
	}
	return append([]string{renderSubSectionHeader("Metrics", habitSectionTitleStyle)}, renderStatsTable(columns, rows, m.statsTableMaxWidth())...)
}

func (m Model) currentMonthDailySelectedSummary(report stats.MonthDailyReport) stats.DailySummary {
	selectedDate := startOfStatsDay(m.currentDailyDate())
	for _, day := range report.Days {
		if startOfStatsDay(day.Date).Equal(selectedDate) {
			return day
		}
	}
	if len(report.Days) > 0 {
		return report.Days[0]
	}
	return stats.DailySummary{Date: selectedDate}
}

func monthDailyPeakInt(days []stats.DailySummary, value func(stats.DailySummary) int) (int, time.Time) {
	var peak int
	var peakDate time.Time
	for _, day := range days {
		if current := value(day); current > peak {
			peak = current
			peakDate = day.Date
		}
	}
	return peak, peakDate
}

func monthDailyPeakInt64(days []stats.DailySummary, value func(stats.DailySummary) int64) (int64, time.Time) {
	var peak int64
	var peakDate time.Time
	for _, day := range days {
		if current := value(day); current > peak {
			peak = current
			peakDate = day.Date
		}
	}
	return peak, peakDate
}

func monthDailyPeakFloat(days []stats.DailySummary, value func(stats.DailySummary) float64) (float64, time.Time) {
	var peak float64
	var peakDate time.Time
	for _, day := range days {
		if current := value(day); current > peak {
			peak = current
			peakDate = day.Date
		}
	}
	return peak, peakDate
}

func formatMonthDailyIntMetricWithShare(value int, total int) string {
	if value <= 0 {
		return "--"
	}
	if total <= 0 {
		return formatGroupedInt(value)
	}
	return fmt.Sprintf("%s (%s)", formatGroupedInt(value), formatPercent(float64(value)/float64(total)))
}

func formatMonthDailyCompactIntMetricWithShare(value int, total int) string {
	if value <= 0 {
		return "--"
	}
	if total <= 0 {
		return formatCompactCount(value)
	}
	return fmt.Sprintf("%s (%s)", formatCompactCount(value), formatPercent(float64(value)/float64(total)))
}

func formatMonthDailyInt64MetricWithShare(value int64, total int64) string {
	if value <= 0 {
		return "--"
	}
	if total <= 0 {
		return formatSummaryTokens(value)
	}
	return fmt.Sprintf("%s (%s)", formatSummaryTokens(value), formatPercent(float64(value)/float64(total)))
}

func formatMonthDailyFloatMetricWithShare(value float64, total float64) string {
	if value <= 0 {
		return "--"
	}
	if total <= 0 {
		return formatSummaryCurrency(value)
	}
	return fmt.Sprintf("%s (%s)", formatSummaryCurrency(value), formatPercent(value/total))
}

func (m Model) renderYearMonthlyLines(report stats.YearMonthlyReport) []string {
	if len(report.Months) == 0 {
		return []string{renderSubSectionHeader(m.currentMonthlySelection().Format("2006-01"), todaySectionTitleStyle)}
	}
	if m.isNarrowLayout() {
		return m.renderCompactYearMonthlyLines(report)
	}
	titleLine := fmt.Sprintf("%s .. %s", report.Start.Format("2006-01"), report.End.AddDate(0, 0, -1).Format("2006-01"))
	right := fmt.Sprintf("active %d/%dmo • streak %dmo (best %dmo)", report.ActiveMonths, len(report.Months), report.CurrentStreak, report.BestStreak)
	lines := []string{renderSubSectionHeader(joinTitleAndMeta(titleLine, right, max(0, m.statsTableMaxWidth()-4)), todaySectionTitleStyle), ""}
	lines = append(lines, m.renderYearMonthlyGridLines(report)...)
	lines = append(lines, "")
	lines = append(lines, m.renderYearMonthlyMetricsLines(report)...)
	lines = append(lines, "")
	lines = append(lines, renderStatsTable(m.yearMonthlyColumns(), m.yearMonthlyTableRows(report), m.statsTableMaxWidth())...)
	return lines
}

func (m Model) renderCompactYearMonthlyLines(report stats.YearMonthlyReport) []string {
	if len(report.Months) == 0 {
		return []string{renderSubSectionHeader(m.currentMonthlySelection().Format("2006-01"), todaySectionTitleStyle)}
	}
	clamp := func(text string) string { return truncateDisplayWidth(text, m.layoutWidth()) }
	bullet := func(text string) string {
		return defaultTextStyle.Render("    ") + truncateDisplayWidth(text, max(0, m.layoutWidth()-4))
	}
	lines := []string{
		clamp(renderSubSectionHeader(fmt.Sprintf("%s .. %s", report.Start.Format("2006-01"), report.End.AddDate(0, 0, -1).Format("2006-01")), todaySectionTitleStyle)),
		bullet(fmt.Sprintf("active %d/%dmo | streak %dmo (best %dmo)", report.ActiveMonths, len(report.Months), report.CurrentStreak, report.BestStreak)),
		"",
	}
	for _, row := range m.yearMonthlyRows() {
		selected := statsMonthStart(row.MonthStart).Equal(m.currentMonthlySelection())
		prefix := "  "
		if selected {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s  %s  %s", prefix, row.MonthStart.Format("2006-01"), formatSummaryTokens(row.TotalTokens), formatSummaryCurrency(row.TotalCost))
		lines = append(lines, bullet(line))
	}
	return lines
}

func (m Model) renderYearMonthlyGridLines(report stats.YearMonthlyReport) []string {
	months := report.Months
	if len(months) == 0 {
		return nil
	}
	first := make([]string, 0, 6)
	second := make([]string, 0, 6)
	for i, month := range months {
		cell := m.renderYearMonthlyGridCell(month, report)
		if i < 6 {
			first = append(first, cell)
		} else {
			second = append(second, cell)
		}
	}
	lines := []string{}
	if len(first) > 0 {
		lines = append(lines, defaultTextStyle.Render("      ")+strings.Join(first, defaultTextStyle.Render("   ")))
	}
	if len(second) > 0 {
		lines = append(lines, defaultTextStyle.Render("      ")+strings.Join(second, defaultTextStyle.Render("   ")))
	}
	return lines
}

func (m Model) renderYearMonthlyGridCell(month stats.MonthlySummary, report stats.YearMonthlyReport) string {
	label := month.MonthStart.Format("01")
	matrix := strings.Repeat("■", m.yearMonthlyMatrixLevel(month, report)) + strings.Repeat("·", 4-m.yearMonthlyMatrixLevel(month, report))
	text := label + " " + matrix
	if statsMonthStart(month.MonthStart).Equal(m.currentMonthlySelection()) {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900")).Bold(true).Render(text)
	}
	if month.TotalTokens == 0 {
		return defaultTextStyle.Render(label + " " + "····")
	}
	return statsValueTextStyle.Render(text)
}

func (m Model) yearMonthlyMatrixLevel(month stats.MonthlySummary, report stats.YearMonthlyReport) int {
	if month.TotalTokens <= 0 {
		return 0
	}
	maxTokens := int64(0)
	for _, item := range report.Months {
		if item.TotalTokens > maxTokens {
			maxTokens = item.TotalTokens
		}
	}
	if maxTokens <= 0 {
		return 1
	}
	share := float64(month.TotalTokens) / float64(maxTokens)
	switch {
	case share <= 0.25:
		return 1
	case share <= 0.50:
		return 2
	case share <= 0.75:
		return 3
	default:
		return 4
	}
}

func (m Model) renderYearMonthlyMetricsLines(report stats.YearMonthlyReport) []string {
	selected := m.currentYearMonthlySelectedSummary(report)
	selectedLabel := "--"
	if !selected.MonthStart.IsZero() {
		selectedLabel = selected.MonthStart.Format("2006-01")
	}
	todayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	columns := []statsTableColumn{
		{Header: "", MinWidth: 8, Style: defaultTextStyle},
		{Header: selectedLabel, MinWidth: 14, Style: todayStyle},
		{Header: "peak month", MinWidth: 18, Style: statsValueTextStyle},
		{Header: "total", MinWidth: 12, Style: statsValueTextStyle},
	}
	peakMessages, peakMessagesMonth := yearMonthlyPeakInt(report.Months, func(month stats.MonthlySummary) int { return month.TotalMessages })
	peakSessions, peakSessionsMonth := yearMonthlyPeakInt(report.Months, func(month stats.MonthlySummary) int { return month.TotalSessions })
	peakTokens, peakTokensMonth := yearMonthlyPeakInt64(report.Months, func(month stats.MonthlySummary) int64 { return month.TotalTokens })
	peakCost, peakCostMonth := yearMonthlyPeakFloat(report.Months, func(month stats.MonthlySummary) float64 { return month.TotalCost })
	rows := []statsTableRow{
		{Cells: []string{"messages", formatMonthDailyCompactIntMetricWithShare(selected.TotalMessages, report.TotalMessages), formatPeakMonthValue(formatCompactCount(peakMessages), peakMessagesMonth), formatCompactCount(report.TotalMessages)}},
		{Cells: []string{"sessions", formatMonthDailyIntMetricWithShare(selected.TotalSessions, report.TotalSessions), formatPeakMonthValue(formatGroupedInt(peakSessions), peakSessionsMonth), formatGroupedInt(report.TotalSessions)}},
		{Cells: []string{"tokens", formatMonthDailyInt64MetricWithShare(selected.TotalTokens, report.TotalTokens), formatPeakMonthValue(formatSummaryTokens(peakTokens), peakTokensMonth), formatSummaryTokens(report.TotalTokens)}},
		{Cells: []string{"cost", formatMonthDailyFloatMetricWithShare(selected.TotalCost, report.TotalCost), formatPeakMonthValue(formatSummaryCurrency(peakCost), peakCostMonth), formatSummaryCurrency(report.TotalCost)}},
	}
	return append([]string{renderSubSectionHeader("Metrics", habitSectionTitleStyle)}, renderStatsTable(columns, rows, m.statsTableMaxWidth())...)
}

func (m Model) currentYearMonthlySelectedSummary(report stats.YearMonthlyReport) stats.MonthlySummary {
	selectedMonth := m.currentMonthlySelection()
	for _, month := range report.Months {
		if statsMonthStart(month.MonthStart).Equal(selectedMonth) {
			return month
		}
	}
	if len(report.Months) > 0 {
		return report.Months[len(report.Months)-1]
	}
	return stats.MonthlySummary{MonthStart: selectedMonth}
}

func (m Model) yearMonthlyColumns() []statsTableColumn {
	return []statsTableColumn{
		{Header: "", MinWidth: 12, Style: defaultTextStyle},
		{Header: "msgs", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "sess", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle},
		{Header: "tokens", MinWidth: 12, AlignRight: true, Style: statsValueTextStyle},
		{Header: "cost", MinWidth: 10, AlignRight: true, Style: statsValueTextStyle},
	}
}

func (m Model) yearMonthlyTableRows(report stats.YearMonthlyReport) []statsTableRow {
	rows := make([]statsTableRow, 0, len(report.Months))
	for _, month := range m.yearMonthlyRows() {
		selected := statsMonthStart(month.MonthStart).Equal(m.currentMonthlySelection())
		label := month.MonthStart.Format("2006-01")
		if selected {
			label = cursorStyle.Render("> " + label)
		} else {
			label = "  " + label
		}
		rows = append(rows, statsTableRow{Cells: []string{
			label,
			formatCompactCount(month.TotalMessages),
			formatGroupedInt(month.TotalSessions),
			formatSummaryTokens(month.TotalTokens),
			formatSummaryCurrency(month.TotalCost),
		}})
	}
	return rows
}

func (m Model) renderYearMonthlyDetailLines(report stats.YearMonthlyReport, detail stats.WindowReport) []string {
	if m.isNarrowLayout() {
		return m.renderCompactYearMonthlyDetailLines(report, detail)
	}
	selected := m.currentYearMonthlySelectedSummary(report)
	monthDaily := m.currentMonthDaily()
	lines := []string{}
	if !monthDaily.MonthStart.IsZero() && statsMonthStart(monthDaily.MonthStart).Equal(statsMonthStart(selected.MonthStart)) {
		lines = append(lines, renderDetailSectionHeader(m.renderMonthDailyTitle(monthDaily), todaySectionTitleStyle), "")
		lines = append(lines, m.renderMonthDailyHeatmapLines(monthDaily)...)
		lines = append(lines, "")
		lines = append(lines, m.renderMonthDailySummaryMetricsLines(monthDaily)...)
	} else {
		header := fmt.Sprintf("%s", selected.MonthStart.Format("2006-01"))
		meta := fmt.Sprintf("selected month • %d active • streak %dmo (best %dmo)", report.ActiveMonths, report.CurrentStreak, report.BestStreak)
		lines = append(lines, renderDetailSectionHeader(joinTitleAndMeta(header, meta, max(0, m.statsTableMaxWidth()-4)), todaySectionTitleStyle), "")
	}
	if detail.Label == "" {
		return append(lines, "", "Loading stats...")
	}
	for _, line := range m.renderMonthlyDetailSections(detail) {
		lines = append(lines, line)
	}
	return lines
}

func (m Model) renderCompactYearMonthlyDetailLines(report stats.YearMonthlyReport, detail stats.WindowReport) []string {
	selected := m.currentYearMonthlySelectedSummary(report)
	bullet := func(text string) string {
		return defaultTextStyle.Render("    ") + truncateDisplayWidth(text, max(0, m.layoutWidth()-4))
	}
	lines := []string{renderDetailSectionHeader(selected.MonthStart.Format("2006-01"), todaySectionTitleStyle)}
	if detail.Label == "" {
		return append(lines, bullet("Loading stats..."))
	}
	lines = append(lines,
		"",
		renderSubSectionHeader(fmt.Sprintf("Providers (%d)", len(aggregateProviderModelUsages(detail.Models))), habitSectionTitleStyle),
	)
	for _, row := range aggregateProviderModelUsages(detail.Models) {
		lines = append(lines, bullet(fmt.Sprintf("• %s %s %s", windowModelDisplayName(row.Model), formatSummaryTokens(row.TotalTokens), formatSummaryCurrency(row.Cost))))
	}
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Models (%d)", len(detail.Models)), habitSectionTitleStyle))
	for _, row := range detail.Models {
		lines = append(lines, bullet(fmt.Sprintf("• %s %s %s", windowModelDisplayName(row.Model), formatSummaryTokens(row.TotalTokens), formatSummaryCurrency(row.Cost))))
	}
	for _, line := range m.renderDailyDetailActivityLines(detail) {
		lines = append(lines, line)
	}
	return lines
}

func (m Model) renderMonthlyDetailSections(report stats.WindowReport) []string {
	lines := []string{}
	providers := aggregateProviderModelUsages(report.Models)
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Providers (%d)", len(providers)), habitSectionTitleStyle))
	lines = append(lines, renderStatsTable(windowModelColumns(), windowModelTableRows(providers), m.statsTableMaxWidth())...)
	if m.projectScope && len(report.TopProjects) > 0 {
		lines = append(lines, "", activitySectionHeader("Projects", len(report.TopProjects)))
		lines = append(lines, m.renderProjectUsageLines(report.TopProjects, report.Tokens, report.TotalProjectCost)...)
	}
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Models (%d)", len(report.Models)), habitSectionTitleStyle))
	lines = append(lines, renderStatsTable(windowModelColumns(), windowModelTableRows(report.Models), m.statsTableMaxWidth())...)
	for _, line := range m.renderDailyDetailActivityLines(report) {
		lines = append(lines, line)
	}
	return lines
}

func aggregateProviderModelUsages(models []stats.ModelUsage) []stats.ModelUsage {
	agg := map[string]*stats.ModelUsage{}
	for _, model := range models {
		provider, _ := splitProviderModelUsageKey(model.Model)
		key := provider
		if strings.TrimSpace(key) == "" {
			key = "--"
		}
		item, ok := agg[key]
		if !ok {
			item = &stats.ModelUsage{Model: key}
			agg[key] = item
		}
		item.InputTokens += model.InputTokens
		item.OutputTokens += model.OutputTokens
		item.CacheReadTokens += model.CacheReadTokens
		item.CacheWriteTokens += model.CacheWriteTokens
		item.ReasoningTokens += model.ReasoningTokens
		item.TotalTokens += model.TotalTokens
		item.Cost += model.Cost
	}
	rows := make([]stats.ModelUsage, 0, len(agg))
	for _, item := range agg {
		rows = append(rows, *item)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Cost == rows[j].Cost {
			return rows[i].TotalTokens > rows[j].TotalTokens
		}
		return rows[i].Cost > rows[j].Cost
	})
	return rows
}

func (m Model) renderMonthDailyTitle(report stats.MonthDailyReport) string {
	monthDays := calendarMonthDayCount(report.MonthStart, report.MonthEnd)
	return truncateDisplayWidth(joinTitleAndMeta(report.MonthStart.Format("2006-01"), fmt.Sprintf("active %d/%dd • %s", report.ActiveDays, monthDays, formatMonthDailyBestStreak(report)), m.statsTableMaxWidth()), m.statsTableMaxWidth())
}

func calendarMonthDayCount(start, end time.Time) int {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	endExclusive := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	startDate := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	count := 0
	for day := startDate; day.Before(endExclusive); day = day.AddDate(0, 0, 1) {
		count++
	}
	return count
}

func yearMonthlyPeakInt(months []stats.MonthlySummary, value func(stats.MonthlySummary) int) (int, time.Time) {
	var peak int
	var peakMonth time.Time
	for _, month := range months {
		if current := value(month); current > peak {
			peak = current
			peakMonth = month.MonthStart
		}
	}
	return peak, peakMonth
}

func yearMonthlyPeakInt64(months []stats.MonthlySummary, value func(stats.MonthlySummary) int64) (int64, time.Time) {
	var peak int64
	var peakMonth time.Time
	for _, month := range months {
		if current := value(month); current > peak {
			peak = current
			peakMonth = month.MonthStart
		}
	}
	return peak, peakMonth
}

func yearMonthlyPeakFloat(months []stats.MonthlySummary, value func(stats.MonthlySummary) float64) (float64, time.Time) {
	var peak float64
	var peakMonth time.Time
	for _, month := range months {
		if current := value(month); current > peak {
			peak = current
			peakMonth = month.MonthStart
		}
	}
	return peak, peakMonth
}

func formatPeakMonthValue(value string, month time.Time) string {
	if month.IsZero() {
		return value
	}
	return fmt.Sprintf("%s (%s)", value, month.Format("2006-01"))
}

func formatCompactCount(value int) string {
	if value < 10000 {
		return formatGroupedInt(value)
	}
	return formatSummaryCodeLines(value)
}

// renderCompactMonthDailyLines renders month-list for narrow terminals
func (m Model) renderCompactMonthDailyLines(report stats.MonthDailyReport) []string {
	if report.MonthStart.IsZero() {
		return []string{"Loading month-daily stats..."}
	}

	clamp := func(text string) string {
		return truncateDisplayWidth(text, m.layoutWidth())
	}
	bullet := func(text string) string {
		return defaultTextStyle.Render("    ") + truncateDisplayWidth(text, max(0, m.layoutWidth()-4))
	}

	monthDays := calendarMonthDayCount(report.MonthStart, report.MonthEnd)
	title := fmt.Sprintf("%s  %s", report.MonthStart.Format("2006-01"), m.statsScopeLabel())
	lines := []string{
		clamp(renderSubSectionHeader(title, todaySectionTitleStyle)),
		bullet(fmt.Sprintf("active %d/%dd | messages %d | sessions %d", report.ActiveDays, monthDays, report.TotalMessages, report.TotalSessions)),
		bullet(fmt.Sprintf("tokens %s | cost %s", formatSummaryTokens(report.TotalTokens), formatSummaryCurrency(report.TotalCost))),
		"",
	}

	for _, day := range report.Days {
		selected := startOfStatsDay(day.Date).Equal(m.currentDailyDate())
		prefix := "  "
		if selected {
			prefix = "> "
		}
		line := fmt.Sprintf("%s%s %s %s %s",
			prefix,
			renderMonthDailyDayLabel(day.Date, selected),
			formatSummaryTokens(day.Tokens),
			formatSummaryCurrency(day.Cost),
			day.FocusTag,
		)
		lines = append(lines, bullet(line))
	}

	return lines
}

// monthDayColumnWidths returns column widths for month-daily table based on layout
func (m Model) monthDailyColumnWidths() monthDailyLayout {
	availWidth := m.statsTableMaxWidth()
	layout := monthDailyLayout{
		dateWidth:     10,
		messagesWidth: 8,
		sessionsWidth: 8,
		tokensWidth:   12,
		costWidth:     10,
		tagWidth:      6,
	}
	if availWidth < 48 {
		layout.dateWidth = 10
		layout.messagesWidth = 0
		layout.sessionsWidth = 0
		layout.tokensWidth = 6
		layout.costWidth = 6
		layout.tagWidth = 0
	} else if availWidth < 60 {
		layout.dateWidth = 10
		layout.messagesWidth = 0
		layout.sessionsWidth = 0
		layout.tokensWidth = 8
		layout.costWidth = 8
		layout.tagWidth = 4
	} else if availWidth < 72 {
		layout.dateWidth = 12
		layout.messagesWidth = 6
		layout.sessionsWidth = 0 // omitted
		layout.tokensWidth = 10
		layout.costWidth = 10
		layout.tagWidth = 6
	}

	return layout
}

type monthDailyLayout struct {
	dateWidth     int
	messagesWidth int
	sessionsWidth int
	tokensWidth   int
	costWidth     int
	tagWidth      int
}

func (m Model) monthDailyColumns() []statsTableColumn {
	layout := m.monthDailyColumnWidths()
	columns := []statsTableColumn{{Header: "", MinWidth: layout.dateWidth, Style: defaultTextStyle}}
	if layout.messagesWidth > 0 {
		columns = append(columns, statsTableColumn{Header: "msgs", MinWidth: layout.messagesWidth, AlignRight: true, Style: statsValueTextStyle})
	}
	if layout.sessionsWidth > 0 {
		columns = append(columns, statsTableColumn{Header: "sess", MinWidth: layout.sessionsWidth, AlignRight: true, Style: statsValueTextStyle})
	}
	tokenHeader := "tok"
	if layout.tokensWidth > 6 {
		tokenHeader = "tokens"
	}
	columns = append(columns, statsTableColumn{Header: tokenHeader, MinWidth: layout.tokensWidth, AlignRight: true, Style: statsValueTextStyle})
	costHeader := "$"
	if layout.costWidth > 6 {
		costHeader = "cost"
	}
	columns = append(columns, statsTableColumn{Header: costHeader, MinWidth: layout.costWidth, AlignRight: true, Style: statsValueTextStyle})
	if layout.tagWidth > 0 {
		tagHeader := "tag"
		if layout.tagWidth > 4 {
			tagHeader = "focus"
		}
		columns = append(columns, statsTableColumn{Header: tagHeader, MinWidth: layout.tagWidth, Style: defaultTextStyle})
	}
	return columns
}

func (m Model) monthDailyTableRows(report stats.MonthDailyReport) []statsTableRow {
	rows := make([]statsTableRow, 0, len(report.Days))
	layout := m.monthDailyColumnWidths()
	for _, day := range report.Days {
		selected := startOfStatsDay(day.Date).Equal(m.currentDailyDate())
		dateLabel := renderMonthDailyDayLabel(day.Date, selected)
		if selected {
			dateLabel = cursorStyle.Render("> " + dateLabel)
		} else {
			dateLabel = "  " + dateLabel
		}
		cells := []string{dateLabel}
		if layout.messagesWidth > 0 {
			cells = append(cells, formatDailyCount(day.Messages))
		}
		if layout.sessionsWidth > 0 {
			cells = append(cells, formatDailyCount(day.Sessions))
		}
		cells = append(cells, formatCompactTokens(day.Tokens), formatSummaryCurrency(day.Cost))
		if layout.tagWidth > 0 {
			cells = append(cells, m.focusTagStyle(day.FocusTag).Render(day.FocusTag))
		}
		rows = append(rows, statsTableRow{Cells: cells})
	}
	return rows
}

// formatMonthDailyHeaderRow formats the table header
func (m Model) formatMonthDailyHeaderRow(layout monthDailyLayout) string {
	parts := []string{}
	parts = append(parts, renderPadded("day", layout.dateWidth))
	if layout.messagesWidth > 0 {
		parts = append(parts, renderPadded("msgs", layout.messagesWidth))
	}
	if layout.sessionsWidth > 0 {
		parts = append(parts, renderPadded("sess", layout.sessionsWidth))
	}
	parts = append(parts, renderPadded(map[bool]string{true: "tokens", false: "tok"}[layout.tokensWidth > 6], layout.tokensWidth))
	parts = append(parts, renderPadded(map[bool]string{true: "cost", false: "$"}[layout.costWidth > 6], layout.costWidth))
	if layout.tagWidth > 0 {
		parts = append(parts, renderPadded(map[bool]string{true: "focus", false: "tag"}[layout.tagWidth > 4], layout.tagWidth))
	}

	return "| " + strings.Join(parts, " | ") + " |"
}

// formatMonthDailyRow formats a single daily summary row
func (m Model) formatMonthDailyRow(day stats.DailySummary, layout monthDailyLayout) string {
	tagStyle := m.focusTagStyle(day.FocusTag)
	selected := startOfStatsDay(day.Date).Equal(m.currentDailyDate())

	parts := []string{}
	dateLabel := day.Date.Format("01-02 Mon")
	if selected {
		dateLabel = "> " + dateLabel
	} else {
		dateLabel = "  " + dateLabel
	}
	parts = append(parts, renderPadded(dateLabel, layout.dateWidth))
	if layout.messagesWidth > 0 {
		parts = append(parts, renderPaddedRight(formatDailyCount(day.Messages), layout.messagesWidth))
	}
	if layout.sessionsWidth > 0 {
		parts = append(parts, renderPaddedRight(formatDailyCount(day.Sessions), layout.sessionsWidth))
	}
	parts = append(parts, renderPaddedRight(formatCompactTokens(day.Tokens), layout.tokensWidth))
	parts = append(parts, renderPaddedRight(formatSummaryCurrency(day.Cost), layout.costWidth))
	if layout.tagWidth > 0 {
		parts = append(parts, tagStyle.Width(layout.tagWidth).Align(lipgloss.Center).Render(day.FocusTag))
	}

	line := "| " + strings.Join(parts, " | ") + " |"
	if selected {
		return cursorStyle.Render(line)
	}
	return defaultTextStyle.Render(line)
}

func formatDailyCount(value int) string {
	if value <= 0 {
		return "--"
	}
	return formatGroupedInt(value)
}

func renderDailyMonthListHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "day") + helpBgTextStyle.Render(" • ") + helpEntry("enter", "detail") + helpBgTextStyle.Render(" • ") + helpEntry("[ ]", "month") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page"),
		helpBgTextStyle.Render("   ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("tab", "launcher") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back"),
	}, targetWidth)
}

func renderDailyDetailHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "scroll") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("esc", "month list") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("←/→", "tabs") + helpBgTextStyle.Render(" • ") + helpEntry("tab", "launcher"),
	}, targetWidth)
}

func renderMonthlyListHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "month") + helpBgTextStyle.Render(" • ") + helpEntry("enter", "detail") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back") + helpBgTextStyle.Render(" • ") + helpEntry("tab", "launcher"),
	}, targetWidth)
}

func renderMonthlyDetailHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "scroll") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("esc", "month list") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("←/→", "tabs") + helpBgTextStyle.Render(" • ") + helpEntry("tab", "launcher"),
	}, targetWidth)
}

func (m Model) renderDailyDetailLines(report stats.WindowReport) []string {
	if m.isNarrowLayout() {
		return m.renderCompactDailyDetailLines(report)
	}
	date := m.currentDailyDate()
	title := renderDetailSectionHeader(m.renderDailyDetailTitle(date, report), todaySectionTitleStyle)
	lines := []string{title, ""}
	lines = append(lines, m.renderDailyDetailHourlyLines(report)...)
	lines = append(lines, m.renderDailyDetailSections(report)...)
	return lines
}

func (m Model) renderCompactDailyDetailLines(report stats.WindowReport) []string {
	date := m.currentDailyDate()
	clamp := func(text string) string { return truncateDisplayWidth(text, m.layoutWidth()) }
	bullet := func(text string) string {
		return defaultTextStyle.Render("    ") + truncateDisplayWidth(text, max(0, m.layoutWidth()-4))
	}
	lines := []string{
		clamp(renderDetailSectionHeader(m.renderDailyDetailTitle(date, report), todaySectionTitleStyle)),
		bullet(m.renderDailyDetailMeta(report)),
		bullet(m.renderDailyDetailAxisLine()),
		bullet(m.renderDailyDetailSparkline(report)),
	}
	for _, line := range m.renderCompactDailyDetailSections(report) {
		lines = append(lines, clamp(line))
	}
	return lines
}

func (m Model) renderDailyDetailSections(report stats.WindowReport) []string {
	lines := []string{}
	if !m.projectScope && len(report.TopProjects) > 0 {
		lines = append(lines, "", activitySectionHeader("Projects", len(report.TopProjects)))
		lines = append(lines, m.renderProjectUsageLines(report.TopProjects, report.Tokens, report.TotalProjectCost)...)
	}
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Models (%d)", len(report.Models)), habitSectionTitleStyle))
	lines = append(lines, renderStatsTable(windowModelColumns(), windowModelTableRows(report.Models), m.statsTableMaxWidth())...)
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Sessions (%d)", len(report.AllSessions)), habitSectionTitleStyle))
	lines = append(lines, renderStatsTable(windowSessionColumns(), windowSessionTableRows(report.AllSessions, m.session.ID), m.statsTableMaxWidth())...)
	for _, line := range m.renderDailyDetailActivityLines(report) {
		lines = append(lines, line)
	}
	return lines
}

func (m Model) renderCompactDailyDetailSections(report stats.WindowReport) []string {
	lines := []string{}
	if !m.projectScope && len(report.TopProjects) > 0 {
		lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Projects (%d)", len(report.TopProjects)), habitSectionTitleStyle))
		for _, row := range report.TopProjects {
			lines = append(lines, defaultTextStyle.Render("    ")+fmt.Sprintf("• %s %s %s", blankDash(row.Name), formatSummaryTokens(row.Amount), formatSummaryCurrency(row.Cost)))
		}
	}
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Models (%d)", len(report.Models)), habitSectionTitleStyle))
	for _, model := range report.Models {
		lines = append(lines, defaultTextStyle.Render("    ")+fmt.Sprintf("• %s %s %s", blankDash(model.Model), formatSummaryTokens(model.TotalTokens), formatSummaryCurrency(model.Cost)))
	}
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Sessions (%d)", len(report.AllSessions)), habitSectionTitleStyle))
	for _, row := range windowSessionTableRows(report.AllSessions, m.session.ID) {
		lines = append(lines, defaultTextStyle.Render("    ")+"• "+strings.TrimSpace(strings.Join(row.Cells, " ")))
	}
	for _, line := range m.renderDailyDetailActivityLines(report) {
		lines = append(lines, line)
	}
	return lines
}

func (m Model) renderDailyDetailActivityLines(report stats.WindowReport) []string {
	lines := []string{}
	if len(report.TopAgentModels) > 0 {
		lines = append(lines, "", activitySectionHeader("Agents", len(report.TopAgentModels)))
		lines = append(lines, m.renderAgentModelUsageLines(report.TopAgentModels, int64(report.TotalAgentModelCalls))...)
	}
	if len(report.TopSkills) > 0 {
		lines = append(lines, "", activitySectionHeader("Skills", len(report.TopSkills)))
		lines = append(lines, m.renderUsageLines("count", report.TopSkills, int64(report.TotalSkillCalls))...)
	}
	if len(report.TopTools) > 0 {
		lines = append(lines, "", activitySectionHeader("Tools", len(report.TopTools)))
		lines = append(lines, m.renderUsageLines("count", report.TopTools, int64(report.TotalToolCalls))...)
	}
	return lines
}

func (m Model) renderDailyDetailHourlyLines(report stats.WindowReport) []string {
	return []string{m.renderDailyDetailAxisLine(), m.renderDailyDetailSparkline(report)}
}

func (m Model) renderDailyDetailTitle(date time.Time, report stats.WindowReport) string {
	left := date.Format("2006-01-02")
	right := m.renderDailyDetailMeta(report)
	padding := max(1, m.statsTableMaxWidth()-lipgloss.Width(left)-lipgloss.Width(right))
	return truncateDisplayWidth(left+strings.Repeat(" ", padding)+right, m.statsTableMaxWidth())
}

func (m Model) renderDailyDetailMeta(report stats.WindowReport) string {
	return fmt.Sprintf("%s active • streak %s (best %dd)", formatActiveHours(report.ActiveMinutes), formatHourlyStreakDuration(currentHalfHourStreakSlots(report.HalfHourSlots)), monthDailyBestStreak(m.currentMonthDaily().Days))
}

func formatActiveHours(minutes int) string {
	if minutes <= 0 {
		return "0h"
	}
	return fmt.Sprintf("%.1fh", float64(minutes)/60)
}

func currentHalfHourStreakSlots(slots [48]int64) int {
	streak := 0
	activeFound := false
	for i := len(slots) - 1; i >= 0; i-- {
		if slots[i] > 0 {
			activeFound = true
			streak++
			continue
		}
		if activeFound {
			return streak
		}
	}
	if !activeFound {
		return 0
	}
	return streak
}

func (m Model) renderDailyDetailAxisLine() string {
	parts := make([]string, 0, 24)
	for hour := 0; hour < 24; hour++ {
		if hour%2 == 0 {
			parts = append(parts, fmt.Sprintf("%02d", hour))
		} else {
			parts = append(parts, "  ")
		}
	}
	return defaultTextStyle.Render("    " + strings.Join(parts, " "))
}

func (m Model) renderDailyDetailSparkline(report stats.WindowReport) string {
	hourly := compressHalfHourSlots(report.HalfHourSlots)
	maxSlot := int64(0)
	for _, slot := range hourly {
		if slot > maxSlot {
			maxSlot = slot
		}
	}
	step := maxSlot / 7
	if step <= 0 {
		step = 1
	}
	var b strings.Builder
	for i, slot := range hourly {
		if i > 0 {
			b.WriteByte(' ')
		}
		level := sparklineLevel(slot, step)
		char := strings.Repeat(string(sparklineChars[level]), 2)
		color := sparklineTodayColors[level]
		if level == 0 {
			color = sparklineYesterdayColors[level]
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(char))
	}
	return defaultTextStyle.Render("    ") + b.String()
}

func compressHalfHourSlots(slots [48]int64) [24]int64 {
	var hourly [24]int64
	for i := 0; i < 24; i++ {
		hourly[i] = slots[i*2] + slots[i*2+1]
	}
	return hourly
}

func windowSessionTableRows(sessions []stats.SessionUsage, currentSessionID string) []statsTableRow {
	rows := make([]statsTableRow, 0, len(sessions))
	for _, session := range sessions {
		currentMark := ""
		if currentSessionID != "" && session.ID == currentSessionID {
			currentMark = "*"
		}
		rows = append(rows, statsTableRow{Cells: []string{currentMark, session.ID, formatGroupedInt(session.Messages), formatSummaryTokens(session.Tokens), formatSummaryCurrency(session.Cost), blankDash(session.Title)}})
	}
	if len(rows) == 0 {
		rows = append(rows, statsTableRow{Cells: []string{"", "-", "-", "-", "-", "-"}})
	}
	return rows
}

// focusTagStyle returns the appropriate style for a focus tag
func (m Model) focusTagStyle(tag string) lipgloss.Style {
	switch tag {
	case "spike":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900")).Bold(true)
	case "heavy":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(false)
	case "quiet":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Bold(false)
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#555555")).Bold(false)
	}
}

// Helper formatting functions
func renderPadded(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return text + strings.Repeat(" ", max(0, width-len(text)))
}

func renderPaddedRight(text string, width int) string {
	if width <= 0 {
		return ""
	}
	padding := max(0, width-lipgloss.Width(text))
	return strings.Repeat(" ", padding) + text
}
