package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

const (
	rhythmFirstColumnWidth = 44
	trendLabelColumnWidth  = 10
)

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

func renderColumn(label string, value string, width int) string {
	rendered := defaultTextStyle.Render(label) + valueText(value)
	return padStyledText(rendered, lipgloss.Width(label)+lipgloss.Width(value), width)
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
