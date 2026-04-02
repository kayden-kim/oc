package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m Model) viewSessionPicker() tea.View {
	var sb strings.Builder
	sb.WriteString(m.renderTopBadge())
	sb.WriteString("\n\n")
	sb.WriteString(renderSectionHeader("🕘 Choose session", m.layoutWidth()))
	sb.WriteString("\n\n")
	start, end := m.visibleSessionRange()

	for i := start; i < end; i++ {
		sb.WriteString(m.renderSessionRow(i))
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	sb.WriteString(renderSessionHelpLine(m.layoutWidth()))

	return tea.NewView(sb.String())
}

func (m Model) renderSessionRow(index int) string {
	cursor := "  "
	focused := m.sessionCursor == index
	if focused {
		cursor = "> "
	}

	rowText := "Start without session"
	if item := m.sessionAt(index); item.ID != "" {
		rowText = selectedSessionSummary(item, max(0, m.layoutWidth()-lipgloss.Width(cursor)))
	}
	line := fmt.Sprintf("%s%s", cursor, rowText)
	line = truncateDisplayWidth(line, m.layoutWidth())
	return stylePluginRow(line, focused, m.sessionAt(index).ID == m.session.ID)
}
