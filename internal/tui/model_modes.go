package tui

import tea "charm.land/bubbletea/v2"

func (m Model) updateForAsyncMessage(msg tea.Msg) (Model, tea.Cmd, bool) {
	return m.updateForStatsMessage(msg)
}

func (m Model) updateForWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ensureSessionCursorVisible()
	return m, nil
}

func (m Model) updateForKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if updated, cmd, handled := m.updateForStatsKey(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.updateForSessionModeKey(msg); handled {
		return updated, cmd
	}
	if updated, cmd, handled := m.updateForEditModeKey(msg); handled {
		return updated, cmd
	}
	return m.updateForLauncherModeKey(msg)
}

func (m Model) updateForSessionModeKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	if !m.sessionMode {
		return m, nil, false
	}

	switch msg.String() {
	case "up", "k":
		return m.handleSessionMoveKey(-1), nil, true
	case "down", "j":
		return m.handleSessionMoveKey(1), nil, true
	case "pgup":
		return m.handleSessionPageKey(-1), nil, true
	case "pgdown":
		return m.handleSessionPageKey(1), nil, true
	case "ctrl+u":
		return m.handleSessionHalfPageKey(-1), nil, true
	case "ctrl+d":
		return m.handleSessionHalfPageKey(1), nil, true
	case "home":
		return m.handleSessionJumpKey(scrollTargetTop), nil, true
	case "end":
		return m.handleSessionJumpKey(scrollTargetBottom), nil, true
	case "enter":
		return m.handleSessionEnterKey(), nil, true
	case "q", "esc":
		return m.handleSessionBackKey(), nil, true
	default:
		return m, nil, false
	}
}

func (m Model) updateForEditModeKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	if !m.editMode {
		return m, nil, false
	}

	switch msg.String() {
	case "up", "k":
		return m.handleEditMoveKey(-1), nil, true
	case "down", "j":
		return m.handleEditMoveKey(1), nil, true
	case "enter":
		return m.handleEditEnterKey(), tea.Quit, true
	case "q", "esc":
		return m.handleEditBackKey(), nil, true
	default:
		return m, nil, false
	}
}

func (m Model) updateForStatsKey(msg tea.KeyPressMsg) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "tab":
		if m.editMode || m.sessionMode {
			return m, nil, false
		}
		return m.handleStatsToggleKey()
	case "g":
		if m.editMode || m.sessionMode {
			return m, nil, false
		}
		return m.handleStatsScopeToggleKey()
	case "up", "k":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsVerticalKey(-1), nil, true
	case "down", "j":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsVerticalKey(1), nil, true
	case "pgup":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsPageKey(-1), nil, true
	case "pgdown":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsPageKey(1), nil, true
	case "ctrl+u":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsHalfPageKey(-1), nil, true
	case "ctrl+d":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsHalfPageKey(1), nil, true
	case "ctrl+up":
		if m.statsMode && m.statsListCanScreenScroll() {
			return m.handleStatsScreenScrollKey(-1), nil, true
		}
		return m, nil, false
	case "ctrl+down":
		if m.statsMode && m.statsListCanScreenScroll() {
			return m.handleStatsScreenScrollKey(1), nil, true
		}
		return m, nil, false
	case "home":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsJumpKey(scrollTargetTop), nil, true
	case "end":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsJumpKey(scrollTargetBottom), nil, true
	case "enter":
		if !m.statsMode {
			return m, nil, false
		}
		return m.handleStatsEnterKey()
	case "[":
		if m.statsMode && m.statsTab == 1 && !m.dailyDetailMode {
			return m.handleStatsMonthNavKey(-1)
		}
		return m, nil, false
	case "]":
		if m.statsMode && m.statsTab == 1 && !m.dailyDetailMode {
			return m.handleStatsMonthNavKey(1)
		}
		return m, nil, false
	case "left", "h":
		if m.statsMode && m.statsTab > 0 {
			return m.handleStatsTabKey(-1)
		}
		return m, nil, false
	case "right", "l":
		if m.statsMode && m.statsTab < len(statsTabTitles())-1 {
			return m.handleStatsTabKey(1)
		}
		return m, nil, false
	case "s":
		if m.statsMode {
			return m, nil, true
		}
		return m, nil, false
	case "ctrl+c", "q", "esc":
		if m.statsMode && msg.String() == "esc" {
			return m.handleStatsBackKey(), nil, true
		}
		return m, nil, false
	default:
		return m, nil, false
	}
}

func (m Model) updateForLauncherModeKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m = m.handleLauncherMoveKey(-1)
	case "down", "j":
		m = m.handleLauncherMoveKey(1)
	case " ", "space":
		m = m.handleLauncherToggleKey()
	case "enter":
		return m.handleLauncherEnterKey(), tea.Quit
	case "s":
		m = m.handleLauncherSessionKey()
	case "c":
		m = m.handleLauncherEditKey()
	case "ctrl+c", "q", "esc":
		return m.handleLauncherQuitKey(), tea.Quit
	}

	return m, nil
}
