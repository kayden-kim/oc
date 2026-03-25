package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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

type SessionItem struct {
	ID        string
	Title     string
	UpdatedAt time.Time
}

// Model holds the state of the multi-select TUI
type Model struct {
	plugins              []PluginItem
	editChoices          []EditChoice
	version              string
	allowMultiplePlugins bool
	sessions             []SessionItem
	session              SessionItem
	cursor               int
	editCursor           int
	sessionCursor        int
	selected             map[int]struct{}
	cancelled            bool
	confirmed            bool
	edit                 bool
	editMode             bool
	sessionMode          bool
	editTarget           string
	height               int
	sessionOffset        int
}

const sessionChromeHeight = 6

var (
	defaultTextStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A"))
	instructionTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#999999"))
	cursorStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	cursorSelectedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	helpKeyStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	sessionContainerStyle = lipgloss.NewStyle()
	sessionLabelStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(lipgloss.Color("#FF9900")).Bold(true).Padding(0, 1)
	sessionContentStyle   = lipgloss.NewStyle().Background(lipgloss.Color("#292929")).Padding(0, 1)
	sessionValueStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#C0C0C0")).Background(lipgloss.Color("#292929")).Bold(false)
	sessionMetaStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Background(lipgloss.Color("#393939")).Bold(false).Padding(0, 1)
)

func (m Model) renderTopBadge() string {
	return sessionContainerStyle.Render(lipgloss.JoinHorizontal(
		lipgloss.Top,
		sessionLabelStyle.Render("OC"),
		sessionContentStyle.Render(sessionValueStyle.Render(m.version)),
		sessionMetaStyle.Render(selectedSessionSummary(m.session)),
	))
}

func selectedSessionSummary(session SessionItem) string {
	if session.ID == "" {
		return "none"
	}

	prefix := sessionTimestampPrefix(session.UpdatedAt, time.Now())
	if session.Title == "" {
		return prefix + session.ID
	}

	return prefix + session.Title + " (" + session.ID + ")"
}

func stylePluginRow(line string, focused bool, selected bool) string {
	switch {
	case focused && selected:
		return cursorSelectedStyle.Render(line)
	case focused:
		return cursorStyle.Render(line)
	default:
		return defaultTextStyle.Render(line)
	}
}

func renderHelpLine() string {
	return defaultTextStyle.Render("💡 ") + helpKeyStyle.Render("↑/↓") + defaultTextStyle.Render(": navigate • ") +
		helpKeyStyle.Render("space") + defaultTextStyle.Render(": toggle • ") +
		helpKeyStyle.Render("enter") + defaultTextStyle.Render(": confirm • ") +
		helpKeyStyle.Render("s") + defaultTextStyle.Render(": sessions • ") +
		helpKeyStyle.Render("e") + defaultTextStyle.Render(": edit config... • ") +
		helpKeyStyle.Render("q") + defaultTextStyle.Render(": quit")
}

func renderEditHelpLine() string {
	return defaultTextStyle.Render("💡 ") + helpKeyStyle.Render("↑/↓") + defaultTextStyle.Render(": navigate • ") +
		helpKeyStyle.Render("enter") + defaultTextStyle.Render(": edit • ") +
		helpKeyStyle.Render("esc") + defaultTextStyle.Render(": back")
}

func renderSessionHelpLine() string {
	return defaultTextStyle.Render("💡 ") + helpKeyStyle.Render("↑/↓") + defaultTextStyle.Render(": navigate • ") +
		helpKeyStyle.Render("enter") + defaultTextStyle.Render(": select • ") +
		helpKeyStyle.Render("esc") + defaultTextStyle.Render(": back")
}

func sessionTimestampPrefix(updatedAt time.Time, now time.Time) string {
	if updatedAt.IsZero() {
		return ""
	}

	localUpdated := updatedAt.Local()
	localNow := now.Local()
	updatedYear, updatedMonth, updatedDay := localUpdated.Date()
	nowYear, nowMonth, nowDay := localNow.Date()

	if updatedYear == nowYear && updatedMonth == nowMonth && updatedDay == nowDay {
		elapsed := localNow.Sub(localUpdated)
		if elapsed < 0 {
			elapsed = 0
		}

		switch {
		case elapsed < time.Minute:
			return "[just now] "
		case elapsed < time.Hour:
			return "[" + strconv.Itoa(int(elapsed/time.Minute)) + "m ago] "
		default:
			return "[" + strconv.Itoa(int(elapsed/time.Hour)) + "h ago] "
		}
	}

	return "[" + localUpdated.Format("2006-01-02 15:04:05") + "] "
}

