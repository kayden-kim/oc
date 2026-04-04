package tui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

func (m Model) renderHeatmapLine(days []stats.Day) string {
	var b strings.Builder
	todayKey := heatmapDayKey(time.Now())
	for i, day := range days {
		if i > 0 && i%7 == 0 {
			b.WriteByte(' ')
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

func isActive(day stats.Day) bool {
	return day.AssistantMessages > 0 || day.ToolCalls > 0 || day.StepFinishes > 0
}
