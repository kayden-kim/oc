package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

const statsTabWidth = 14

func statsTabTitles() []string {
	return []string{"Overview", "Daily", "Monthly"}
}

func filterNonEmpty(parts []string) []string {
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			result = append(result, part)
		}
	}
	return result
}

func (m Model) currentReport() stats.Report {
	if m.projectScope {
		return m.projectStats
	}
	return m.globalStats
}

func (m Model) renderStatsView() string {
	lines := m.statsContentLines()
	start, end := m.visibleStatsRange(len(lines))
	help := renderStatsHelpLine(m.layoutWidth())
	switch m.statsTab {
	case 1:
		if m.dailyDetailMode {
			help = renderDetailModeHelpLine(m.layoutWidth())
		} else {
			help = renderDailyMonthListHelpLine(m.layoutWidth())
		}
	case 2:
		if m.monthlyDetailMode {
			help = renderDetailModeHelpLine(m.layoutWidth())
		} else {
			help = renderMonthlyListHelpLine(m.layoutWidth())
		}
	}
	parts := []string{
		m.renderTopBadge(),
		m.renderStatsTabs() + "\n" + strings.Join(lines[start:end], "\n"),
		help,
	}
	return strings.Join(filterNonEmpty(parts), "\n\n")
}

// statsContentLines keeps stats-mode dispatch centralized across split view files.
func (m Model) statsContentLines() []string {
	if m.statsTab == 0 && m.currentStatsLoading() && len(m.currentReport().Days) == 0 {
		return []string{"Loading stats..."}
	}
	switch m.statsTab {
	case 0:
		return m.renderOverviewLines()
	case 1:
		if m.dailyDetailMode {
			if m.currentWindowLoading() && m.currentWindowReport().Label == "" {
				return []string{"Loading stats..."}
			}
			return m.renderDailyDetailLines(m.currentWindowReport())
		}
		report := m.currentMonthDaily()
		if m.currentMonthDailyLoading() && report.MonthStart.IsZero() {
			return []string{renderSubSectionHeader(m.currentDailyMonth().Format("2006-01"), todaySectionTitleStyle)}
		}
		return m.renderMonthDailyLines(report)
	case 2:
		report := m.currentYearMonthly()
		if m.monthlyDetailMode {
			return m.renderYearMonthlyDetailLines(report, m.currentWindowReport())
		}
		if m.currentYearMonthlyLoading() && len(report.Months) == 0 {
			return []string{renderSubSectionHeader(m.currentMonthlySelection().Format("2006-01"), todaySectionTitleStyle)}
		}
		return m.renderYearMonthlyLines(report)
	default:
		if m.currentWindowLoading() && m.currentWindowReport().Label == "" {
			return []string{"Loading stats..."}
		}
		return m.renderWindowLines(m.currentWindowReport())
	}
}

func (m Model) currentStatsLoading() bool {
	if m.projectScope {
		return m.projectStatsLoading
	}
	return m.globalStatsLoading
}

func (m Model) currentWindowReport() stats.WindowReport {
	if m.projectScope {
		if m.statsTab == 1 {
			return m.projectDaily
		}
		return m.projectMonthly
	}
	if m.statsTab == 1 {
		return m.globalDaily
	}
	return m.globalMonthly
}

func (m Model) currentWindowLoading() bool {
	if m.projectScope {
		if m.statsTab == 1 {
			return m.projectDailyLoading
		}
		return m.projectMonthlyLoading
	}
	if m.statsTab == 1 {
		return m.globalDailyLoading
	}
	return m.globalMonthlyLoading
}

func (m Model) currentMonthDaily() stats.MonthDailyReport {
	if m.projectScope {
		return m.projectMonthDaily
	}
	return m.globalMonthDaily
}

func (m Model) currentMonthDailyLoading() bool {
	if m.projectScope {
		return m.projectMonthDailyLoading
	}
	return m.globalMonthDailyLoading
}

func (m Model) renderStatsTabs() string {
	titles := statsTabTitles()
	if len(titles) == 0 {
		return ""
	}
	targetWidth := m.layoutWidth()
	labels := make([]string, 0, len(titles))
	indicators := make([]string, 0, len(titles))
	for i, title := range titles {
		labels = append(labels, renderStatsTabLabel(title, i == m.statsTab))
		if i == m.statsTab {
			indicators = append(indicators, statsTabIndicatorStyle.Render(strings.Repeat("▔", statsTabWidth)))
			continue
		}
		indicators = append(indicators, strings.Repeat(" ", statsTabWidth))
	}
	left := strings.Join(labels, "")
	if m.isNarrowLayout() {
		meta := statsTabMetaStyle.Width(targetWidth).Align(lipgloss.Right).Render(m.statsTabMeta())
		return left + "\n" + meta
	}
	metaWidth := max(0, targetWidth-lipgloss.Width(left))
	meta := statsTabMetaStyle.Width(metaWidth).Align(lipgloss.Right).Render(m.statsTabMeta())
	indicatorRow := strings.Join(indicators, "") + strings.Repeat(" ", metaWidth)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, meta) + "\n" + indicatorRow
}

func renderStatsTabLabel(title string, active bool) string {
	style := statsTabStyle
	if active {
		style = statsTabActiveStyle
	}
	return style.Padding(0, 0).Width(statsTabWidth).Align(lipgloss.Center).Render("   " + title + "   ")
}

func (m Model) statsTabMeta() string {
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color("#202020")).Render("|")
	label := lipgloss.NewStyle().Foreground(lipgloss.Color("#8A8A8A")).Render(strings.ToUpper(m.statsScopeLabel()))
	return " " + divider + " " + label + " "
}

func (m Model) statsScopeLabel() string {
	if m.projectScope {
		return "project"
	}
	return "global"
}

func renderDetailModeHelpLine(targetWidth int) string {
	return renderHelpBlock([]string{
		helpBgTextStyle.Render("💡 ") + helpEntry("↑/↓", "scroll") + helpBgTextStyle.Render(" • ") + helpEntry("pgup/pgdn", "page") + helpBgTextStyle.Render(" • ") + helpEntry("ctrl+u/d", "half") + helpBgTextStyle.Render(" • ") + helpEntry("home/end", "top/bottom"),
		helpBgTextStyle.Render("   ") + helpEntry("esc", "month list") + helpBgTextStyle.Render(" • ") + helpEntry("g", "scope") + helpBgTextStyle.Render(" • ") + helpEntry("←/→", "tabs") + helpBgTextStyle.Render(" • ") + helpEntry("tab", "launcher"),
	}, targetWidth)
}
