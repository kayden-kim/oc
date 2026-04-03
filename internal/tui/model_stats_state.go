package tui

import (
	"time"

	"github.com/kayden-kim/oc/internal/stats"
)

func (m Model) WithStatsLoaders(global func() (stats.Report, error), project func() (stats.Report, error), globalWindow func(string, time.Time, time.Time) (stats.WindowReport, error), projectWindow func(string, time.Time, time.Time) (stats.WindowReport, error)) Model {
	m.loadGlobalStats = global
	m.loadProjectStats = project
	m.loadGlobalWindow = globalWindow
	m.loadProjectWindow = projectWindow
	m.globalStatsLoaded = false
	m.projectStatsLoaded = false
	return m
}

func (m Model) WithYearMonthlyLoaders(global func(time.Time) (stats.YearMonthlyReport, error), project func(time.Time) (stats.YearMonthlyReport, error)) Model {
	m.loadGlobalYearMonthly = global
	m.loadProjectYearMonthly = project
	m.globalYearMonthlyLoaded = false
	m.projectYearMonthlyLoaded = false
	return m
}

func (m Model) WithMonthDailyLoaders(global func(time.Time) (stats.MonthDailyReport, error), project func(time.Time) (stats.MonthDailyReport, error)) Model {
	m.loadGlobalMonthDaily = global
	m.loadProjectMonthDaily = project
	m.globalMonthDailyLoaded = false
	m.projectMonthDailyLoaded = false
	return m
}

func statsMonthStart(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func startOfStatsDay(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func (m *Model) resetDailyState(now time.Time) {
	month := statsMonthStart(now)
	m.dailyDetailMode = false
	m.dailyMonthAnchor = month
	m.dailySelectedDate = startOfStatsDay(now)
	m.dailyListOffset = 0
	m.dailyDetailOffset = 0
	m.statsOffset = 0
	if m.dailySelectedDate.Before(month) || !m.dailySelectedDate.Before(month.AddDate(0, 1, 0)) {
		m.dailySelectedDate = month
	}
}

func (m *Model) resetMonthlyState(now time.Time) {
	month := statsMonthStart(now)
	m.monthlyDetailMode = false
	m.monthlySelectedMonth = month
	m.monthlyListOffset = 0
	m.monthlyDetailOffset = 0
	m.statsOffset = 0
}
