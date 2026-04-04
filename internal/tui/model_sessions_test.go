package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func TestUpdate_SessionPickerSelectsSession(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_latest", Title: "Latest session"}, {ID: "ses_older", Title: "Older session"}}
	m := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, sessions, SessionItem{}, true)

	newModel, cmd := m.Update(mockKeyMsg("s"))
	m = newModel.(Model)
	if !m.sessionMode {
		t.Fatal("expected session mode after s")
	}
	if cmd != nil {
		t.Fatal("expected no command when opening session picker")
	}

	newModel, _ = m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	newModel, cmd = m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)

	if m.sessionMode {
		t.Fatal("expected session mode to close after selecting a session")
	}
	if got := m.SelectedSession(); got.ID != "ses_latest" {
		t.Fatalf("expected latest session to be selected, got %+v", got)
	}
	if cmd != nil {
		t.Fatal("expected no quit command after selecting session")
	}
}

func TestUpdate_SessionPickerClearsSession(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_latest", Title: "Latest session"}}
	m := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, sessions, sessions[0], true)

	newModel, _ := m.Update(mockKeyMsg("s"))
	m = newModel.(Model)
	newModel, _ = m.Update(mockKeyMsg("up"))
	m = newModel.(Model)
	newModel, cmd := m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)

	if got := m.SelectedSession(); got.ID != "" {
		t.Fatalf("expected session to be cleared, got %+v", got)
	}
	if cmd != nil {
		t.Fatal("expected no quit command when clearing session")
	}
}

func TestUpdate_SessionPickerEscReturnsToPluginList(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_latest", Title: "Latest session"}}
	m := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, sessions, sessions[0], true)

	newModel, _ := m.Update(mockKeyMsg("s"))
	m = newModel.(Model)
	newModel, cmd := m.Update(mockKeyMsg("esc"))
	m = newModel.(Model)

	if m.sessionMode {
		t.Fatal("expected session mode to close on esc")
	}
	if got := m.SelectedSession(); got.ID != "ses_latest" {
		t.Fatalf("expected selected session to remain unchanged, got %+v", got)
	}
	if cmd != nil {
		t.Fatal("expected no quit command when closing session picker")
	}
}

func TestView_SessionModeBoundsRowsToWindowHeight(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_1", Title: "Session 1"}, {ID: "ses_2", Title: "Session 2"}, {ID: "ses_3", Title: "Session 3"}}
	m := openSessionPickerWithHeight(t, sessions, SessionItem{}, 8)
	view := m.View().Content

	if !strings.Contains(view, "Start without session") {
		t.Fatal("expected start-without-session row to remain visible at top")
	}
	if !strings.Contains(view, "Session 1") {
		t.Fatal("expected first session row to be visible in bounded view")
	}
	if strings.Contains(view, "Session 2") || strings.Contains(view, "Session 3") {
		t.Fatal("expected rows beyond visible window to be hidden")
	}
}

func TestUpdate_SessionPickerScrollsToKeepFocusedRowVisible(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_1", Title: "Session 1"}, {ID: "ses_2", Title: "Session 2"}, {ID: "ses_3", Title: "Session 3"}, {ID: "ses_4", Title: "Session 4"}, {ID: "ses_5", Title: "Session 5"}}
	m := openSessionPickerWithHeight(t, sessions, SessionItem{}, 8)

	for range 5 {
		updatedModel, _ := m.Update(mockKeyMsg("down"))
		m = updatedModel.(Model)
	}

	view := m.View().Content
	if !strings.Contains(view, "Session 4") || !strings.Contains(view, "Session 5") {
		t.Fatalf("expected bottom window to include focused rows, got %q", view)
	}
	if strings.Contains(view, "Start without session") || strings.Contains(view, "Session 1") {
		t.Fatalf("expected top rows to scroll out of view, got %q", view)
	}
	if m.sessionCursor != 5 {
		t.Fatalf("expected cursor to land on final row, got %d", m.sessionCursor)
	}
}

