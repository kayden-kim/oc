package tui

import (
	"fmt"
	"math"
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
	headerPrefix := ""
	if m.projectScope {
		headerPrefix = "[Project] "
	}
	sections := []string{renderSubSectionHeader(headerPrefix+"My Pulse", habitSectionTitleStyle)}
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
	sections = append(sections, "", renderSubSectionHeader(headerPrefix+"Metrics", todaySectionTitleStyle))
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
	parts := []string{
		m.renderTopBadge(),
		m.renderStatsTabs() + "\n" + strings.Join(lines[start:end], "\n"),
		renderStatsHelpLine(m.layoutWidth()),
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
		if m.currentWindowLoading() && m.currentWindowReport().Label == "" {
			return []string{"Loading stats..."}
		}
		return m.renderWindowLines(m.currentWindowReport())
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
		return left
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
	return fmt.Sprintf(" %s • %s ", m.statsScopeLabel(), m.statsTabDateRange())
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
			bulletLine(styledTrendLine("• tokens ", renderValueTrend(report.Days, func(day stats.Day) float64 { return float64(day.Tokens) }))),
			bulletLine(styledTrendLine("• cost ", renderValueTrend(report.Days, func(day stats.Day) float64 { return day.Cost }))),
			bulletLine(styledTrendLine("• hours ", renderValueTrend(report.Days, func(day stats.Day) float64 { return float64(day.SessionMinutes) }))),
			bulletLine(styledTrendLine("• lines ", renderValueTrend(report.Days, func(day stats.Day) float64 { return float64(day.CodeLines) }))),
			bulletLine(styledTrendLine("• files ", renderValueTrend(report.Days, func(day stats.Day) float64 { return float64(day.ChangedFiles) }))),
		)
	}
	lines = append(lines,
		"",
		bulletLine(styledMetricLine("• reasoning ", fmt.Sprintf("%s today | %s baseline", formatPercent(report.TodayReasoningShare), formatPercent(report.RecentReasoningShare)))),
		"",
		activitySectionHeader("Activity - Models", report.UniqueModelCount),
	)
	lines = append(lines, m.renderModelUsageLines(report.TopModels, report.TotalModelTokens)...)
	if !m.projectScope {
		lines = append(lines,
			"",
			activitySectionHeader("Activity - Projects", report.UniqueProjectCount),
		)
		lines = append(lines, m.renderUsageLines("tokens", report.TopProjects, report.ThirtyDayTokens)...)
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
		rows := []statsTableRow{{Cells: []string{"top 15", "--", placeholderShare("--")}}}
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
		rows := []statsTableRow{{Cells: []string{"top 15", "--", "--", placeholderShare("--")}}}
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

func (m Model) renderModelUsageLines(items []stats.UsageCount, total int64) []string {
	metricLabel := "tokens"
	shareLabel := "share"
	metricMinWidth := 7
	shareMinWidth := 5
	if m.isNarrowLayout() {
		metricLabel = "tok"
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
		{Header: "", MinWidth: 18, Style: defaultTextStyle},
		{Header: "provider", MinWidth: 10, Style: defaultTextStyle},
		{Header: metricLabel, MinWidth: metricMinWidth, AlignRight: true, Style: statsValueTextStyle},
		{Header: shareLabel, MinWidth: shareMinWidth, Style: statsValueTextStyle},
	}
	if len(items) == 0 {
		rows := []statsTableRow{{Cells: []string{"top 15", "--", "--", placeholderShare("--")}}}
		if total > 0 {
			rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatSummaryTokens(total), placeholderShare("100%")}})
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
		provider, model := splitProviderModelUsageKey(item.Name)
		itemMetric := usageMetric(item)
		othersMetric -= itemMetric
		rows = append(rows, statsTableRow{Cells: []string{model, provider, formatSummaryTokens(itemMetric), shareCell(itemMetric, top)}})
	}
	if showOthers && othersMetric > 0 {
		rows = append(rows, statsTableRow{Cells: []string{"others", "", formatSummaryTokens(othersMetric), shareCell(othersMetric, top)}})
	}
	if total > 0 {
		rows = append(rows, statsTableRow{Divider: true}, statsTableRow{Cells: []string{"Total", "", formatSummaryTokens(total), placeholderShare("100%")}})
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
	for _, model := range models {
		rows = append(rows, statsTableRow{Cells: []string{
			model.Model,
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
	return rows
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
	rows := make([][]string, 0, len(report.TopSessions)+1)
	foundCurrent := false
	for _, session := range report.TopSessions {
		currentMark := ""
		if m.session.ID != "" && session.ID == m.session.ID {
			currentMark = "*"
			foundCurrent = true
		}
		rows = append(rows, []string{currentMark, session.ID, formatGroupedInt(session.Messages), formatSummaryTokens(session.Tokens), formatSummaryCurrency(session.Cost), blankDash(session.Title)})
	}
	if m.session.ID != "" && !foundCurrent {
		rows = append([][]string{{"*", "(current session not in selected window)", "-", "-", "-", "-"}}, rows...)
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
