package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleSessionMoveKey(delta int) Model {
	m.moveSessionCursor(delta)
	return m
}

func (m Model) handleSessionPageKey(delta int) Model {
	m.pageSession(delta)
	return m
}

func (m Model) handleSessionHalfPageKey(delta int) Model {
	m.halfPageSession(delta)
	return m
}

func (m Model) handleSessionJumpKey(target scrollTarget) Model {
	m.jumpSessionTo(target)
	return m
}

func (m Model) handleSessionEnterKey() Model {
	m.session = m.sessionAt(m.sessionCursor)
	m.sessionMode = false
	return m
}

func (m Model) handleSessionBackKey() Model {
	m.sessionMode = false
	return m
}

func (m Model) handleEditMoveKey(delta int) Model {
	if delta < 0 && m.editCursor > 0 {
		m.editCursor--
	}
	if delta > 0 && m.editCursor < len(m.editChoices)-1 {
		m.editCursor++
	}
	return m
}

func (m Model) handleEditEnterKey() Model {
	m.edit = true
	m.editTarget = m.editChoices[m.editCursor].Path
	return m
}

func (m Model) handleEditBackKey() Model {
	m.editMode = false
	return m
}

func (m Model) handleLauncherMoveKey(delta int) Model {
	if delta < 0 && m.cursor > 0 {
		m.cursor--
	}
	if delta > 0 && m.cursor < len(m.plugins)-1 {
		m.cursor++
	}
	return m
}

func (m Model) handleLauncherToggleKey() Model {
	if !m.editMode && !m.sessionMode && !m.statsMode {
		if _, ok := m.selected[m.cursor]; ok {
			delete(m.selected, m.cursor)
		} else {
			if !m.allowMultiplePlugins {
				m.selected = map[int]struct{}{}
			}
			m.selected[m.cursor] = struct{}{}
		}
	}
	return m
}

func (m Model) handleLauncherEnterKey() Model {
	m.confirmed = true
	return m
}

func (m Model) handleLauncherSessionKey() Model {
	m.sessionMode = true
	for i, item := range m.sessions {
		if item.ID == m.session.ID {
			m.sessionCursor = i + 1
			m.ensureSessionCursorVisible()
			return m
		}
	}
	m.sessionCursor = 0
	m.ensureSessionCursorVisible()
	return m
}

func (m Model) handleLauncherEditKey() Model {
	if !m.sessionMode && !m.statsMode && len(m.editChoices) > 0 {
		m.editMode = true
		m.editCursor = 0
	}
	return m
}

func (m Model) handleLauncherQuitKey() Model {
	m.cancelled = true
	return m
}

func (m *Model) syncStatsOffsetForActiveTab() {
	if m.statsTab == 1 && !m.dailyDetailMode {
		m.statsOffset = m.dailyListOffset
	} else if m.statsTab == 1 {
		m.statsOffset = m.dailyDetailOffset
	} else if m.statsTab == 2 && !m.monthlyDetailMode {
		m.statsOffset = m.monthlyListOffset
	} else if m.statsTab == 2 {
		m.statsOffset = m.monthlyDetailOffset
	} else {
		m.statsOffset = 0
	}
}

func (m *Model) syncStatsDetailOffset() {
	if m.statsTab == 1 && m.dailyDetailMode {
		m.dailyDetailOffset = m.statsOffset
	} else if m.statsTab == 2 && m.monthlyDetailMode {
		m.monthlyDetailOffset = m.statsOffset
	}
}

func (m Model) handleStatsToggleKey() (Model, tea.Cmd, bool) {
	m.statsMode = !m.statsMode
	if m.statsMode {
		now := time.Now()
		m.resetDailyState(now)
		m.resetMonthlyState(now)
		return m, m.loadCurrentScopeCmd(), true
	}

	now := time.Now()
	start := startOfStatsDay(now)
	m.statsOffset = 0
	if m.windowLoading(m.projectScope, "Daily") || m.windowFresh(m.projectScope, "Daily", start, now) {
		return m, nil, true
	}
	return m, m.loadCurrentScopeCmd(), true
}

