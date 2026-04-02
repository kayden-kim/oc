package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/stats"
)

func loadStatsCmd(project bool, loader func() (stats.Report, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader()
		return statsLoadedMsg{project: project, report: report, err: err}
	}
}

func loadWindowCmdWithRange(project bool, label string, start, end time.Time, loader func() (stats.WindowReport, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader()
		return windowReportLoadedMsg{project: project, label: label, start: start, end: end, report: report, err: err}
	}
}

func loadMonthDailyCmd(project bool, monthStart time.Time, loader func(time.Time) (stats.MonthDailyReport, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader(monthStart)
		return monthDailyReportLoadedMsg{project: project, monthStart: monthStart, report: report, err: err}
	}
}

func loadYearMonthlyCmd(project bool, endMonth time.Time, loader func(time.Time) (stats.YearMonthlyReport, error)) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		report, err := loader(endMonth)
		return yearMonthlyReportLoadedMsg{project: project, endMonth: endMonth, report: report, err: err}
	}
}

func (m Model) loadCurrentScopeCmd() tea.Cmd {
	now := time.Now()
	if m.statsMode {
		if m.statsTab == 0 {
			return m.loadOverviewCmd(now)
		}
		if m.statsTab == 1 && !m.dailyDetailMode {
			return m.loadMonthDailyReportCmd(m.currentDailyMonth(), now)
		}
		if m.statsTab == 2 && !m.monthlyDetailMode {
			return m.loadYearMonthlyReportCmd(m.currentYearMonthlyEnd(), now)
		}
		label, start, end := m.currentWindowSpec(now)
		if m.statsTab == 2 && m.monthlyDetailMode {
			return tea.Batch(
				m.loadMonthDailyReportCmd(m.currentMonthlySelection(), now),
				m.loadWindowReportCmd(label, start, end, now),
			)
		}
		return m.loadWindowReportCmd(label, start, end, now)
	}
	start := startOfStatsDay(now)
	return m.loadWindowReportCmd("Daily", start, start.AddDate(0, 0, 1), now)
}

func (m Model) loadOverviewCmd(now time.Time) tea.Cmd {
	if m.projectScope {
		if (m.projectStatsLoaded && now.Sub(m.projectStatsUpdatedAt) < statsViewTTL) || m.projectStatsLoading || m.loadProjectStats == nil {
			return nil
		}
		m.projectStatsLoading = true
		return loadStatsCmd(true, m.loadProjectStats)
	}
	if (m.globalStatsLoaded && now.Sub(m.globalStatsUpdatedAt) < statsViewTTL) || m.globalStatsLoading || m.loadGlobalStats == nil {
		return nil
	}
	m.globalStatsLoading = true
	return loadStatsCmd(false, m.loadGlobalStats)
}

func (m Model) currentWindowSpec(now time.Time) (string, time.Time, time.Time) {
	if m.statsTab == 1 {
		date := m.currentDailyDate()
		if date.IsZero() {
			date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		}
		start := startOfStatsDay(date)
		return "Daily", start, start.AddDate(0, 0, 1)
	}
	start := m.currentMonthlySelection()
	if start.IsZero() {
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}
	return "Monthly", start, start.AddDate(0, 1, 0)
}

func (m Model) loadYearMonthlyReportCmd(endMonth, now time.Time) tea.Cmd {
	endMonth = statsMonthStart(endMonth)
	if m.projectScope {
		if m.currentYearMonthlyFresh(now) || m.projectYearMonthlyLoading || m.loadProjectYearMonthly == nil {
			return nil
		}
		m.setYearMonthlyLoading(true, true)
		return loadYearMonthlyCmd(true, endMonth, m.loadProjectYearMonthly)
	}
	if m.currentYearMonthlyFresh(now) || m.globalYearMonthlyLoading || m.loadGlobalYearMonthly == nil {
		return nil
	}
	m.setYearMonthlyLoading(false, true)
	return loadYearMonthlyCmd(false, endMonth, m.loadGlobalYearMonthly)
}

func (m Model) loadMonthDailyReportCmd(monthStart, now time.Time) tea.Cmd {
	monthStart = statsMonthStart(monthStart)
	if m.projectScope {
		if m.currentMonthDailyFresh(monthStart, now) || m.projectMonthDailyLoading || m.loadProjectMonthDaily == nil {
			return nil
		}
		m.setMonthDailyLoading(true, true)
		return loadMonthDailyCmd(true, monthStart, m.loadProjectMonthDaily)
	}
	if m.currentMonthDailyFresh(monthStart, now) || m.globalMonthDailyLoading || m.loadGlobalMonthDaily == nil {
		return nil
	}
	m.setMonthDailyLoading(false, true)
	return loadMonthDailyCmd(false, monthStart, m.loadGlobalMonthDaily)
}

