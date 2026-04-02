package tui

import "charm.land/lipgloss/v2"

var (
	defaultTextStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A"))
	statsValueTextStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	cursorStyle             = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	cursorSelectedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	helpBgKeyStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Background(lipgloss.Color("#191919")).Bold(true)
	helpBgTextStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A")).Background(lipgloss.Color("#191919"))
	helpBarStyle            = lipgloss.NewStyle().Foreground(lipgloss.Color("#191919"))
	helpBlockStyle          = lipgloss.NewStyle().Background(lipgloss.Color("#191919"))
	sessionContainerStyle   = lipgloss.NewStyle()
	sessionLabelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#FF9900")).Bold(true).Padding(0, 1)
	sessionContentStyle     = lipgloss.NewStyle().Background(lipgloss.Color("#292929")).Padding(0, 1)
	sessionValueStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#292929")).Bold(false)
	sessionMetaStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Background(lipgloss.Color("#393939")).Bold(false).Padding(0, 1)
	habitSectionTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#191919")).Bold(false).Padding(0, 1)
	todaySectionTitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#191919")).Bold(false).Bold(true).Padding(0, 1)
	instructionTitleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#292929")).Bold(false).Padding(0, 1)
	statsTabActiveStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Background(lipgloss.Color("#393939")).Bold(true).Padding(0, 1)
	statsTabStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Background(lipgloss.Color("#1F1F1F")).Padding(0, 1)
	statsTabIndicatorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#60C5F1"))
	statsTabMetaStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	dimmedLabelStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	sundayTextStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#C97373"))
	selectedSundayTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B6B")).Bold(true)
	sectionBarStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	instructionBarStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#60C5F1"))
	detailSectionBarStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#60C5F1"))
)

func (m Model) renderTopBadge() string {
	targetWidth := m.layoutWidth()
	label := sessionLabelStyle.Render("OC")
	version := sessionContentStyle.Render(sessionValueStyle.Render(m.version))
	if m.isNarrowLayout() {
		return sessionContainerStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, label, version))
	}
	metaWidth := max(0, targetWidth-lipgloss.Width(label)-lipgloss.Width(version))
	metaText := selectedSessionSummary(m.session, max(0, metaWidth-2))
	return sessionContainerStyle.Render(lipgloss.JoinHorizontal(
		lipgloss.Top,
		label,
		version,
		sessionMetaStyle.Width(metaWidth).Render(metaText),
	))
}

func renderSectionHeader(text string, targetWidth int) string {
	bar := instructionBarStyle.Render("┃")
	barWidth := lipgloss.Width(bar)
	return bar + instructionTitleStyle.Width(max(0, targetWidth-barWidth)).Render(text)
}

func renderSubSectionHeader(text string, style lipgloss.Style) string {
	return "  " + sectionBarStyle.Render("┃") + style.Render(text)
}

func renderDetailSectionHeader(text string, style lipgloss.Style) string {
	return "  " + detailSectionBarStyle.Render("┃") + style.Render(text)
}

func stylePluginRow(line string, focused bool, selected bool) string {
	switch {
	case focused && selected:
		return cursorSelectedStyle.Render(line)
	case focused:
		return cursorStyle.Render(line)
	default:
		return defaultTextStyle.Render(line)
	}
}
