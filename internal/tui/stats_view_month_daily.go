package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

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
	for range firstWeekday {
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
		{Cells: []string{"sessions", formatMonthDailyIntMetricWithShare(selected.Sessions, report.TotalSessions), formatPeakValue(formatGroupedInt(peakSessions), peakSessionsDate), formatGroupedInt(report.TotalSessions)}},
		{Cells: []string{"messages", formatMonthDailyCompactIntMetricWithShare(selected.Messages, report.TotalMessages), formatPeakValue(formatCompactCount(peakMessages), peakMessagesDate), formatCompactCount(report.TotalMessages)}},
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
		{Cells: []string{"sessions", formatPeakValue(formatGroupedInt(peakSessions), peakSessionsDate), formatGroupedInt(report.TotalSessions)}},
		{Cells: []string{"messages", formatPeakValue(formatCompactCount(peakMessages), peakMessagesDate), formatCompactCount(report.TotalMessages)}},
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

func (m Model) monthDailyColumnWidths() monthDailyLayout {
	availWidth := m.statsTableMaxWidth()
	layout := monthDailyLayout{dateWidth: 10, messagesWidth: 8, sessionsWidth: 8, tokensWidth: 12, costWidth: 10, tagWidth: 6}
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
		layout.sessionsWidth = 0
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
	if layout.sessionsWidth > 0 {
		columns = append(columns, statsTableColumn{Header: "sess", MinWidth: layout.sessionsWidth, AlignRight: true, Style: statsValueTextStyle})
	}
	if layout.messagesWidth > 0 {
		columns = append(columns, statsTableColumn{Header: "msgs", MinWidth: layout.messagesWidth, AlignRight: true, Style: statsValueTextStyle})
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
		if layout.sessionsWidth > 0 {
			cells = append(cells, formatDailyCount(day.Sessions))
		}
		if layout.messagesWidth > 0 {
			cells = append(cells, formatDailyCount(day.Messages))
		}
		cells = append(cells, formatCompactTokens(day.Tokens), formatSummaryCurrency(day.Cost))
		if layout.tagWidth > 0 {
			cells = append(cells, m.focusTagStyle(day.FocusTag).Render(day.FocusTag))
		}
		rows = append(rows, statsTableRow{Cells: cells})
	}
	return rows
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

func (m Model) renderMonthDailyTitle(report stats.MonthDailyReport) string {
	monthDays := calendarMonthDayCount(report.MonthStart, report.MonthEnd)
	return truncateDisplayWidth(joinTitleAndMeta(report.MonthStart.Format("2006-01"), fmt.Sprintf("active %d/%dd • %s", report.ActiveDays, monthDays, formatMonthDailyBestStreak(report)), m.statsTableMaxWidth()), m.statsTableMaxWidth())
}

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