func (m Model) loadWindowReportCmd(label string, start, end, now time.Time) tea.Cmd {
	if m.projectScope {
		if m.windowFresh(true, label, start, now) || m.windowLoading(true, label) || m.loadProjectWindow == nil {
			return nil
		}
		m.setWindowLoading(true, label, true)
		return loadWindowCmdWithRange(true, label, start, end, func() (stats.WindowReport, error) {
			return m.loadProjectWindow(label, start, end)
		})
	}
	if m.windowFresh(false, label, start, now) || m.windowLoading(false, label) || m.loadGlobalWindow == nil {
		return nil
	}
	m.setWindowLoading(false, label, true)
	return loadWindowCmdWithRange(false, label, start, end, func() (stats.WindowReport, error) {
		return m.loadGlobalWindow(label, start, end)
	})
}

func (m Model) currentYearMonthlyFresh(now time.Time) bool {
	if m.projectScope {
		return m.projectYearMonthlyLoaded && now.Sub(m.projectYearMonthlyUpdatedAt) < statsViewTTL
	}
	return m.globalYearMonthlyLoaded && now.Sub(m.globalYearMonthlyUpdatedAt) < statsViewTTL
}

func (m Model) currentYearMonthlyLoading() bool {
	if m.projectScope {
		return m.projectYearMonthlyLoading
	}
	return m.globalYearMonthlyLoading
}

func (m *Model) setYearMonthlyLoading(project bool, loading bool) {
	if project {
		m.projectYearMonthlyLoading = loading
		return
	}
	m.globalYearMonthlyLoading = loading
}

func (m *Model) setYearMonthlyReport(project bool, report stats.YearMonthlyReport) {
	if project {
		m.projectYearMonthly = report
		m.projectYearMonthlyLoaded = true
		m.projectYearMonthlyUpdatedAt = time.Now()
		return
	}
	m.globalYearMonthly = report
	m.globalYearMonthlyLoaded = true
	m.globalYearMonthlyUpdatedAt = time.Now()
}

func (m Model) currentMonthDailyFresh(month, now time.Time) bool {
	month = statsMonthStart(month)
	if m.projectScope {
		return m.projectMonthDailyLoaded && statsMonthStart(m.projectMonthDailyMonth).Equal(month) && now.Sub(m.projectMonthDailyUpdatedAt) < statsViewTTL
	}
	return m.globalMonthDailyLoaded && statsMonthStart(m.globalMonthDailyMonth).Equal(month) && now.Sub(m.globalMonthDailyUpdatedAt) < statsViewTTL
}

func (m *Model) setMonthDailyLoading(project bool, loading bool) {
	if project {
		m.projectMonthDailyLoading = loading
		return
	}
	m.globalMonthDailyLoading = loading
}

func (m *Model) setMonthDailyReport(project bool, month time.Time, report stats.MonthDailyReport) {
	month = statsMonthStart(month)
	if project {
		m.projectMonthDaily = report
		m.projectMonthDailyLoaded = true
		m.projectMonthDailyMonth = month
		m.projectMonthDailyUpdatedAt = time.Now()
		return
	}
	m.globalMonthDaily = report
	m.globalMonthDailyLoaded = true
	m.globalMonthDailyMonth = month
	m.globalMonthDailyUpdatedAt = time.Now()
}

func (m Model) windowFresh(project bool, label string, start time.Time, now time.Time) bool {
	switch {
	case project && label == "Daily":
		return m.projectDailyLoaded && startOfStatsDay(m.projectDailyDate).Equal(startOfStatsDay(start)) && now.Sub(m.projectDailyUpdatedAt) < statsViewTTL
	case project && label == "Monthly":
		return m.projectMonthlyLoaded && statsMonthStart(m.projectMonthly.Start).Equal(statsMonthStart(start)) && now.Sub(m.projectMonthlyUpdatedAt) < statsViewTTL
	case !project && label == "Daily":
		return m.globalDailyLoaded && startOfStatsDay(m.globalDailyDate).Equal(startOfStatsDay(start)) && now.Sub(m.globalDailyUpdatedAt) < statsViewTTL
	default:
		return m.globalMonthlyLoaded && statsMonthStart(m.globalMonthly.Start).Equal(statsMonthStart(start)) && now.Sub(m.globalMonthlyUpdatedAt) < statsViewTTL
	}
}

