package tui

import (
	"fmt"
	"sort"
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
	columns := []statsTableColumn{{Header: "", MinWidth: 8, Style: defaultTextStyle}, {Header: selectedLabel, MinWidth: 14, Style: todayStyle}, {Header: "peak month", MinWidth: 18, Style: statsValueTextStyle}, {Header: "total", MinWidth: 12, Style: statsValueTextStyle}}
	peakMessages, peakMessagesMonth := yearMonthlyPeakInt(report.Months, func(month stats.MonthlySummary) int { return month.TotalMessages })
	peakSessions, peakSessionsMonth := yearMonthlyPeakInt(report.Months, func(month stats.MonthlySummary) int { return month.TotalSessions })
	peakTokens, peakTokensMonth := yearMonthlyPeakInt64(report.Months, func(month stats.MonthlySummary) int64 { return month.TotalTokens })
	peakCost, peakCostMonth := yearMonthlyPeakFloat(report.Months, func(month stats.MonthlySummary) float64 { return month.TotalCost })
	rows := []statsTableRow{
		{Cells: []string{"sessions", formatMonthDailyIntMetricWithShare(selected.TotalSessions, report.TotalSessions), formatPeakMonthValue(formatGroupedInt(peakSessions), peakSessionsMonth), formatGroupedInt(report.TotalSessions)}},
		{Cells: []string{"messages", formatMonthDailyCompactIntMetricWithShare(selected.TotalMessages, report.TotalMessages), formatPeakMonthValue(formatCompactCount(peakMessages), peakMessagesMonth), formatCompactCount(report.TotalMessages)}},
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
	return []statsTableColumn{{Header: "", MinWidth: 12, Style: defaultTextStyle}, {Header: "msgs", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle}, {Header: "sess", MinWidth: 8, AlignRight: true, Style: statsValueTextStyle}, {Header: "tokens", MinWidth: 12, AlignRight: true, Style: statsValueTextStyle}, {Header: "cost", MinWidth: 10, AlignRight: true, Style: statsValueTextStyle}}
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
		rows = append(rows, statsTableRow{Cells: []string{label, formatCompactCount(month.TotalMessages), formatGroupedInt(month.TotalSessions), formatSummaryTokens(month.TotalTokens), formatSummaryCurrency(month.TotalCost)}})
	}
	return rows
}

func (m Model) renderYearMonthlyDetailLines(report stats.YearMonthlyReport, detail stats.WindowReport) []string {
	if m.isNarrowLayout() {
		return m.renderCompactYearMonthlyDetailLines(report, detail)
	}
	selected := m.currentYearMonthlySelectedSummary(report)
	monthDaily := m.currentMonthDaily()
	if detail.Label == "" {
		return []string{renderDetailSectionHeader(selected.MonthStart.Format("2006-01"), todaySectionTitleStyle), "", "Loading stats..."}
	}
	lines := []string{}
	if !monthDaily.MonthStart.IsZero() && statsMonthStart(monthDaily.MonthStart).Equal(statsMonthStart(selected.MonthStart)) {
		lines = append(lines, renderDetailSectionHeader(m.renderMonthDailyTitle(monthDaily), todaySectionTitleStyle), "")
		lines = append(lines, m.renderMonthDailyHeatmapLines(monthDaily)...)
		lines = append(lines, "")
		lines = append(lines, m.renderMonthDailySummaryMetricsLines(monthDaily)...)
	} else {
		header := selected.MonthStart.Format("2006-01")
		meta := fmt.Sprintf("selected month • %d active • streak %dmo (best %dmo)", report.ActiveMonths, report.CurrentStreak, report.BestStreak)
		lines = append(lines, renderDetailSectionHeader(joinTitleAndMeta(header, meta, max(0, m.statsTableMaxWidth()-4)), todaySectionTitleStyle), "")
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
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Providers (%d)", len(aggregateProviderModelUsages(detail.Models))), habitSectionTitleStyle))
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
	lines = append(lines, renderStatsTable(windowModelColumns(), windowModelTableRows(providers, windowProviderDisplayName), m.statsTableMaxWidth())...)
	if m.projectScope && len(report.TopProjects) > 0 {
		lines = append(lines, "", activitySectionHeader("Projects", len(report.TopProjects)))
		lines = append(lines, m.renderProjectUsageLines(report.TopProjects, report.Tokens, report.TotalProjectCost)...)
	}
	lines = append(lines, "", renderSubSectionHeader(fmt.Sprintf("Models (%d)", len(report.Models)), habitSectionTitleStyle))
	lines = append(lines, renderStatsTable(windowModelColumns(), windowModelTableRows(report.Models, windowModelDisplayName), m.statsTableMaxWidth())...)
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
	lines = append(lines, renderStatsTable(windowModelColumns(), windowModelTableRows(report.Models, windowModelDisplayName), m.statsTableMaxWidth())...)
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
		lines = append(lines, defaultTextStyle.Render("    ")+fmt.Sprintf("• %s %s %s", blankDash(windowModelDisplayName(model.Model)), formatSummaryTokens(model.TotalTokens), formatSummaryCurrency(model.Cost)))
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
	for hour := range 24 {
		if hour%2 == 0 {
			parts = append(parts, fmt.Sprintf("%02d", hour))
		} else {
			parts = append(parts, "  ")
		}
	}
	return defaultTextStyle.Render("      " + strings.Join(parts, " "))
}

func (m Model) renderDailyDetailSparkline(report stats.WindowReport) string {
	now := time.Now()
	highlightCurrent := startOfStatsDay(m.currentDailyDate()).Equal(startOfStatsDay(now))
	return renderHalfHourSparkline(report.HalfHourSlots, now, highlightCurrent)
}

func renderHalfHourSparkline(slots [48]int64, now time.Time, highlightCurrent bool) string {
	maxSlot := int64(0)
	for _, slot := range slots {
		if slot > maxSlot {
			maxSlot = slot
		}
	}
	step := maxSlot / 7
	if step <= 0 {
		step = 1
	}
	var b strings.Builder
	currentSlot := -1
	if highlightCurrent {
		currentSlot = now.Hour()*2 + now.Minute()/30
	}
	for i, slot := range slots {
		if i > 0 {
			if i%2 == 0 {
				b.WriteByte(' ')
			}
		}
		level := sparklineLevel(slot, step)
		char := string(sparklineChars[level])
		color := sparklineTodayColors[level]
		if level == 0 {
			color = sparklineYesterdayColors[level]
		}
		if i == currentSlot {
			color = currentHalfHourHighlightColor
		}
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(char))
	}
	return defaultTextStyle.Render("      ") + b.String()
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
