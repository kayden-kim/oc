package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// PluginItem represents a plugin that can be selected in the TUI
type PluginItem struct {
	Name             string
	InitiallyEnabled bool
}

// Model holds the state of the multi-select TUI
type Model struct {
	plugins   []PluginItem
	cursor    int
	selected  map[int]struct{}
	cancelled bool
	confirmed bool
	edit      bool
}

var (
	cursorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("cyan"))
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("green"))
)

// NewModel creates a new TUI model with the given plugin items
func NewModel(items []PluginItem) Model {
	selected := make(map[int]struct{})
	for i, item := range items {
		if item.InitiallyEnabled {
			selected[i] = struct{}{}
		}
	}

	// Empty list: auto-confirm immediately
	confirmed := len(items) == 0

	return Model{
		plugins:   items,
		cursor:    0,
		selected:  selected,
		confirmed: confirmed,
	}
}

// Init initializes the model (no initial command needed)
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles state transitions based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.plugins)-1 {
				m.cursor++
			}
		case " ", "space": // Space key toggles selection (v2 uses "space")
			if _, ok := m.selected[m.cursor]; ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		case "enter":
			m.confirmed = true
			return m, tea.Quit
		case "e":
			m.edit = true
			return m, tea.Quit
		case "ctrl+c", "q", "esc":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the TUI
func (m Model) View() tea.View {
	if len(m.plugins) == 0 {
		return tea.NewView("")
	}

	s := "Select plugins (Space: toggle, Enter: confirm, e: edit opencode.json, q: quit):\n\n"

	for i, p := range m.plugins {
		cursor := "  "
		if m.cursor == i {
			cursor = "> "
		}

		checked := " "
		if _, ok := m.selected[i]; ok {
			checked = "*"
		}

		line := fmt.Sprintf("%s[%s] %s", cursor, checked, p.Name)

		if m.cursor == i {
			line = cursorStyle.Render(line)
		} else if _, ok := m.selected[i]; ok {
			line = selectedStyle.Render(line)
		}

		s += line + "\n"
	}

	s += "\n↑/↓: navigate • space: toggle • enter: confirm • e: edit config • q: quit"

	return tea.NewView(s)
}

// Selections returns a map of plugin names to their selection state
func (m Model) Selections() map[string]bool {
	result := make(map[string]bool)
	for i, p := range m.plugins {
		_, isSelected := m.selected[i]
		result[p.Name] = isSelected
	}
	return result
}

// Cancelled returns true if the user cancelled the TUI
func (m Model) Cancelled() bool {
	return m.cancelled
}

// EditRequested returns true if the user chose to open the config in an editor.
func (m Model) EditRequested() bool {
	return m.edit
}
