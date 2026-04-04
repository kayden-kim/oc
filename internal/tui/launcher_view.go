package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

func (m Model) viewLauncher() tea.View {
	sections := []string{m.renderLauncherTodaySummary(), renderSectionHeader("📋 Choose plugins", m.layoutWidth())}
	var sb strings.Builder
	sb.WriteString(m.renderTopBadge())
	sb.WriteString("\n\n")
	sb.WriteString(strings.Join(filterNonEmpty(sections), "\n\n"))
	sb.WriteString("\n\n")

	for i, p := range m.plugins {
		sb.WriteString(m.renderLauncherRow(i, p))
		sb.WriteByte('\n')
	}
	if len(m.plugins) == 0 {
		hint := dimmedLabelStyle.Render("Press enter to launch opencode")
		sb.WriteString(lipgloss.PlaceHorizontal(m.layoutWidth(), lipgloss.Center, hint))
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	sb.WriteString(renderHelpLine(m.layoutWidth()))

	return tea.NewView(sb.String())
}

func (m Model) renderLauncherRow(index int, plugin PluginItem) string {
	cursor := "  "
	focused := m.cursor == index
	if focused {
		cursor = "> "
	}

	checked := "   "
	_, selected := m.selected[index]
	if selected {
		checked = "✔  "
	}

	plainLine := fmt.Sprintf("%s%s%s", cursor, checked, plugin.Name)
	line := truncateDisplayWidth(plainLine, m.layoutWidth())
	if plugin.SourceLabel != "" {
		plainWithLabel := plainLine + " [" + plugin.SourceLabel + "]"
		if lipgloss.Width(plainWithLabel) <= m.layoutWidth() {
			line = plainLine + " " + dimmedLabelStyle.Render("["+plugin.SourceLabel+"]")
		} else {
			line = truncateDisplayWidth(plainWithLabel, m.layoutWidth())
		}
	}
	return stylePluginRow(line, focused, selected)
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

func (m Model) launcherSparklineHasActivity(report stats.WindowReport) bool {
	if m.isNarrowLayout() {
		return false
	}
	return windowHasActivity(report)
}

func currentWindowStreakSlots(slots [48]int64) int {
	return currentTrailingActiveSlots(slots)
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
	now := time.Now()
	highlightCurrent := startOfStatsDay(report.Start).Equal(startOfStatsDay(now))
	return renderHalfHourSparkline(report.HalfHourSlots, now, highlightCurrent)
}
