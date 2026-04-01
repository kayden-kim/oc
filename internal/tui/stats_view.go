package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

const (
	rhythmFirstColumnWidth = 44
	trendLabelColumnWidth  = 10
	maxActivityItems       = 15
	statsTabWidth          = 14
	statsTableMaxWidth     = 76
	statsTableColumnGap    = 2
	usageBarWidth          = 8
)

type statsTableColumn struct {
	Header     string
	MinWidth   int
	Expand     bool
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

func (m Model) renderLauncherTodaySummary() string {
	report := m.currentLauncherReport()
	sections := []string{renderSubSectionHeader("Today", todaySectionTitleStyle), ""}
	sections = append(sections, bulletLine(styledMetricLine("• active ", formatLauncherActiveValue(report))))
	if m.launcherSparklineHasActivity(report) {
		sections = append(sections, "", m.renderLauncherTodayAxisLine(), m.renderLauncherTodaySparkline(report))
	}
	sections = append(sections, "", renderSubSectionHeader("Metrics", todaySectionTitleStyle))
	sections = append(sections, m.renderLauncherMetricsTables(report)...)
	return strings.Join(sections, "\n")
}

func (m Model) currentLauncherReport() stats.WindowReport {
	if m.projectScope {
		return m.projectDaily
	}
	return m.globalDaily
}

func formatLauncherActiveValue(report stats.WindowReport) string {
	if !windowHasActivity(report) {
		return "--"
	}
	minutes := report.ActiveMinutes
	if minutes <= 0 {
		minutes = 1
	}
	return formatPulseValue(
		formatRolling24hHours(minutes),
		formatInlineStreakSummary(
			formatHourlyStreakDuration(currentWindowStreakSlots(report.HalfHourSlots)),
			formatHourlyStreakDuration(bestWindowStreakSlots(report.HalfHourSlots)),
		),
	)
}

func currentWindowStreakSlots(slots [48]int64) int {
	streak := 0
	index := len(slots) - 1
	for index >= 0 && slots[index] <= 0 {
		index--
	}
	for i := index; i >= 0; i-- {
		if slots[i] <= 0 {
			break
		}
		streak++
	}
	return streak
}

func bestWindowStreakSlots(slots [48]int64) int {
	best := 0
	current := 0
	for _, slot := range slots {
		if slot > 0 {
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

func windowHasActivity(report stats.WindowReport) bool {
	if report.ActiveMinutes > 0 || report.Messages > 0 || report.Sessions > 0 || report.Tokens > 0 || report.Cost > 0 {
		return true
	}
	for _, slot := range report.HalfHourSlots {
		if slot > 0 {
			return true
		}
	}
	return false
}

func (m Model) launcherSparklineHasActivity(report stats.WindowReport) bool {
	if m.isNarrowLayout() {
		return false
	}
	return windowHasActivity(report)
}

func (m Model) renderLauncherMetricsTables(report stats.WindowReport) []string {
	tokenColumns := []statsTableColumn{
		{Header: "", MinWidth: 6, Style: defaultTextStyle},
		{Header: "input", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "output", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "c.read", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "c.write", MinWidth: 7, AlignRight: true, Style: statsValueTextStyle},
		{Header: "reasoning", MinWidth: 9, AlignRight: true, Style: statsValueTextStyle},
		{Header: "total", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "cost", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
	}
	activityColumns := []statsTableColumn{
		{Header: "hours", MinWidth: 5, AlignRight: true, Style: statsValueTextStyle},
		{Header: "sess", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle},
		{Header: "msgs", MinWidth: 4, AlignRight: true, Style: statsValueTextStyle},
		{Header: "lines", MinWidth: 5, AlignRight: true, Style: statsValueTextStyle},
		{Header: "files", MinWidth: 5, AlignRight: true, Style: statsValueTextStyle},
		{Header: "agents", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "skills", MinWidth: 6, AlignRight: true, Style: statsValueTextStyle},
		{Header: "tools", MinWidth: 5, AlignRight: true, Style: statsValueTextStyle},
	}
	tokenRows := []statsTableRow{{Cells: []string{
		"tokens",
		formatSummaryTokens(report.InputTokens),
		formatSummaryTokens(report.OutputTokens),
		formatSummaryTokens(report.CacheReadTokens),
		formatSummaryTokens(report.CacheWriteTokens),
		formatSummaryTokens(report.ReasoningTokens),
		formatSummaryTokens(report.Tokens),
		formatSummaryCurrency(report.Cost),
	}}}
	activityRows := []statsTableRow{{Cells: []string{
		formatSummaryHours(report.ActiveMinutes),
		formatGroupedInt(report.Sessions),
		formatGroupedInt(report.Messages),
		formatSummaryCodeLines(report.CodeLines),
		formatSummaryChangedFiles(report.ChangedFiles),
		formatGroupedInt(report.TotalAgentModelCalls),
		formatGroupedInt(report.TotalSkillCalls),
		formatGroupedInt(report.TotalToolCalls),
	}}}
	lines := renderStatsTable(tokenColumns, tokenRows, m.statsTableMaxWidth())
	lines = append(lines, "")
	lines = append(lines, renderStatsTable(activityColumns, activityRows, m.statsTableMaxWidth())...)
	return lines
}

func (m Model) renderLauncherTodayAxisLine() string {
	return m.renderDailyDetailAxisLine()
}

func (m Model) renderLauncherTodaySparkline(report stats.WindowReport) string {
	return m.renderDailyDetailSparkline(report)
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
	best := max(report.BestHourlyStreakSlots, report.CurrentHourlyStreakSlots)
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

func padStyledText(rendered string, visibleWidth int, targetWidth int) string {
	if visibleWidth >= targetWidth {
		return rendered
	}
	return rendered + defaultTextStyle.Render(strings.Repeat(" ", targetWidth-visibleWidth))
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

func activitySectionHeader(title string, unique int) string {
	title = strings.TrimPrefix(title, "Activity - ")
	return renderSubSectionHeader(fmt.Sprintf("%s (%s)", title, formatGroupedInt(unique)), habitSectionTitleStyle)
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
		widths[i] = max(minWidth, lipgloss.Width(column.Header))
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
			widths[i] = max(widths[i], lipgloss.Width(cell))
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
	growOrder := expandingColumnOrder(columns)
	if len(growOrder) == 0 {
		growOrder = make([]int, len(widths))
		for i := range widths {
			growOrder[i] = i
		}
	}
	for sumInts(widths) < available {
		for _, i := range growOrder {
			if sumInts(widths) >= available {
				break
			}
			widths[i]++
		}
	}
	return widths
}

func expandingColumnOrder(columns []statsTableColumn) []int {
	order := make([]int, 0, len(columns))
	for i, column := range columns {
		if column.Expand {
			order = append(order, i)
		}
	}
	return order
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
	padding := max(width-visible, 0)
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
		slotHigh = config.DefaultActivityHighTokens / 2
	}
	step := slotHigh / 7
	if step <= 0 {
		step = 1
	}

	var b strings.Builder
	compressedTodayStart := rolling24hCompressedTodayStartIndex(now)
	for i := range 24 {
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

func (m Model) renderStatsView() string {
	lines := m.statsContentLines()
	start, end := m.visibleStatsRange(len(lines))
	help := renderStatsHelpLine(m.layoutWidth())
	switch m.statsTab {
	case 1:
		if m.dailyDetailMode {
			help = renderDailyDetailHelpLine(m.layoutWidth())
		} else {
			help = renderDailyMonthListHelpLine(m.layoutWidth())
		}
	case 2:
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
		switch metricHeader {
		case "tokens":
			metricLabel = "tok"
		case "count":
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
	filled = min(max(filled, 1), width)
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
