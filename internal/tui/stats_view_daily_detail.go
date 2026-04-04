package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

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
	for _, line := range m.renderSharedDetailActivityLines(report) {
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
	for _, line := range m.renderSharedDetailActivityLines(report) {
		lines = append(lines, line)
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
	return fmt.Sprintf("%s active • streak %s (best %dd)", formatActiveHours(report.ActiveMinutes), formatHourlyStreakDuration(currentTrailingActiveSlots(report.HalfHourSlots)), monthDailyBestStreak(m.currentMonthDaily().Days))
}

func currentTrailingActiveSlots(slots [48]int64) int {
	streak := 0
	for i := len(slots) - 1; i >= 0; i-- {
		if slots[i] <= 0 {
			if streak > 0 {
				return streak
			}
			continue
		}
		streak++
	}
	return streak
}

func formatActiveHours(minutes int) string {
	if minutes <= 0 {
		return "0h"
	}
	return fmt.Sprintf("%.1fh", float64(minutes)/60)
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