func TestUpdate_SessionPickerPageNavigationKeys(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_1", Title: "Session 1"}, {ID: "ses_2", Title: "Session 2"}, {ID: "ses_3", Title: "Session 3"}, {ID: "ses_4", Title: "Session 4"}, {ID: "ses_5", Title: "Session 5"}, {ID: "ses_6", Title: "Session 6"}}
	m := openSessionPickerWithHeight(t, sessions, SessionItem{}, 8)

	updatedModel, _ := m.Update(mockKeyMsg("pgdown"))
	m = updatedModel.(Model)
	if m.sessionCursor != 2 {
		t.Fatalf("expected pgdown to move one visible page, got cursor %d", m.sessionCursor)
	}

	updatedModel, _ = m.Update(mockKeyMsg("ctrl+d"))
	m = updatedModel.(Model)
	if m.sessionCursor != 3 {
		t.Fatalf("expected ctrl+d to move half page, got cursor %d", m.sessionCursor)
	}

	updatedModel, _ = m.Update(mockKeyMsg("end"))
	m = updatedModel.(Model)
	if m.sessionCursor != len(sessions) {
		t.Fatalf("expected end to jump to final row, got cursor %d", m.sessionCursor)
	}
	if m.sessionOffset == 0 {
		t.Fatalf("expected end to move viewport near bottom, got offset %d", m.sessionOffset)
	}

	updatedModel, _ = m.Update(mockKeyMsg("home"))
	m = updatedModel.(Model)
	if m.sessionCursor != 0 || m.sessionOffset != 0 {
		t.Fatalf("expected home to reset cursor and viewport, got cursor=%d offset=%d", m.sessionCursor, m.sessionOffset)
	}

	updatedModel, _ = m.Update(mockKeyMsg("pgup"))
	m = updatedModel.(Model)
	if m.sessionCursor != 0 {
		t.Fatalf("expected pgup at top to stay clamped, got cursor %d", m.sessionCursor)
	}
	updatedModel, _ = m.Update(mockKeyMsg("ctrl+u"))
	m = updatedModel.(Model)
	if m.sessionCursor != 0 {
		t.Fatalf("expected ctrl+u at top to stay clamped, got cursor %d", m.sessionCursor)
	}
}

func TestUpdate_SessionPickerWindowedViewStillAllowsClearingSession(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_latest", Title: "Latest session"}}
	m := openSessionPickerWithHeight(t, sessions, sessions[0], 8)

	updatedModel, _ := m.Update(mockKeyMsg("up"))
	m = updatedModel.(Model)
	updatedModel, cmd := m.Update(mockKeyMsg("enter"))
	m = updatedModel.(Model)

	if got := m.SelectedSession(); got.ID != "" {
		t.Fatalf("expected session to be cleared, got %+v", got)
	}
	if cmd != nil {
		t.Fatal("expected no quit command when clearing session in bounded view")
	}
}

func TestView_SessionModeTinyWindowHidesSessionRows(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_1", Title: "Session 1"}, {ID: "ses_2", Title: "Session 2"}}
	m := openSessionPickerWithHeight(t, sessions, SessionItem{}, 6)
	view := m.View().Content

	if strings.Contains(view, "Start without session") || strings.Contains(view, "Session 1") || strings.Contains(view, "Session 2") {
		t.Fatalf("expected tiny window to hide session rows, got %q", view)
	}
	if !strings.Contains(view, "🕘 Choose session") || !strings.Contains(view, "esc") {
		t.Fatalf("expected tiny window to keep session chrome, got %q", view)
	}
}

func TestUpdate_SessionPickerResizeKeepsFocusedRowVisible(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_1", Title: "Session 1"}, {ID: "ses_2", Title: "Session 2"}, {ID: "ses_3", Title: "Session 3"}, {ID: "ses_4", Title: "Session 4"}, {ID: "ses_5", Title: "Session 5"}}
	m := openSessionPickerWithHeight(t, sessions, SessionItem{}, 10)

	for range 5 {
		updatedModel, _ := m.Update(mockKeyMsg("down"))
		m = updatedModel.(Model)
	}

	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 8})
	m = updatedModel.(Model)
	view := m.View().Content

	if !strings.Contains(view, "Session 5") {
		t.Fatalf("expected focused session to remain visible after resize, got %q", view)
	}
	if strings.Contains(view, "Start without session") || strings.Contains(view, "Session 1") {
		t.Fatalf("expected top rows to stay out of view after resize near bottom, got %q", view)
	}
}

func TestUpdate_SpaceDoesNotToggleInSessionMode(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a"}}
	sessions := []SessionItem{{ID: "ses_latest", Title: "Latest session"}}
	m := newTestModelWithSession(items, nil, sessions, SessionItem{}, true)

	newModel, _ := m.Update(mockKeyMsg("s"))
	m = newModel.(Model)
	if !m.sessionMode {
		t.Fatal("expected session mode after s")
	}

	newModel, _ = m.Update(mockKeyMsg("space"))
	m = newModel.(Model)
	if m.Selections()["plugin-a"] {
		t.Fatal("expected plugin selection to remain unchanged while session picker is open")
	}
}

func TestView_SessionModeRendersInstructionPrompt(t *testing.T) {
	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, []SessionItem{{ID: "ses_latest", Title: "Latest session"}}, SessionItem{}, true)
	updatedModel, _ := model.Update(mockKeyMsg("s"))
	view := updatedModel.(Model).View().Content

	expected := renderSectionHeader("🕘 Choose session", maxLayoutWidth)
	if !strings.Contains(view, expected) {
		t.Fatalf("expected session prompt line %q in %q", expected, view)
	}
}