func sessionLine(session SessionItem) string {
	if session.ID == "" {
		return "Start without session"
	}

	prefix := sessionTimestampPrefix(session.UpdatedAt, time.Now())

	if session.Title == "" {
		return prefix + session.ID
	}

	return prefix + session.Title + " (" + session.ID + ")"
}

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
	if rows < 0 {
		return 0
	}

	return rows
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
	if start < 0 {
		start = 0
	}
	maxOffset := totalRows - visibleRows
	if start > maxOffset {
		start = maxOffset
	}

	end := start + visibleRows
	if end > totalRows {
		end = totalRows
	}

	return start, end
}

// NewModel creates a new TUI model with the given plugin items
func NewModel(items []PluginItem, editChoices []EditChoice, sessions []SessionItem, session SessionItem, version string, allowMultiplePlugins bool) Model {
	selected := make(map[int]struct{})
	for i, item := range items {
		if item.InitiallyEnabled {
			if !allowMultiplePlugins && len(selected) > 0 {
				continue
			}
			selected[i] = struct{}{}
		}
	}

	// Empty list: auto-confirm immediately
	confirmed := len(items) == 0

	sessionCursor := 0
	for i, item := range sessions {
		if item.ID == session.ID {
			sessionCursor = i + 1
			break
		}
	}

	return Model{
		plugins:              items,
		editChoices:          editChoices,
		version:              version,
		allowMultiplePlugins: allowMultiplePlugins,
		sessions:             append([]SessionItem(nil), sessions...),
		session:              session,
		cursor:               0,
		editCursor:           0,
		sessionCursor:        sessionCursor,
		selected:             selected,
		confirmed:            confirmed,
	}
}

// Init initializes the model (no initial command needed)
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles state transitions based on messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.ensureSessionCursorVisible()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.sessionMode {
				if m.sessionCursor > 0 {
					m.sessionCursor--
				}
				m.ensureSessionCursorVisible()
			} else if m.editMode {
				if m.editCursor > 0 {
					m.editCursor--
				}
			} else if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.sessionMode {
				if m.sessionCursor < len(m.sessions) {
					m.sessionCursor++
				}
				m.ensureSessionCursorVisible()
			} else if m.editMode {
				if m.editCursor < len(m.editChoices)-1 {
					m.editCursor++
				}
			} else if m.cursor < len(m.plugins)-1 {
				m.cursor++
			}
		case " ", "space": // Space key toggles selection (v2 uses "space")
			if !m.editMode && !m.sessionMode {
				if _, ok := m.selected[m.cursor]; ok {
					delete(m.selected, m.cursor)
				} else {
					if !m.allowMultiplePlugins {
						m.selected = map[int]struct{}{}
					}
					m.selected[m.cursor] = struct{}{}
				}
			}
		case "enter":
			if m.sessionMode {
				m.session = m.sessionAt(m.sessionCursor)
				m.sessionMode = false
				return m, nil
			}
			if m.editMode {
				m.edit = true
				m.editTarget = m.editChoices[m.editCursor].Path
				return m, tea.Quit
			}
			m.confirmed = true
			return m, tea.Quit
		case "s":
			m.sessionMode = true
			for i, item := range m.sessions {
				if item.ID == m.session.ID {
					m.sessionCursor = i + 1
					m.ensureSessionCursorVisible()
					return m, nil
				}
			}
			m.sessionCursor = 0
			m.ensureSessionCursorVisible()
		case "e":
			if !m.sessionMode && len(m.editChoices) > 0 {
				m.editMode = true
				m.editCursor = 0
			}
		case "ctrl+c", "q", "esc":
			if m.sessionMode && (msg.String() == "q" || msg.String() == "esc") {
				m.sessionMode = false
				return m, nil
			}
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
	if m.confirmed || m.cancelled || m.edit {
		return tea.NewView("")
	}

	if len(m.plugins) == 0 {
		return tea.NewView("")
	}

	if m.editMode {
		s := m.renderTopBadge() + "\n\n" + instructionTextStyle.Render("📂 Choose config to edit") + "\n\n"

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

	if m.sessionMode {
		s := m.renderTopBadge() + "\n\n" + instructionTextStyle.Render("🕘 Choose session") + "\n\n"
		start, end := m.visibleSessionRange()

		for i := start; i < end; i++ {
			cursor := "  "
			focused := m.sessionCursor == i
			if focused {
				cursor = "> "
			}

			line := fmt.Sprintf("%s%s", cursor, sessionLine(m.sessionAt(i)))
			line = stylePluginRow(line, focused, m.sessionAt(i).ID == m.session.ID)
			s += line + "\n"
		}

		s += "\n" + renderSessionHelpLine()

		return tea.NewView(s)
	}

	parts := []string{m.renderTopBadge(), instructionTextStyle.Render("📋 Choose plugins to enable")}
	s := strings.Join(parts, "\n\n") + "\n\n"

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

func (m Model) SelectedSession() SessionItem {
	return m.session
}
