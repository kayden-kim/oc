package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

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
		index = min(max(index, 0), len(levels)-1)
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

func formatRatioToTop(today float64, maxValue float64) string {
	if today <= 0 || maxValue <= 0 {
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

func renderDetailModeHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "scroll") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("esc", "month list") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("←/→", "tabs") + helpBgTextStyle.Render(" • ") + helpEntry("tab", "launcher"),
	}, targetWidth)
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

func (m Model) renderSharedDetailActivityLines(report stats.WindowReport) []string {
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