func TestViewSessionPicker_MatchesSessionModeView(t *testing.T) {
	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, []SessionItem{{ID: "ses_latest", Title: "Latest session"}}, SessionItem{}, true)
	updatedModel, _ := model.Update(mockKeyMsg("s"))
	m := updatedModel.(Model)

	if got, want := m.viewSessionPicker().Content, m.View().Content; got != want {
		t.Fatalf("expected session helper to match session mode view\nhelper: %q\nview:   %q", got, want)
	}
}

func TestSessionTimestampPrefix_ReturnsHoursAgoForToday(t *testing.T) {
	now := time.Date(2026, time.March, 23, 14, 30, 0, 0, time.Local)
	updatedAt := time.Date(2026, time.March, 23, 9, 8, 7, 0, time.Local)

	if got := sessionTimestampPrefix(updatedAt, now); got != "[5h ago] " {
		t.Fatalf("expected relative prefix for today's older session, got %q", got)
	}
}

func TestSessionTimestampPrefix_ReturnsMinutesAgoForToday(t *testing.T) {
	now := time.Date(2026, time.March, 23, 14, 30, 0, 0, time.Local)
	updatedAt := time.Date(2026, time.March, 23, 14, 15, 0, 0, time.Local)

	if got := sessionTimestampPrefix(updatedAt, now); got != "[15m ago] " {
		t.Fatalf("expected relative minutes prefix for today's session, got %q", got)
	}
}

func TestSessionTimestampPrefix_ReturnsJustNowForRecentSession(t *testing.T) {
	now := time.Date(2026, time.March, 23, 14, 30, 30, 0, time.Local)
	updatedAt := time.Date(2026, time.March, 23, 14, 30, 0, 0, time.Local)

	if got := sessionTimestampPrefix(updatedAt, now); got != "[just now] " {
		t.Fatalf("expected just-now prefix for recent session, got %q", got)
	}
}

func TestSessionTimestampPrefix_ReturnsFullDateTimeForOlderSession(t *testing.T) {
	now := time.Date(2026, time.March, 23, 14, 30, 0, 0, time.Local)
	updatedAt := time.Date(2026, time.March, 22, 9, 8, 7, 0, time.Local)

	if got := sessionTimestampPrefix(updatedAt, now); got != "[2026-03-22 09:08] " {
		t.Fatalf("expected full datetime prefix for older session, got %q", got)
	}
}

func TestSessionTimestampPrefix_ZeroTimeReturnsEmptyString(t *testing.T) {
	if got := sessionTimestampPrefix(time.Time{}, time.Now()); got != "" {
		t.Fatalf("expected empty prefix for zero time, got %q", got)
	}
}

func TestView_SessionModeRendersUnboxedSessionRow(t *testing.T) {
	now := time.Date(2026, time.March, 23, 9, 8, 7, 0, time.Local)
	session := SessionItem{ID: "ses_latest", Title: "Latest session", UpdatedAt: now}
	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, []SessionItem{session}, session, true)
	updatedModel, _ := model.Update(mockKeyMsg("s"))
	view := updatedModel.(Model).View().Content
	rowLine := strings.Split(view, "\n")[5]
	expected := stylePluginRow("> "+selectedSessionSummary(session, maxLayoutWidth), true, true)

	if rowLine != expected {
		t.Fatalf("expected unboxed session row %q, got %q", expected, rowLine)
	}
}

func TestSelectedSessionSummary_TruncatesTitleButPreservesFullID(t *testing.T) {
	session := SessionItem{ID: "ses_abcdefghijklmnopqrstuvwxyz1234567890", Title: "This is a very long session title that should be truncated", UpdatedAt: time.Now().Add(-10 * time.Minute)}
	summary := selectedSessionSummary(session, 80)
	if !strings.Contains(summary, "("+session.ID+")") {
		t.Fatalf("expected full session ID to be preserved, got %q", summary)
	}
	if !strings.Contains(summary, "...") {
		t.Fatalf("expected truncated title with ellipsis, got %q", summary)
	}
}

func TestSelectedSessionSummary_ShortensPathLikeTitlesInMiddle(t *testing.T) {
	session := SessionItem{ID: "ses_123", Title: "/Users/kayden/workspace/super-long-project-name", UpdatedAt: time.Now()}
	summary := selectedSessionSummary(session, 42)
	for _, snippet := range []string{"/Users", "..", "name", "(ses_123)"} {
		if !strings.Contains(summary, snippet) {
			t.Fatalf("expected %q in %q", snippet, summary)
		}
	}
	if lipgloss.Width(summary) > 42 {
		t.Fatalf("expected width <= 42, got %d in %q", lipgloss.Width(summary), summary)
	}
}

func TestRenderSessionHelpLine_IncludesScrollNavigationTokens(t *testing.T) {
	helpLine := renderSessionHelpLine(maxLayoutWidth)
	for _, token := range []string{"↑/↓", "pgup/pgdn", "ctrl+u/d", "home/end", "enter", "esc"} {
		if !strings.Contains(helpLine, helpBgKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}
}