func (m Model) windowLoading(project bool, label string) bool {
	switch {
	case project && label == "Daily":
		return m.projectDailyLoading
	case project && label == "Monthly":
		return m.projectMonthlyLoading
	case !project && label == "Daily":
		return m.globalDailyLoading
	default:
		return m.globalMonthlyLoading
	}
}

func (m *Model) setWindowLoading(project bool, label string, loading bool) {
	switch {
	case project && label == "Daily":
		m.projectDailyLoading = loading
	case project && label == "Monthly":
		m.projectMonthlyLoading = loading
	case !project && label == "Daily":
		m.globalDailyLoading = loading
	default:
		m.globalMonthlyLoading = loading
	}
}

func (m *Model) setWindowReport(project bool, label string, report stats.WindowReport) {
	switch {
	case project && label == "Daily":
		m.projectDaily = report
		m.projectDailyLoaded = true
		m.projectDailyDate = startOfStatsDay(report.Start)
	case project && label == "Monthly":
		m.projectMonthly = report
		m.projectMonthlyLoaded = true
	case !project && label == "Daily":
		m.globalDaily = report
		m.globalDailyLoaded = true
		m.globalDailyDate = startOfStatsDay(report.Start)
	default:
		m.globalMonthly = report
		m.globalMonthlyLoaded = true
	}
}

func (m *Model) setWindowUpdatedAt(project bool, label string, updatedAt time.Time) {
	switch {
	case project && label == "Daily":
		m.projectDailyUpdatedAt = updatedAt
	case project && label == "Monthly":
		m.projectMonthlyUpdatedAt = updatedAt
	case !project && label == "Daily":
		m.globalDailyUpdatedAt = updatedAt
	default:
		m.globalMonthlyUpdatedAt = updatedAt
	}
}

func (m Model) updateForStatsMessage(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case statsLoadedMsg:
		if msg.project {
			m.projectStatsLoading = false
			if msg.err == nil {
				m.projectStats = msg.report
				m.projectStatsLoaded = true
				m.projectStatsUpdatedAt = time.Now()
			}
			return m, nil, true
		}
		m.globalStatsLoading = false
		if msg.err == nil {
			m.globalStats = msg.report
			m.globalStatsLoaded = true
			m.globalStatsUpdatedAt = time.Now()
		}
		return m, nil, true
	case windowReportLoadedMsg:
		m.setWindowLoading(msg.project, msg.label, false)
		if msg.err == nil {
			if msg.label == "Daily" {
				if m.statsMode {
					if !m.dailyDetailMode || msg.project != m.projectScope || !startOfStatsDay(msg.start).Equal(m.currentDailyDate()) {
						return m, nil, true
					}
				} else {
					today := startOfStatsDay(time.Now())
					if msg.project != m.projectScope {
						return m, nil, true
					}
					if !startOfStatsDay(msg.start).Equal(today) {
						return m, m.loadCurrentScopeCmd(), true
					}
				}
			}
			m.setWindowReport(msg.project, msg.label, msg.report)
			m.setWindowUpdatedAt(msg.project, msg.label, time.Now())
		}
		return m, nil, true
	case monthDailyReportLoadedMsg:
		m.setMonthDailyLoading(msg.project, false)
		if msg.err == nil {
			expectedMonth := m.currentDailyMonth()
			if m.statsTab == 2 && m.monthlyDetailMode {
				expectedMonth = m.currentMonthlySelection()
			}
			if msg.project != m.projectScope || !statsMonthStart(msg.monthStart).Equal(statsMonthStart(expectedMonth)) {
				return m, nil, true
			}
			m.setMonthDailyReport(msg.project, msg.monthStart, msg.report)
			if m.statsTab == 1 && !m.dailyDetailMode {
				m.syncDailySelectionToMonth()
				m.ensureDailySelectionVisible()
			}
		}
		return m, nil, true
	case yearMonthlyReportLoadedMsg:
		m.setYearMonthlyLoading(msg.project, false)
		if msg.err == nil {
			if msg.project != m.projectScope {
				return m, nil, true
			}
			m.setYearMonthlyReport(msg.project, msg.report)
			m.syncMonthlySelectionToYear()
			m.ensureMonthlySelectionVisible()
		}
		return m, nil, true
	default:
		return m, nil, false
	}
}
