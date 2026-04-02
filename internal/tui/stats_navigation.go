package tui

import (
	"time"

	"github.com/kayden-kim/oc/internal/stats"
)

func (m Model) availableStatsRows() int {
	if m.height <= 0 {
		return 1000
	}
	rows := m.height - m.statsChromeHeight()
	return max(rows, 0)
}

func (m Model) statsChromeHeight() int {
	if m.isNarrowLayout() {
		return 6
	}
	return 7
}

func (m Model) visibleStatsRange(total int) (int, int) {
	if total <= 0 {
		return 0, 0
	}
	visibleRows := m.availableStatsRows()
	if visibleRows <= 0 || visibleRows >= total {
		return 0, total
	}
	start := m.statsOffset
	start = max(start, 0)
	maxOffset := total - visibleRows
	start = min(start, maxOffset)
	end := start + visibleRows
	end = min(end, total)
	return start, end
}

func (m *Model) scrollStats(delta int, total int) {
	if total <= 0 {
		m.statsOffset = 0
		return
	}
	visibleRows := m.availableStatsRows()
	if visibleRows <= 0 || visibleRows >= total {
		m.statsOffset = 0
		return
	}
	maxOffset := total - visibleRows
	m.statsOffset += delta
	m.statsOffset = min(max(m.statsOffset, 0), maxOffset)
}

func (m *Model) pageStats(delta int, total int) {
	m.scrollStats(delta*pageStep(m.availableStatsRows()), total)
}

func (m *Model) halfPageStats(delta int, total int) {
	m.scrollStats(delta*halfPageStep(m.availableStatsRows()), total)
}

func (m *Model) jumpStatsTo(target scrollTarget, total int) {
	m.statsOffset = jumpTarget(target, total, m.availableStatsRows())
}

func (m Model) statsListCanScreenScroll() bool {
	if !m.statsMode {
		return false
	}
	if (m.statsTab == 1 && m.dailyDetailMode) || (m.statsTab == 2 && m.monthlyDetailMode) {
		return false
	}
	total := len(m.statsContentLines())
	visible := m.availableStatsRows()
	return visible > 0 && visible < total
}

func (m *Model) syncStatsListOffsetFromScreenScroll() {
	if m.statsTab == 1 && !m.dailyDetailMode {
		m.dailyListOffset = m.statsOffset
		return
	}
	if m.statsTab == 2 && !m.monthlyDetailMode {
		m.monthlyListOffset = m.statsOffset
	}
}

