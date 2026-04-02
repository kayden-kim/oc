package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

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
	for _, line := range m.renderSharedMonthlyDetailSections(detail) {
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
	for _, line := range m.renderSharedDetailActivityLines(detail) {
		lines = append(lines, line)
	}
	return lines
}

func (m Model) renderSharedMonthlyDetailSections(report stats.WindowReport) []string {
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
	for _, line := range m.renderSharedDetailActivityLines(report) {
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

func renderMonthlyListHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "month") + helpBgTextStyle.Render(" • ") + helpEntry("enter", "detail") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back") + helpBgTextStyle.Render(" • ") + helpEntry("tab", "launcher"),
	}, targetWidth)
}
