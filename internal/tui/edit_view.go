package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) viewEditPicker() tea.View {
	var sb strings.Builder
	sb.WriteString(m.renderTopBadge())
	sb.WriteString("\n\n")
	sb.WriteString(renderSectionHeader("📂 Choose config to edit", m.layoutWidth()))
	sb.WriteString("\n\n")

	for i, choice := range m.editChoices {
		sb.WriteString(m.renderEditChoiceRow(i, choice))
		sb.WriteByte('\n')
	}

	sb.WriteByte('\n')
	sb.WriteString(renderEditHelpLine(m.layoutWidth()))

	return tea.NewView(sb.String())
}

func (m Model) renderEditChoiceRow(index int, choice EditChoice) string {
	cursor := "  "
	focused := m.editCursor == index
	if focused {
		cursor = "> "
	}

	line := truncateDisplayWidth(fmt.Sprintf("%s%s", cursor, choice.Label), m.layoutWidth())
	return stylePluginRow(line, focused, false)
}