func (m Model) currentDailyMonth() time.Time {
	if !m.dailyMonthAnchor.IsZero() {
		return statsMonthStart(m.dailyMonthAnchor)
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
}

func (m Model) currentDailyDate() time.Time {
	if !m.dailySelectedDate.IsZero() {
		return startOfStatsDay(m.dailySelectedDate)
	}
	month := m.currentDailyMonth()
	if !month.IsZero() {
		return month
	}
	return startOfStatsDay(time.Now())
}

func (m Model) currentMonthlySelection() time.Time {
	if !m.monthlySelectedMonth.IsZero() {
		return statsMonthStart(m.monthlySelectedMonth)
	}
	if report := m.currentYearMonthly(); len(report.Months) > 0 {
		return statsMonthStart(report.Months[len(report.Months)-1].MonthStart)
	}
	return statsMonthStart(time.Now())
}

func (m Model) currentYearMonthlyEnd() time.Time {
	return statsMonthStart(time.Now())
}

func (m Model) currentYearMonthly() stats.YearMonthlyReport {
	if m.projectScope {
		return m.projectYearMonthly
	}
	return m.globalYearMonthly
}

func (m Model) yearMonthlyRows() []stats.MonthlySummary {
	report := m.currentYearMonthly()
	rows := make([]stats.MonthlySummary, 0, len(report.Months))
	for i := len(report.Months) - 1; i >= 0; i-- {
		rows = append(rows, report.Months[i])
	}
	return rows
}

func (m Model) monthlySelectionIndex() int {
	rows := m.yearMonthlyRows()
	if len(rows) == 0 {
		return -1
	}
	selected := m.currentMonthlySelection()
	for i, row := range rows {
		if statsMonthStart(row.MonthStart).Equal(selected) {
			return i
		}
	}
	return 0
}

func (m *Model) syncMonthlySelectionToYear() {
	rows := m.yearMonthlyRows()
	if len(rows) == 0 {
		m.monthlySelectedMonth = time.Time{}
		return
	}
	idx := m.monthlySelectionIndex()
	if idx < 0 || idx >= len(rows) {
		idx = 0
	}
	m.monthlySelectedMonth = statsMonthStart(rows[idx].MonthStart)
}

func (m *Model) ensureMonthlySelectionVisible() {
	total := len(m.statsContentLines())
	if total <= 0 {
		m.monthlyListOffset = 0
		m.statsOffset = 0
		return
	}
	idx := m.monthlySelectionIndex()
	if idx < 0 {
		m.monthlyListOffset = 0
		m.statsOffset = 0
		return
	}
	lineIndex := m.yearMonthlyRowLineOffset() + idx
	visibleRows := m.availableStatsRows()
	if visibleRows <= 0 || visibleRows >= total {
		m.monthlyListOffset = 0
		m.statsOffset = 0
		return
	}
	if m.monthlyListOffset > total-visibleRows {
		m.monthlyListOffset = max(0, total-visibleRows)
	}
	if lineIndex < m.monthlyListOffset {
		m.monthlyListOffset = lineIndex
	}
	if lineIndex >= m.monthlyListOffset+visibleRows {
		m.monthlyListOffset = lineIndex - visibleRows + 1
	}
	if m.monthlyListOffset < 0 {
		m.monthlyListOffset = 0
	}
	if maxOffset := max(0, total-visibleRows); m.monthlyListOffset > maxOffset {
		m.monthlyListOffset = maxOffset
	}
	m.statsOffset = m.monthlyListOffset
}

func (m Model) yearMonthlyRowLineOffset() int {
	if m.isNarrowLayout() {
		return 3
	}
	return 13
}

func (m *Model) moveMonthlySelection(delta int) {
	rows := m.yearMonthlyRows()
	if len(rows) == 0 {
		return
	}
	idx := m.monthlySelectionIndex()
	idx = max(idx, 0)
	idx = clampCursor(idx+delta, len(rows))
	m.monthlySelectedMonth = statsMonthStart(rows[idx].MonthStart)
	m.ensureMonthlySelectionVisible()
}

func (m *Model) jumpMonthlySelection(target scrollTarget) {
	rows := m.yearMonthlyRows()
	if len(rows) == 0 {
		return
	}
	idx := 0
	if target == scrollTargetBottom {
		idx = len(rows) - 1
	}
	m.monthlySelectedMonth = statsMonthStart(rows[idx].MonthStart)
	m.ensureMonthlySelectionVisible()
}

func (m *Model) pageMonthlySelection(delta int) {
	m.moveMonthlySelection(delta * pageStep(m.availableStatsRows()))
}

func (m *Model) halfPageMonthlySelection(delta int) {
	m.moveMonthlySelection(delta * halfPageStep(m.availableStatsRows()))
}

func (m *Model) enterMonthlyDetail() {
	m.syncMonthlySelectionToYear()
	m.monthlyDetailMode = true
	m.monthlyDetailOffset = 0
	m.statsOffset = 0
}

func (m *Model) exitMonthlyDetail() {
	m.monthlyDetailMode = false
	m.statsOffset = m.monthlyListOffset
	m.ensureMonthlySelectionVisible()
}

func (m Model) monthDailyRows() []stats.DailySummary {
	return m.currentMonthDaily().Days
}

func (m Model) dailySelectionIndex() int {
	rows := m.monthDailyRows()
	if len(rows) == 0 {
		return -1
	}
	selected := m.currentDailyDate()
	for i, row := range rows {
		if startOfStatsDay(row.Date).Equal(selected) {
			return i
		}
	}
	return 0
}

func (m *Model) syncDailySelectionToMonth() {
	rows := m.monthDailyRows()
	if len(rows) == 0 {
		m.dailySelectedDate = time.Time{}
		return
	}
	idx := m.dailySelectionIndex()
	if idx < 0 || idx >= len(rows) {
		idx = 0
	}
	m.dailySelectedDate = startOfStatsDay(rows[idx].Date)
}

func (m *Model) ensureDailySelectionVisible() {
	total := len(m.statsContentLines())
	if total <= 0 {
		m.dailyListOffset = 0
		m.statsOffset = 0
		return
	}
	idx := m.dailySelectionIndex()
	if idx < 0 {
		m.dailyListOffset = 0
		m.statsOffset = 0
		return
	}
	lineIndex := m.monthDailyRowLineOffset() + idx
	visibleRows := m.availableStatsRows()
	if visibleRows <= 0 || visibleRows >= total {
		m.dailyListOffset = 0
		m.statsOffset = 0
		return
	}
	if m.dailyListOffset > total-visibleRows {
		m.dailyListOffset = max(0, total-visibleRows)
	}
	if lineIndex < m.dailyListOffset {
		m.dailyListOffset = lineIndex
	}
	if lineIndex >= m.dailyListOffset+visibleRows {
		m.dailyListOffset = lineIndex - visibleRows + 1
	}
	if m.dailyListOffset < 0 {
		m.dailyListOffset = 0
	}
	if maxOffset := max(0, total-visibleRows); m.dailyListOffset > maxOffset {
		m.dailyListOffset = maxOffset
	}
	m.statsOffset = m.dailyListOffset
}

func (m Model) monthDailyRowLineOffset() int {
	if m.isNarrowLayout() {
		return 4
	}
	return 4
}

func (m *Model) moveDailySelection(delta int) {
	rows := m.monthDailyRows()
	if len(rows) == 0 {
		return
	}
	idx := m.dailySelectionIndex()
	idx = max(idx, 0)
	idx = clampCursor(idx+delta, len(rows))
	m.dailySelectedDate = startOfStatsDay(rows[idx].Date)
	m.ensureDailySelectionVisible()
}

func (m *Model) jumpDailySelection(target scrollTarget) {
	rows := m.monthDailyRows()
	if len(rows) == 0 {
		return
	}
	idx := 0
	if target == scrollTargetBottom {
		idx = len(rows) - 1
	}
	m.dailySelectedDate = startOfStatsDay(rows[idx].Date)
	m.ensureDailySelectionVisible()
}

func (m *Model) pageDailySelection(delta int) {
	m.moveDailySelection(delta * pageStep(m.availableStatsRows()))
}

func (m *Model) halfPageDailySelection(delta int) {
	m.moveDailySelection(delta * halfPageStep(m.availableStatsRows()))
}

func (m *Model) enterDailyDetail() {
	m.syncDailySelectionToMonth()
	m.dailyDetailMode = true
	m.dailyDetailOffset = 0
	m.statsOffset = 0
}

func (m *Model) exitDailyDetail() {
	m.dailyDetailMode = false
	m.statsOffset = m.dailyListOffset
	m.ensureDailySelectionVisible()
}

func (m *Model) navigateDailyMonth(delta int) {
	month := m.currentDailyMonth()
	if month.IsZero() {
		month = statsMonthStart(time.Now())
	}
	currentMonth := statsMonthStart(time.Now())
	selected := m.currentDailyDate()
	preferredDay := selected.Day()
	newMonth := month.AddDate(0, delta, 0)
	if newMonth.After(currentMonth) {
		newMonth = currentMonth
	}
	lastDay := newMonth.AddDate(0, 1, -1).Day()
	if preferredDay > lastDay {
		preferredDay = lastDay
	}
	m.dailyMonthAnchor = newMonth
	m.dailySelectedDate = time.Date(newMonth.Year(), newMonth.Month(), preferredDay, 0, 0, 0, 0, newMonth.Location())
	m.dailyListOffset = 0
	m.statsOffset = 0
}
