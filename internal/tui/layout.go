package tui

const (
	maxLayoutWidth         = 80
	minSparklineWidth      = 27
	minMinimapDayCountWide = 28
	minMinimapDayCountSlim = 21
)

func (m Model) layoutWidth() int {
	if m.width <= 0 {
		return maxLayoutWidth
	}
	if m.width > maxLayoutWidth {
		return maxLayoutWidth
	}
	return m.width
}

func (m Model) isNarrowLayout() bool {
	return m.width > 0 && m.width < maxLayoutWidth
}

func (m Model) statsTableMaxWidth() int {
	contentWidth := m.layoutWidth() - 4
	if contentWidth <= 0 {
		return 0
	}
	if contentWidth > statsTableMaxWidth {
		return statsTableMaxWidth
	}
	return contentWidth
}

func (m Model) launcherVisualWidth() int {
	available := m.layoutWidth() - 4 - rhythmFirstColumnWidth
	if available < 0 {
		return 0
	}
	return available
}
