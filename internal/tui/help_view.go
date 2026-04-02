package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func renderHelpBlock(lines []string, targetWidth int) string {
	bar := helpBarStyle.Render("┃")
	barWidth := lipgloss.Width(bar)
	contentWidth := max(0, targetWidth-barWidth)
	rendered := make([]string, len(lines))
	for i, line := range lines {
		rendered[i] = bar + helpBlockStyle.Width(contentWidth).Render(" "+line)
	}
	return strings.Join(rendered, "\n")
}

func helpEntry(key string, action string) string {
	return helpBgKeyStyle.Render(key) + helpBgTextStyle.Render(": "+action)
}

func renderHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "navigate") + helpBgTextStyle.Render(" • ") + helpEntry("space", "toggle") + helpBgTextStyle.Render(" • ") + helpEntry("enter", "confirm") + helpBgTextStyle.Render(" • ") + helpEntry("q", "quit"),
		helpBgTextStyle.Render("   ") + helpEntry("tab", "stats") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("s", "sessions") + helpBgTextStyle.Render(" • ") + helpEntry("c", "config"),
	}, targetWidth)
}

func renderStatsHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "scroll") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("tab", "launcher") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("←/→", "tabs") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back"),
	}, targetWidth)
}

func renderEditHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "navigate") + helpBgTextStyle.Render(" • ") + helpEntry("enter", "edit") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back"),
	}, targetWidth)
}

func renderSessionHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "navigate") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("enter", "select") + helpBgTextStyle.Render(" • ") + helpEntry("esc", "back"),
	}, targetWidth)
}
