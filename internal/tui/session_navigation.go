package tui

func (m Model) sessionAt(cursor int) SessionItem {
	if cursor <= 0 || cursor > len(m.sessions) {
		return SessionItem{}
	}

	return m.sessions[cursor-1]
}

func (m Model) availableSessionRows() int {
	if m.height <= 0 {
		return len(m.sessions) + 1
	}

	rows := m.height - sessionChromeHeight
	return max(rows, 0)
}

func (m *Model) ensureSessionCursorVisible() {
	totalRows := len(m.sessions) + 1
	if totalRows <= 0 {
		m.sessionOffset = 0
		return
	}

	visibleRows := m.availableSessionRows()
	if visibleRows <= 0 || visibleRows >= totalRows {
		m.sessionOffset = 0
		return
	}

	maxOffset := totalRows - visibleRows
	if m.sessionOffset > maxOffset {
		m.sessionOffset = maxOffset
	}
	if m.sessionOffset < 0 {
		m.sessionOffset = 0
	}

	if m.sessionCursor < m.sessionOffset {
		m.sessionOffset = m.sessionCursor
	}
	if m.sessionCursor >= m.sessionOffset+visibleRows {
		m.sessionOffset = m.sessionCursor - visibleRows + 1
	}

	if m.sessionOffset > maxOffset {
		m.sessionOffset = maxOffset
	}
	if m.sessionOffset < 0 {
		m.sessionOffset = 0
	}
}

func (m Model) visibleSessionRange() (int, int) {
	totalRows := len(m.sessions) + 1
	if totalRows <= 0 {
		return 0, 0
	}

	visibleRows := m.availableSessionRows()
	if visibleRows <= 0 {
		return 0, 0
	}
	if visibleRows >= totalRows {
		return 0, totalRows
	}

	start := m.sessionOffset
	start = max(start, 0)
	maxOffset := totalRows - visibleRows
	start = min(start, maxOffset)

	end := start + visibleRows
	end = min(end, totalRows)

	return start, end
}

func (m *Model) moveSessionCursor(delta int) {
	m.sessionCursor = clampCursor(m.sessionCursor+delta, len(m.sessions)+1)
	m.ensureSessionCursorVisible()
}

func (m *Model) pageSession(delta int) {
	m.moveSessionCursor(delta * pageStep(m.availableSessionRows()))
}

func (m *Model) halfPageSession(delta int) {
	m.moveSessionCursor(delta * halfPageStep(m.availableSessionRows()))
}

func (m *Model) jumpSessionTo(target scrollTarget) {
	total := len(m.sessions) + 1
	if target == scrollTargetTop {
		m.sessionCursor = 0
	} else {
		m.sessionCursor = total - 1
	}
	m.ensureSessionCursorVisible()
}
