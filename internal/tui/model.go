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

// EditChoice represents a config file that can be opened from the TUI.
type EditChoice struct {
	Label string
	Path  string
}

// Model holds the state of the multi-select TUI
type Model struct {
	plugins     []PluginItem
	editChoices []EditChoice
	version     string
	cursor      int
	editCursor  int
	selected    map[int]struct{}
	cancelled   bool
	confirmed   bool
	edit        bool
	editMode    bool
	editTarget  string
}

var (
	headerAccentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	cursorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	cursorSelectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	helpKeyStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
)

func (m Model) renderHeader() string {
	return "⚡ oc " + m.version + " : Launching " + headerAccentStyle.Render("OpenCode") + " with plugins"
}

func stylePluginRow(line string, focused bool, selected bool) string {
	switch {
	case focused && selected:
		return cursorSelectedStyle.Render(line)
	case focused:
		return cursorStyle.Render(line)
	default:
		return line
	}
}

func renderHelpLine() string {
	return "💡 " + helpKeyStyle.Render("↑/↓") + ": navigate • " +
		helpKeyStyle.Render("space") + ": toggle • " +
		helpKeyStyle.Render("enter") + ": confirm • " +
		helpKeyStyle.Render("e") + ": edit config... • " +
		helpKeyStyle.Render("q") + ": quit"
}

func renderEditHelpLine() string {
	return "💡 " + helpKeyStyle.Render("↑/↓") + ": navigate • " +
		helpKeyStyle.Render("enter") + ": edit • " +
		helpKeyStyle.Render("esc") + ": back"
}

// NewModel creates a new TUI model with the given plugin items
func NewModel(items []PluginItem, editChoices []EditChoice, version string) Model {
	selected := make(map[int]struct{})
	for i, item := range items {
		if item.InitiallyEnabled {
			selected[i] = struct{}{}
		}
	}

	// Empty list: auto-confirm immediately
	confirmed := len(items) == 0

	return Model{
		plugins:     items,
		editChoices: editChoices,
		version:     version,
		cursor:      0,
		editCursor:  0,
		selected:    selected,
		confirmed:   confirmed,
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
			if m.editMode {
				if m.editCursor > 0 {
					m.editCursor--
				}
			} else if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.editMode {
				if m.editCursor < len(m.editChoices)-1 {
					m.editCursor++
				}
			} else if m.cursor < len(m.plugins)-1 {
				m.cursor++
			}
		case " ", "space": // Space key toggles selection (v2 uses "space")
			if !m.editMode {
				if _, ok := m.selected[m.cursor]; ok {
					delete(m.selected, m.cursor)
				} else {
					m.selected[m.cursor] = struct{}{}
				}
			}
		case "enter":
			if m.editMode {
				m.edit = true
				m.editTarget = m.editChoices[m.editCursor].Path
				return m, tea.Quit
			}
			m.confirmed = true
			return m, tea.Quit
		case "e":
			if len(m.editChoices) > 0 {
				m.editMode = true
				m.editCursor = 0
			}
		case "ctrl+c", "q", "esc":
			if m.editMode && (msg.String() == "q" || msg.String() == "esc") {
				m.editMode = false
				return m, nil
			}
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

	if m.editMode {
		s := "📂 Choose config to edit\n\n"

		for i, choice := range m.editChoices {
			cursor := "  "
			focused := m.editCursor == i
			if focused {
				cursor = "> "
			}

			line := fmt.Sprintf("%s%s", cursor, choice.Label)
			line = stylePluginRow(line, focused, false)

			s += line + "\n"
		}

		s += "\n" + renderEditHelpLine()

		return tea.NewView(s)
	}

	s := m.renderHeader() + "\n\n"

	for i, p := range m.plugins {
		cursor := "  "
		focused := m.cursor == i
		if focused {
			cursor = "> "
		}

		checked := "   "
		_, selected := m.selected[i]
		if selected {
			checked = "✔  "
		}

		line := fmt.Sprintf("%s%s%s", cursor, checked, p.Name)
		line = stylePluginRow(line, focused, selected)

		s += line + "\n"
	}

	s += "\n" + renderHelpLine()

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

// EditTarget returns the selected config file path when edit was requested.
func (m Model) EditTarget() string {
	return m.editTarget
}