func (m Model) handleStatsScopeToggleKey() (Model, tea.Cmd, bool) {
	m.projectScope = !m.projectScope
	m.syncStatsOffsetForActiveTab()
	now := time.Now()
	if !m.statsMode {
		start := startOfStatsDay(now)
		if m.windowLoading(m.projectScope, "Daily") || m.windowFresh(m.projectScope, "Daily", start, now) {
			return m, nil, true
		}
	}
	return m, m.loadCurrentScopeCmd(), true
}

func (m Model) handleStatsVerticalKey(delta int) Model {
	if m.statsTab == 1 && !m.dailyDetailMode {
		m.moveDailySelection(delta)
		return m
	}
	if m.statsTab == 2 && !m.monthlyDetailMode {
		m.moveMonthlySelection(delta)
		return m
	}
	m.scrollStats(delta, len(m.statsContentLines()))
	m.syncStatsDetailOffset()
	return m
}

func (m Model) handleStatsPageKey(delta int) Model {
	if m.statsListCanScreenScroll() {
		m.pageStats(delta, len(m.statsContentLines()))
		m.syncStatsListOffsetFromScreenScroll()
		return m
	}
	if m.statsTab == 1 && !m.dailyDetailMode {
		m.pageDailySelection(delta)
		return m
	}
	if m.statsTab == 2 && !m.monthlyDetailMode {
		m.pageMonthlySelection(delta)
		return m
	}
	m.pageStats(delta, len(m.statsContentLines()))
	m.syncStatsDetailOffset()
	return m
}

func (m Model) handleStatsHalfPageKey(delta int) Model {
	if m.statsListCanScreenScroll() {
		m.halfPageStats(delta, len(m.statsContentLines()))
		m.syncStatsListOffsetFromScreenScroll()
		return m
	}
	if m.statsTab == 1 && !m.dailyDetailMode {
		m.halfPageDailySelection(delta)
		return m
	}
	if m.statsTab == 2 && !m.monthlyDetailMode {
		m.halfPageMonthlySelection(delta)
		return m
	}
	m.halfPageStats(delta, len(m.statsContentLines()))
	m.syncStatsDetailOffset()
	return m
}

func (m Model) handleStatsScreenScrollKey(delta int) Model {
	m.scrollStats(delta, len(m.statsContentLines()))
	m.syncStatsListOffsetFromScreenScroll()
	return m
}

func (m Model) handleStatsJumpKey(target scrollTarget) Model {
	if m.statsTab == 1 && !m.dailyDetailMode {
		m.jumpDailySelection(target)
		return m
	}
	if m.statsTab == 2 && !m.monthlyDetailMode {
		m.jumpMonthlySelection(target)
		return m
	}
	m.jumpStatsTo(target, len(m.statsContentLines()))
	m.syncStatsDetailOffset()
	return m
}

func (m Model) handleStatsEnterKey() (Model, tea.Cmd, bool) {
	if m.statsTab == 1 && !m.dailyDetailMode {
		m.enterDailyDetail()
		return m, m.loadCurrentScopeCmd(), true
	}
	if m.statsTab == 2 && !m.monthlyDetailMode {
		m.enterMonthlyDetail()
		return m, m.loadCurrentScopeCmd(), true
	}
	return m, nil, true
}

func (m Model) handleStatsMonthNavKey(delta int) (Model, tea.Cmd, bool) {
	m.navigateDailyMonth(delta)
	return m, m.loadCurrentScopeCmd(), true
}

func (m Model) handleStatsTabKey(delta int) (Model, tea.Cmd, bool) {
	m.statsTab += delta
	m.syncStatsOffsetForActiveTab()
	return m, m.loadCurrentScopeCmd(), true
}

func (m Model) handleStatsBackKey() Model {
	if m.statsTab == 1 && m.dailyDetailMode {
		m.exitDailyDetail()
		return m
	}
	if m.statsTab == 2 && m.monthlyDetailMode {
		m.exitMonthlyDetail()
		return m
	}
	m.statsMode = false
	return m
}
