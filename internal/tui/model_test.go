package tui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
)

const testVersion = "dev"

func newTestModel(items []PluginItem, editChoices []EditChoice, allowMultiplePlugins bool) Model {
	return NewModel(items, editChoices, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, allowMultiplePlugins)
}

func newTestModelWithSession(items []PluginItem, editChoices []EditChoice, sessions []SessionItem, session SessionItem, allowMultiplePlugins bool) Model {
	return NewModel(items, editChoices, sessions, session, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, allowMultiplePlugins)
}

func expectedTopBadge(version string, session SessionItem) string {
	targetWidth := 80
	label := sessionLabelStyle.Render("OC")
	versionText := sessionContentStyle.Render(sessionValueStyle.Render(version))
	metaWidth := max(0, targetWidth-lipgloss.Width(label)-lipgloss.Width(versionText))
	return sessionContainerStyle.Render(lipgloss.JoinHorizontal(
		lipgloss.Top,
		label,
		versionText,
		sessionMetaStyle.Width(metaWidth).Render(selectedSessionSummary(session, max(0, metaWidth-2))),
	))
}

// mockKeyMsg creates a KeyPressMsg for testing
func mockKeyMsg(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: key}
}

func openSessionPickerWithHeight(t *testing.T, sessions []SessionItem, session SessionItem, height int) Model {
	t.Helper()

	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, sessions, session, true)
	updatedModel, _ := model.Update(mockKeyMsg("s"))
	model = updatedModel.(Model)
	updatedModel, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: height})

	return updatedModel.(Model)
}

func TestNewModel_InitialState(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: true},
		{Name: "plugin-b", InitiallyEnabled: false},
		{Name: "plugin-c", InitiallyEnabled: true},
	}

	m := newTestModel(items, nil, true)

	if m.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", m.cursor)
	}

	selections := m.Selections()
	if !selections["plugin-a"] {
		t.Error("expected plugin-a to be selected (initially enabled)")
	}
	if selections["plugin-b"] {
		t.Error("expected plugin-b to be unselected (initially disabled)")
	}
	if !selections["plugin-c"] {
		t.Error("expected plugin-c to be selected (initially enabled)")
	}

	if m.cancelled {
		t.Error("expected cancelled=false initially")
	}
	if m.confirmed {
		t.Error("expected confirmed=false initially")
	}
	if got := m.SelectedSession(); got.ID != "" {
		t.Fatalf("expected empty selected session initially, got %+v", got)
	}
}

func TestNewModel_EmptyList(t *testing.T) {
	m := newTestModel([]PluginItem{}, nil, true)

	if !m.confirmed {
		t.Error("expected confirmed=true for empty list")
	}
	if m.cancelled {
		t.Error("expected cancelled=false for empty list")
	}
}

func TestNewModel_SingleSelectKeepsOnlyFirstInitiallyEnabledPlugin(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: true},
		{Name: "plugin-b", InitiallyEnabled: true},
		{Name: "plugin-c", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, false)
	selections := m.Selections()

	if !selections["plugin-a"] {
		t.Fatal("expected first initially enabled plugin to stay selected")
	}
	if selections["plugin-b"] {
		t.Fatal("expected second initially enabled plugin to be dropped in single-select mode")
	}
	if selections["plugin-c"] {
		t.Fatal("expected disabled plugin to remain unselected")
	}
}

func TestUpdate_ArrowDown(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
		{Name: "plugin-b", InitiallyEnabled: false},
		{Name: "plugin-c", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)

	// Move down from 0 to 1
	newModel, _ := m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1, got %d", m.cursor)
	}

	// Move down from 1 to 2
	newModel, _ = m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	if m.cursor != 2 {
		t.Errorf("expected cursor=2, got %d", m.cursor)
	}
}

func TestUpdate_ArrowDownBoundary(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
		{Name: "plugin-b", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)
	m.cursor = 1 // Last item

	// Try to move down past last item
	newModel, _ := m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor to stay at 1 (boundary), got %d", m.cursor)
	}
}

func TestUpdate_ArrowUp(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
		{Name: "plugin-b", InitiallyEnabled: false},
		{Name: "plugin-c", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)
	m.cursor = 2

	// Move up from 2 to 1
	newModel, _ := m.Update(mockKeyMsg("up"))
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1, got %d", m.cursor)
	}

	// Move up from 1 to 0
	newModel, _ = m.Update(mockKeyMsg("up"))
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", m.cursor)
	}
}

func TestUpdate_ArrowUpBoundary(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
		{Name: "plugin-b", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)
	// cursor already at 0

	// Try to move up past first item
	newModel, _ := m.Update(mockKeyMsg("up"))
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0 (boundary), got %d", m.cursor)
	}
}

func TestUpdate_VimBindings(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
		{Name: "plugin-b", InitiallyEnabled: false},
		{Name: "plugin-c", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)

	// j moves down
	newModel, _ := m.Update(mockKeyMsg("j"))
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after 'j', got %d", m.cursor)
	}

	// k moves up
	newModel, _ = m.Update(mockKeyMsg("k"))
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after 'k', got %d", m.cursor)
	}
}

func TestUpdate_SpaceToggle(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
		{Name: "plugin-b", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)

	// plugin-a starts unselected
	selections := m.Selections()
	if selections["plugin-a"] {
		t.Error("expected plugin-a to be unselected initially")
	}

	// Press space at cursor=0 to select plugin-a
	newModel, _ := m.Update(mockKeyMsg("space"))
	m = newModel.(Model)
	selections = m.Selections()
	if !selections["plugin-a"] {
		t.Error("expected plugin-a to be selected after space")
	}

	// Press space again to unselect
	newModel, _ = m.Update(mockKeyMsg("space"))
	m = newModel.(Model)
	selections = m.Selections()
	if selections["plugin-a"] {
		t.Error("expected plugin-a to be unselected after second space")
	}
}

func TestUpdate_SpaceToggleSingleSelectMode(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: true},
		{Name: "plugin-b", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, false)
	newModel, _ := m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	newModel, _ = m.Update(mockKeyMsg("space"))
	m = newModel.(Model)

	selections := m.Selections()
	if selections["plugin-a"] {
		t.Error("expected plugin-a to be deselected in single-select mode")
	}
	if !selections["plugin-b"] {
		t.Error("expected plugin-b to be selected in single-select mode")
	}
}

func TestUpdate_SpaceToggleMultiSelectMode(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: true},
		{Name: "plugin-b", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)
	newModel, _ := m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	newModel, _ = m.Update(mockKeyMsg("space"))
	m = newModel.(Model)

	selections := m.Selections()
	if !selections["plugin-a"] || !selections["plugin-b"] {
		t.Fatalf("expected both plugins selected in multi-select mode, got %+v", selections)
	}
}

func TestUpdate_EnterConfirm(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)

	// Press enter to confirm
	newModel, cmd := m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)

	if !m.confirmed {
		t.Error("expected confirmed=true after enter")
	}
	if m.cancelled {
		t.Error("expected cancelled=false after enter")
	}
	if cmd == nil || cmd() != tea.Quit() {
		t.Error("expected tea.Quit command after enter")
	}
}

func TestUpdate_CtrlCCancel(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)

	// Press ctrl+c to cancel
	newModel, cmd := m.Update(mockKeyMsg("ctrl+c"))
	m = newModel.(Model)

	if !m.cancelled {
		t.Error("expected cancelled=true after ctrl+c")
	}
	if m.confirmed {
		t.Error("expected confirmed=false after ctrl+c")
	}
	if cmd == nil || cmd() != tea.Quit() {
		t.Error("expected tea.Quit command after ctrl+c")
	}
}

func TestUpdate_QuitKeys(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
	}

	tests := []struct {
		key  string
		name string
	}{
		{"q", "q key"},
		{"esc", "esc key"},
	}

	for _, tt := range tests {
		m := newTestModel(items, nil, true)
		newModel, cmd := m.Update(mockKeyMsg(tt.key))
		m = newModel.(Model)

		if !m.cancelled {
			t.Errorf("%s: expected cancelled=true", tt.name)
		}
		if m.confirmed {
			t.Errorf("%s: expected confirmed=false", tt.name)
		}
		if cmd == nil || cmd() != tea.Quit() {
			t.Errorf("%s: expected tea.Quit command", tt.name)
		}
	}
}

func TestUpdate_EditRequest(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
	}
	editChoices := []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}

	m := newTestModel(items, editChoices, true)

	newModel, cmd := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)

	if m.EditRequested() {
		t.Error("expected EditRequested()=false until edit target is selected")
	}
	if !m.editMode {
		t.Error("expected edit mode after e")
	}
	if m.cancelled {
		t.Error("expected cancelled=false after e")
	}
	if m.confirmed {
		t.Error("expected confirmed=false after e")
	}
	if cmd != nil {
		t.Error("expected no quit command after entering edit mode")
	}
}

func TestUpdate_EditModeEnterSelectsTarget(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}
	editChoices := []EditChoice{
		{Label: ".oc file", Path: "/tmp/.oc"},
		{Label: "opencode.json", Path: "/tmp/opencode.json"},
	}

	m := newTestModel(items, editChoices, true)
	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	newModel, _ = m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	newModel, cmd := m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)

	if !m.EditRequested() {
		t.Fatal("expected EditRequested()=true after selecting an edit target")
	}
	if got := m.EditTarget(); got != "/tmp/opencode.json" {
		t.Fatalf("expected edit target /tmp/opencode.json, got %q", got)
	}
	if cmd == nil || cmd() != tea.Quit() {
		t.Error("expected tea.Quit command after edit target selection")
	}
}

func TestUpdate_EditModeEscReturnsToPluginList(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}
	editChoices := []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}

	m := newTestModel(items, editChoices, true)
	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	newModel, cmd := m.Update(mockKeyMsg("esc"))
	m = newModel.(Model)

	if m.editMode {
		t.Error("expected edit mode to close on esc")
	}
	if m.Cancelled() {
		t.Error("expected model not to be cancelled when backing out of edit mode")
	}
	if cmd != nil {
		t.Error("expected no quit command when backing out of edit mode")
	}
}

func TestUpdate_SessionPickerSelectsSession(t *testing.T) {
	sessions := []SessionItem{{ID: "ses_latest", Title: "Latest session"}, {ID: "ses_older", Title: "Older session"}}
	m := newTestModelWithSession([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, sessions, SessionItem{}, true)

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
	m := newTestModelWithSession([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, sessions, sessions[0], true)

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
	m := newTestModelWithSession([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, sessions, sessions[0], true)

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
	sessions := []SessionItem{
		{ID: "ses_1", Title: "Session 1"},
		{ID: "ses_2", Title: "Session 2"},
		{ID: "ses_3", Title: "Session 3"},
	}
	m := openSessionPickerWithHeight(t, sessions, SessionItem{}, 8)
	view := m.View().Content

	if !strings.Contains(view, "Start without session") {
		t.Fatal("expected start-without-session row to remain visible at top")
	}
	if !strings.Contains(view, "Session 1") {
		t.Fatal("expected first session row to be visible in bounded view")
	}
	if strings.Contains(view, "Session 2") {
		t.Fatal("expected rows beyond visible window to be hidden")
	}
	if strings.Contains(view, "Session 3") {
		t.Fatal("expected rows beyond visible window to be hidden")
	}
}

func TestUpdate_SessionPickerScrollsToKeepFocusedRowVisible(t *testing.T) {
	sessions := []SessionItem{
		{ID: "ses_1", Title: "Session 1"},
		{ID: "ses_2", Title: "Session 2"},
		{ID: "ses_3", Title: "Session 3"},
		{ID: "ses_4", Title: "Session 4"},
		{ID: "ses_5", Title: "Session 5"},
	}
	m := openSessionPickerWithHeight(t, sessions, SessionItem{}, 8)

	for i := 0; i < 5; i++ {
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
	sessions := []SessionItem{
		{ID: "ses_1", Title: "Session 1"},
		{ID: "ses_2", Title: "Session 2"},
	}
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
	sessions := []SessionItem{
		{ID: "ses_1", Title: "Session 1"},
		{ID: "ses_2", Title: "Session 2"},
		{ID: "ses_3", Title: "Session 3"},
		{ID: "ses_4", Title: "Session 4"},
		{ID: "ses_5", Title: "Session 5"},
	}
	m := openSessionPickerWithHeight(t, sessions, SessionItem{}, 10)

	for i := 0; i < 5; i++ {
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
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}
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

func TestSelections_Output(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: true},
		{Name: "plugin-b", InitiallyEnabled: false},
		{Name: "plugin-c", InitiallyEnabled: true},
	}

	m := newTestModel(items, nil, true)

	// Verify initial selections
	selections := m.Selections()
	if len(selections) != 3 {
		t.Errorf("expected 3 entries in selections, got %d", len(selections))
	}
	if !selections["plugin-a"] {
		t.Error("expected plugin-a=true")
	}
	if selections["plugin-b"] {
		t.Error("expected plugin-b=false")
	}
	if !selections["plugin-c"] {
		t.Error("expected plugin-c=true")
	}

	// Toggle plugin-b (cursor at 0, move to 1, toggle)
	newModel, _ := m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	newModel, _ = m.Update(mockKeyMsg("space"))
	m = newModel.(Model)

	selections = m.Selections()
	if !selections["plugin-b"] {
		t.Error("expected plugin-b=true after toggle")
	}
}

func TestCancelled_Method(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
	}

	m := newTestModel(items, nil, true)
	if m.Cancelled() {
		t.Error("expected Cancelled()=false initially")
	}

	newModel, _ := m.Update(mockKeyMsg("ctrl+c"))
	m = newModel.(Model)
	if !m.Cancelled() {
		t.Error("expected Cancelled()=true after ctrl+c")
	}
}

func TestEditRequested_Method(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: false},
	}
	editChoices := []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}

	m := newTestModel(items, editChoices, true)
	if m.EditRequested() {
		t.Error("expected EditRequested()=false initially")
	}

	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	if m.EditRequested() {
		t.Error("expected EditRequested()=false while only the edit picker is open")
	}

	newModel, _ = m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)
	if !m.EditRequested() {
		t.Error("expected EditRequested()=true after choosing an edit target")
	}
	if got := m.EditTarget(); got != "/tmp/.oc" {
		t.Fatalf("expected EditTarget()=/tmp/.oc, got %q", got)
	}
}

func TestRenderTopBadge_ContainsBrandAndVersion(t *testing.T) {
	rendered := Model{version: testVersion}.renderTopBadge()
	expected := expectedTopBadge(testVersion, SessionItem{})

	if rendered != expected {
		t.Fatalf("expected top badge %q, got %q", expected, rendered)
	}
}

func TestRenderTopBadge_IncludesSelectedSessionInfoWithMetaBackground(t *testing.T) {
	session := SessionItem{ID: "ses_latest", Title: "Latest session", UpdatedAt: time.Now()}
	rendered := Model{version: testVersion, session: session}.renderTopBadge()
	expected := expectedTopBadge(testVersion, session)

	if rendered != expected {
		t.Fatalf("expected top badge %q, got %q", expected, rendered)
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

func TestStylePluginRow_UsesCombinedStyleForFocusedSelectedRow(t *testing.T) {
	cursorSelectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F0F0F0")).Bold(true)
	rowLine := stylePluginRow("> ✔  plugin-a", true, true)
	expected := cursorSelectedStyle.Render("> ✔  plugin-a")

	if !strings.Contains(rowLine, expected) {
		t.Fatalf("expected focused+selected style %q in %q", expected, rowLine)
	}
}

func TestRenderHelpLine_IncludesStyledKeyTokens(t *testing.T) {
	helpLine := renderHelpLine()

	for _, token := range []string{"↑/↓", "space", "enter", "s", "c", "q"} {
		if !strings.Contains(helpLine, helpBgKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}

	for _, action := range []string{"navigate", "toggle", "confirm", "sessions", "config", "quit"} {
		if !strings.Contains(helpLine, action) {
			t.Fatalf("expected plain help action %q in %q", action, helpLine)
		}
		if strings.Contains(helpLine, helpBgKeyStyle.Render(action)) {
			t.Fatalf("expected action %q to remain unstyled in %q", action, helpLine)
		}
	}
	if !strings.Contains(helpLine, helpBgTextStyle.Render(": quit")) {
		t.Fatalf("expected default text color on help copy, got %q", helpLine)
	}
}

func TestView_RendersStyledHeaderLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	headerLine := strings.Split(view, "\n")[0]

	expected := Model{version: testVersion}.renderTopBadge()
	if headerLine != expected {
		t.Fatalf("expected top badge %q, got %q", expected, headerLine)
	}
}

func TestView_RendersPluginSelectionPrompt(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content

	expected := renderSectionHeader("📋 Choose plugins")
	if !strings.Contains(view, expected) {
		t.Fatalf("expected plugin prompt line %q in %q", expected, view)
	}
}

func TestView_EditModeRendersInstructionPrompt(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	view := updatedModel.(Model).View().Content

	expected := renderSectionHeader("📂 Choose config to edit")
	if !strings.Contains(view, expected) {
		t.Fatalf("expected edit prompt line %q in %q", expected, view)
	}
}

func TestView_SessionModeRendersInstructionPrompt(t *testing.T) {
	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, []SessionItem{{ID: "ses_latest", Title: "Latest session"}}, SessionItem{}, true)
	updatedModel, _ := model.Update(mockKeyMsg("s"))
	view := updatedModel.(Model).View().Content

	expected := renderSectionHeader("🕘 Choose session")
	if !strings.Contains(view, expected) {
		t.Fatalf("expected session prompt line %q in %q", expected, view)
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

	if got := sessionTimestampPrefix(updatedAt, now); got != "[2026-03-22 09:08:07] " {
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
	expected := stylePluginRow("> "+sessionLine(session), true, true)

	if rowLine != expected {
		t.Fatalf("expected unboxed session row %q, got %q", expected, rowLine)
	}
}

func TestView_RendersFocusedSelectedRowLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, true).View().Content
	expected := stylePluginRow("> ✔  plugin-a", true, true)

	if !strings.Contains(view, expected) {
		t.Fatalf("expected row line %q in %q", expected, view)
	}
}

func TestView_EditModeRendersStyledHeaderLine(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	view := updatedModel.(Model).View().Content
	headerLine := strings.Split(view, "\n")[0]

	expected := Model{version: testVersion}.renderTopBadge()
	if headerLine != expected {
		t.Fatalf("expected edit-mode top badge %q, got %q", expected, headerLine)
	}
}

func TestView_RendersStyledHelpLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content

	if !strings.Contains(view, renderHelpLine()) {
		t.Fatalf("expected help line %q in %q", renderHelpLine(), view)
	}
}

func TestView_RendersRhythmAndMetricsSections(t *testing.T) {
	report := stats.Report{CurrentStreak: 6, AgentDays: 17, TodayCost: 1.84, YesterdayCost: 1.50, TodayTokens: 148000, YesterdayTokens: 170000, ThirtyDayCost: 7.42, ThirtyDayTokens: 420000, TodaySessionMinutes: 95, YesterdaySessionMinutes: 120, ThirtyDaySessionMinutes: 765, TodayCodeLines: 150, YesterdayCodeLines: 190, ThirtyDayCodeLines: 1820, WeeklyActiveDays: 4, HighestBurnDay: stats.Day{Cost: 12.34}, HighestCodeDay: stats.Day{CodeLines: 190}, CoachingNote: stats.DefaultCoachingNote(), Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 1000, Cost: 0.5, SessionMinutes: 10, CodeLines: 20}
	}
	report.Days[len(report.Days)-1].Tokens = 160000
	report.Days[len(report.Days)-1].Cost = 1.84
	report.Days[len(report.Days)-1].SessionMinutes = 95
	report.Days[len(report.Days)-1].CodeLines = 150
	report.Days[len(report.Days)-2].Cost = 12.34
	report.Days[len(report.Days)-2].SessionMinutes = 120
	report.Days[len(report.Days)-2].CodeLines = 190
	report.HighestBurnDay = report.Days[len(report.Days)-2]
	report.HighestCodeDay = report.Days[len(report.Days)-2]
	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	view := model.View().Content

	if !strings.Contains(view, "Rhythm") {
		t.Fatalf("expected Rhythm section, got %q", view)
	}
	if strings.Contains(view, report.Days[0].Date.Format("2006-01-02")+"~") {
		t.Fatalf("did not expect rhythm header date range, got %q", view)
	}
	if !strings.Contains(view, "Metrics") {
		t.Fatalf("expected Metrics section, got %q", view)
	}
	if !strings.Contains(view, defaultTextStyle.Render("• active ")+statsValueTextStyle.Render("0/30d")) {
		t.Fatalf("expected active 30d summary, got %q", view)
	}
	if !strings.Contains(view, defaultTextStyle.Render("    ")+defaultTextStyle.Render("• active ")+statsValueTextStyle.Render("0/30d")) {
		t.Fatalf("expected bulleted active summary, got %q", view)
	}
	if !strings.Contains(view, defaultTextStyle.Render("• streak ")+statsValueTextStyle.Render("6d (best)")) {
		t.Fatalf("expected streak line with parenthetical best streak, got %q", view)
	}
	if strings.Contains(view, "Today") {
		t.Fatalf("did not expect Today section after Metrics table change, got %q", view)
	}
	if strings.Contains(view, "agent 17/30d") {
		t.Fatalf("did not expect agent split metric, got %q", view)
	}
	if !strings.Contains(view, "\n\n") || !strings.Contains(view, "Metrics") {
		t.Fatalf("expected Metrics section, got %q", view)
	}
	if !strings.Contains(view, defaultTextStyle.Render("  tokens")) || !strings.Contains(view, defaultTextStyle.Render("today")) || !strings.Contains(view, defaultTextStyle.Render("peak day")) || !strings.Contains(view, defaultTextStyle.Render("30d total")) {
		t.Fatalf("expected metrics table header, got %q", view)
	}
	todayAccent := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF9900"))
	for _, row := range []struct{ label, today, peak, total string }{
		{"tokens", "148k (92%)", "160k (" + report.Days[len(report.Days)-1].Date.Format("01-02") + ")", "420k"},
		{"cost", "$1.84 (15%)", "$12.34 (" + report.HighestBurnDay.Date.Format("01-02") + ")", "$7.42"},
		{"hours", "1.6h (79%)", "2.0h (" + report.Days[len(report.Days)-2].Date.Format("01-02") + ")", "12.8h"},
		{"lines", "150 (79%)", "190 (" + report.HighestCodeDay.Date.Format("01-02") + ")", "1.8k"},
	} {
		if !strings.Contains(view, defaultTextStyle.Render("  "+row.label)) || !strings.Contains(view, todayAccent.Render(row.today)) || !strings.Contains(view, statsValueTextStyle.Render(row.peak)) || !strings.Contains(view, statsValueTextStyle.Render(row.total)) {
			t.Fatalf("expected metrics row for %s, got %q", row.label, view)
		}
	}
	if !strings.Contains(view, "(15%)") || !strings.Contains(view, "(92%)") {
		t.Fatalf("expected top-based ratios, got %q", view)
	}
	if strings.Contains(view, "This Week") {
		t.Fatalf("did not expect This Week section, got %q", view)
	}
}

func TestRenderOverviewLines_GroupsPostMetricsIntoSections(t *testing.T) {
	report := stats.Report{
		TodayCost:               1.84,
		TodayTokens:             148000,
		TodaySessionMinutes:     95,
		TodayReasoningShare:     0.25,
		RecentReasoningShare:    0.18,
		ThirtyDayCost:           7.42,
		ThirtyDayTokens:         420000,
		ThirtyDaySessionMinutes: 765,
		TotalSubtasks:           11,
		TotalToolCalls:          42,
		TotalSkillCalls:         7,
		UniqueAgentCount:        3,
		UniqueSkillCount:        2,
		UniqueToolCount:         9,
		HighestBurnDay:          stats.Day{Date: time.Now().AddDate(0, 0, -1), Cost: 12.34},
		MostEfficientDay:        stats.Day{Date: time.Now().AddDate(0, 0, -3), Cost: 0.42, Tokens: 25000},
		CoachingNote:            stats.DefaultCoachingNote(),
		Days:                    make([]stats.Day, 30),
	}
	setRankedUsageField(&report, "TopTools", []usageFixture{{"bash", 21}, {"read", 11}, {"edit", 8}, {"grep", 6}, {"write", 4}, {"glob", 2}})
	setRankedUsageField(&report, "TopSkills", []usageFixture{{"writing-plans", 5}, {"test-driven-development", 2}})
	setRankedUsageField(&report, "TopAgents", []usageFixture{{"explore", 7}, {"oracle", 3}, {"planner", 2}, {"review", 2}, {"debug", 1}, {"legacy", 1}})
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}
	report.TodayCodeLines = 150
	report.ThirtyDayCodeLines = 1820
	report.HighestCodeDay = stats.Day{Date: time.Now().AddDate(0, 0, -1), CodeLines: 190}
	report.Days[len(report.Days)-1].CodeLines = 150
	report.Days[len(report.Days)-2].CodeLines = 190
	report.WeekdayActiveCounts = [7]int{4, 4, 4, 3, 3, 3, 1}
	report.WeekdayAgentCounts = [7]int{4, 4, 4, 3, 3, 3, 1}
	report.LongestSessionDay = report.Days[len(report.Days)-1]

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")

	for _, section := range []string{"Trends", "Activity - Models (0)", "Activity - Agents (3)", "Activity - Skills (2)", "Activity - Tools (9)"} {
		if !strings.Contains(content, section) {
			t.Fatalf("expected %s section in overview, got %q", section, content)
		}
	}
	if strings.Contains(content, "Extremes") {
		t.Fatalf("expected Extremes section to be removed, got %q", content)
	}
	if strings.Contains(content, "weekday pattern     ") || strings.Contains(content, "daily cost trend    ") || strings.Contains(content, "reasoning share     ") {
		t.Fatalf("expected old flat overview labels to be removed, got %q", content)
	}
	if strings.Contains(content, renderSubSectionHeader("Activity", habitSectionTitleStyle)) {
		t.Fatalf("expected old activity header to be replaced, got %q", content)
	}
	for _, snippet := range []string{
		defaultTextStyle.Render("• calls ") + statsValueTextStyle.Render("42"),
		defaultTextStyle.Render("• delegated ") + statsValueTextStyle.Render("11"),
		defaultTextStyle.Render("• unique "),
	} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected activity summary snippet %q to be removed, got %q", snippet, content)
		}
	}
	for _, snippet := range []string{"bash", "read", "write", "explore", "oracle", "debug", "writing-plans", "test-driven-development", "Total", "100%", "(50%)", "(64%)"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected ranked activity snippet %q, got %q", snippet, content)
		}
	}
	for _, snippet := range []string{"• 1 bash ", "• 2 read ", "• 1 explore ", "• 1 writing-plans "} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected ordinal prefixes to be removed, got %q", content)
		}
	}
	for _, snippet := range []string{"• hours ", "1.6h", "150 (79%)", "today", "peak day", "30d total", "tokens", "lines", "(03-28)", "420k", "1.8k"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected hours snippet %q, got %q", snippet, content)
		}
	}
	if !strings.Contains(content, renderSubSectionHeader("Metrics", todaySectionTitleStyle)) {
		t.Fatalf("expected Metrics section in overview, got %q", content)
	}
	if strings.Contains(content, renderSubSectionHeader("Today", todaySectionTitleStyle)) {
		t.Fatalf("did not expect Today section in overview, got %q", content)
	}
	if !strings.Contains(content, defaultTextStyle.Render("  tokens")) || !strings.Contains(content, defaultTextStyle.Render("  cost")) || !strings.Contains(content, defaultTextStyle.Render("  hours")) || !strings.Contains(content, defaultTextStyle.Render("  lines")) {
		t.Fatalf("expected metrics table rows, got %q", content)
	}
	if !strings.Contains(content, defaultTextStyle.Render("• lines ")) {
		t.Fatalf("expected lines trend row, got %q", content)
	}
	for _, snippet := range []string{"• high burn ", "• longest day ", "• code peak ", "• efficient day "} {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected extremes snippet %q to be removed, got %q", snippet, content)
		}
	}
}

func TestRenderOverviewLines_KeepsTrendsAsCompactList(t *testing.T) {
	report := stats.Report{
		TodayCost:            1.84,
		TodayTokens:          148000,
		TodaySessionMinutes:  95,
		TodayCodeLines:       150,
		TodayReasoningShare:  0.25,
		RecentReasoningShare: 0.18,
		Days:                 make([]stats.Day, 30),
	}
	for i := range report.Days {
		report.Days[i] = stats.Day{
			Date:           time.Now().AddDate(0, 0, -(29 - i)),
			Tokens:         int64((i + 1) * 1000),
			Cost:           float64(i+1) / 10,
			SessionMinutes: i + 1,
			CodeLines:      i + 2,
		}
	}

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")

	trendsSection := strings.SplitN(content, renderSubSectionHeader("Activity - Models", habitSectionTitleStyle), 2)[0]
	for _, snippet := range []string{
		defaultTextStyle.Render("• tokens "),
		defaultTextStyle.Render("• cost "),
		defaultTextStyle.Render("• hours "),
		defaultTextStyle.Render("• lines "),
		defaultTextStyle.Render("• reasoning "),
	} {
		if !strings.Contains(trendsSection, snippet) {
			t.Fatalf("expected trends snippet %q, got %q", snippet, trendsSection)
		}
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• tokens ")+statsValueTextStyle.Render(" ")) {
		t.Fatalf("expected tokens trend to stay single-line, got %q", trendsSection)
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+statsValueTextStyle.Render(" ")) {
		t.Fatalf("expected cost trend to stay single-line, got %q", trendsSection)
	}
	if strings.Contains(trendsSection, renderTwoColumns("• tokens ", "", 28, "• cost ", "", 28)) || strings.Contains(trendsSection, renderTwoColumns("• hours ", "", 28, "• lines ", "", 28)) {
		t.Fatalf("expected trends to avoid two-column paired rows, got %q", trendsSection)
	}
	if strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+statsValueTextStyle.Render(renderValueTrend(report.Days, func(day stats.Day) float64 { return day.Cost }))) {
		t.Fatalf("expected cost trend label column to include fixed-width padding, got %q", trendsSection)
	}
	if !strings.Contains(trendsSection, defaultTextStyle.Render("• cost ")+defaultTextStyle.Render("   ")) {
		t.Fatalf("expected padded cost trend label column, got %q", trendsSection)
	}
}

func TestRenderOverviewLines_OrdersTrendRowsAsRequested(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{
			Date:           time.Now().AddDate(0, 0, -(29 - i)),
			Tokens:         int64((i + 1) * 1000),
			Cost:           float64(i + 1),
			SessionMinutes: i + 1,
			CodeLines:      i + 2,
		}
	}

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	trendsSection := strings.SplitN(content, renderSubSectionHeader("Activity - Models", habitSectionTitleStyle), 2)[0]

	positions := []int{
		strings.Index(trendsSection, defaultTextStyle.Render("• tokens ")),
		strings.Index(trendsSection, defaultTextStyle.Render("• cost ")),
		strings.Index(trendsSection, defaultTextStyle.Render("• hours ")),
		strings.Index(trendsSection, defaultTextStyle.Render("• lines ")),
	}
	for i, pos := range positions {
		if pos < 0 {
			t.Fatalf("expected trend row %d in %q", i, trendsSection)
		}
	}
	if !(positions[0] < positions[1] && positions[1] < positions[2] && positions[2] < positions[3]) {
		t.Fatalf("expected trend order tokens -> cost -> hours -> lines, got %q", trendsSection)
	}
}

func TestRenderOverviewLines_IncludesModelActivitySection(t *testing.T) {
	report := stats.Report{
		TotalToolCalls:   42,
		UniqueToolCount:  9,
		TotalSubtasks:    11,
		UniqueAgentCount: 3,
		TotalModelTokens: 730,
		UniqueModelCount: 12,
		TotalSkillCalls:  0,
		UniqueSkillCount: 0,
		Days:             make([]stats.Day, 30),
		TopModels: []stats.UsageCount{
			{Name: "[OpenAI] gpt-5.4", Amount: 120},
			{Name: "[Anthropic] claude-sonnet-4.5", Amount: 100},
			{Name: "[Google] gemini-2.5-pro", Amount: 90},
			{Name: "[OpenRouter] qwen/qwen3-coder", Amount: 75},
			{Name: "[Azure] gpt-4.1", Amount: 65},
			{Name: "[Bedrock] claude-3.7-sonnet", Amount: 55},
			{Name: "[Vertex] gemini-2.0-flash", Amount: 50},
			{Name: "[Copilot] gpt-4o", Amount: 45},
			{Name: "[Copilot] mistral-large", Amount: 40},
			{Name: "[OpenAI] o4-mini", Amount: 35},
			{Name: "[Anthropic] claude-haiku-4.5", Amount: 30},
		},
	}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: int64((i + 1) * 1000), Cost: float64(i+1) / 10, SessionMinutes: i + 1}
	}

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")
	modelSection := strings.SplitN(strings.SplitN(content, renderSubSectionHeader("Activity - Models (12)", habitSectionTitleStyle), 2)[1], renderSubSectionHeader("Activity - Agents (3)", habitSectionTitleStyle), 2)[0]

	for _, snippet := range []string{"Activity - Models (12)", "730", "[OpenAI] gpt-5.4", "[Anthropic] claude-haiku-4.5", "Total", "100%", "(16%)"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected model activity snippet %q, got %q", snippet, content)
		}
	}
	for _, snippet := range []string{"• tokens ", "• unique ", "• 1 [OpenAI] gpt-5.4", "• 10 [OpenAI] o4-mini"} {
		if strings.Contains(modelSection, snippet) {
			t.Fatalf("expected old model activity formatting to be removed, got %q", modelSection)
		}
	}
	if strings.Contains(modelSection, "11 [Anthropic] claude-haiku-4.5") {
		t.Fatalf("expected model activity section to keep plain labels without ordinal prefixes, got %q", modelSection)
	}
}

func TestRenderOverviewLines_OrdersActivitySectionsAsRequested(t *testing.T) {
	report := stats.Report{
		UniqueModelCount: 1,
		UniqueAgentCount: 1,
		UniqueSkillCount: 1,
		UniqueToolCount:  1,
		TotalModelTokens: 100,
		TotalSubtasks:    2,
		TotalSkillCalls:  3,
		TotalToolCalls:   4,
		TopModels:        []stats.UsageCount{{Name: "[OpenAI] gpt-5.4", Amount: 100}},
		TopAgents:        []stats.UsageCount{{Name: "explore", Count: 2}},
		TopSkills:        []stats.UsageCount{{Name: "writing-plans", Count: 3}},
		TopTools:         []stats.UsageCount{{Name: "bash", Count: 4}},
		Days:             make([]stats.Day, 30),
	}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 100}
	}

	model := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderOverviewLines(), "\n")

	positions := []int{
		strings.Index(content, renderSubSectionHeader("Activity - Models (1)", habitSectionTitleStyle)),
		strings.Index(content, renderSubSectionHeader("Activity - Agents (1)", habitSectionTitleStyle)),
		strings.Index(content, renderSubSectionHeader("Activity - Skills (1)", habitSectionTitleStyle)),
		strings.Index(content, renderSubSectionHeader("Activity - Tools (1)", habitSectionTitleStyle)),
	}
	for i, pos := range positions {
		if pos < 0 {
			t.Fatalf("expected activity section %d in %q", i, content)
		}
	}
	if !(positions[0] < positions[1] && positions[1] < positions[2] && positions[2] < positions[3]) {
		t.Fatalf("expected activity order models -> agents -> skills -> tools, got %q", content)
	}
}

func TestRenderUsageLines_AlignsBarsToLongestLabel(t *testing.T) {
	lines := renderUsageLines([]stats.UsageCount{
		{Name: "bash", Count: 21},
		{Name: "very-long-tool-name", Count: 11},
		{Name: "go", Count: 8},
	}, 42)

	if len(lines) != 4 {
		t.Fatalf("expected 4 usage lines, got %d", len(lines))
	}

	shortNeedle := defaultTextStyle.Render("• bash ") + defaultTextStyle.Render(strings.Repeat(" ", len("• very-long-tool-name ")-len("• bash "))) + statsValueTextStyle.Render("████████ 21 (50%)")
	longNeedle := defaultTextStyle.Render("• very-long-tool-name ") + statsValueTextStyle.Render("████···· 11 (26%)")
	thirdNeedle := defaultTextStyle.Render("• go ") + defaultTextStyle.Render(strings.Repeat(" ", len("• very-long-tool-name ")-len("• go "))) + statsValueTextStyle.Render("███····· 8 (19%)")
	totalNeedle := defaultTextStyle.Render("• Total ") + defaultTextStyle.Render(strings.Repeat(" ", len("• very-long-tool-name ")-len("• Total "))) + statsValueTextStyle.Render("········ 42 (100%)")

	if lines[0] != bulletLine(shortNeedle) {
		t.Fatalf("expected aligned first usage line %q, got %q", bulletLine(shortNeedle), lines[0])
	}
	if lines[1] != bulletLine(longNeedle) {
		t.Fatalf("expected aligned second usage line %q, got %q", bulletLine(longNeedle), lines[1])
	}
	if lines[2] != bulletLine(thirdNeedle) {
		t.Fatalf("expected aligned third usage line %q, got %q", bulletLine(thirdNeedle), lines[2])
	}
	if lines[3] != bulletLine(totalNeedle) {
		t.Fatalf("expected aligned total usage line %q, got %q", bulletLine(totalNeedle), lines[3])
	}
}

func TestRenderUsageLines_GroupsRemainderIntoOthersAfterTop15(t *testing.T) {
	items := make([]stats.UsageCount, 0, 17)
	total := int64(0)
	for i := range 17 {
		count := 20 - i
		items = append(items, stats.UsageCount{Name: fmt.Sprintf("tool-%02d", i+1), Count: count})
		total += int64(count)
	}

	lines := renderUsageLines(items, total)

	if len(lines) != 17 {
		t.Fatalf("expected 17 usage lines including others and total, got %d", len(lines))
	}
	if !strings.Contains(lines[15], "others") {
		t.Fatalf("expected others row at index 15, got %q", lines[15])
	}
	if !strings.Contains(lines[15], "9") || !strings.Contains(lines[15], "(4%)") {
		t.Fatalf("expected others row to aggregate hidden items, got %q", lines[15])
	}
	if !strings.Contains(lines[16], "204") || !strings.Contains(lines[16], "100%") {
		t.Fatalf("expected total row to remain at the end, got %q", lines[16])
	}
}

func TestRenderUsageLines_AlignsOthersAndTotalToLongestLabel(t *testing.T) {
	items := make([]stats.UsageCount, 0, 16)
	for i := range 16 {
		items = append(items, stats.UsageCount{Name: fmt.Sprintf("t%d", i+1), Count: 20 - i})
	}

	lines := renderUsageLines(items, 200)
	if len(lines) < 3 {
		t.Fatalf("expected usage lines, got %v", lines)
	}
	othersLine := lines[len(lines)-2]
	totalLine := lines[len(lines)-1]
	othersNeedle := defaultTextStyle.Render("• others ")
	totalNeedle := defaultTextStyle.Render("• Total ") + defaultTextStyle.Render(" ")
	if !strings.Contains(othersLine, othersNeedle) {
		t.Fatalf("expected others line, got %q", othersLine)
	}
	if !strings.Contains(totalLine, totalNeedle) {
		t.Fatalf("expected padded total line aligned to others label, got %q", totalLine)
	}
}

func TestRenderUsageLines_GroupsLargeCounts(t *testing.T) {
	lines := renderUsageLines([]stats.UsageCount{{Name: "bash", Count: 12345}}, 23456)

	if len(lines) != 2 {
		t.Fatalf("expected 2 usage lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "12,345") {
		t.Fatalf("expected grouped usage count, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "23,456") || !strings.Contains(lines[1], "100%") {
		t.Fatalf("expected grouped total usage count, got %q", lines[1])
	}
	if strings.Contains(lines[0], "• 1 bash ") {
		t.Fatalf("expected no ordinal prefix in usage row, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "········") {
		t.Fatalf("expected neutral placeholder bar in total row, got %q", lines[1])
	}
	if strings.Contains(lines[1], "████") {
		t.Fatalf("expected total row to avoid filled bars, got %q", lines[1])
	}
}

func TestRenderUsageLines_ShowsPlaceholderOnlyWhenTotalIsZero(t *testing.T) {
	lines := renderUsageLines(nil, 0)

	if len(lines) != 1 {
		t.Fatalf("expected 1 usage line, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "top 15") {
		t.Fatalf("expected placeholder row, got %q", lines[0])
	}
	if strings.Contains(lines[0], "Total") {
		t.Fatalf("expected no total row for zero totals, got %q", lines[0])
	}
}

func TestRenderUsageLines_FormatsModelAmountsCompactly(t *testing.T) {
	lines := renderUsageLines([]stats.UsageCount{{Name: "[OpenAI] gpt-5.4", Amount: 1_250_000}}, 1_500_000)

	if len(lines) != 2 {
		t.Fatalf("expected 2 usage lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "1.2M") {
		t.Fatalf("expected compact model amount in usage row, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "1.5M") || !strings.Contains(lines[1], "100%") {
		t.Fatalf("expected compact model amount in total row, got %q", lines[1])
	}
}

func TestRenderWindowLines_GroupsSummaryCounts(t *testing.T) {
	report := stats.WindowReport{
		Label:    "Daily",
		Start:    time.Date(2026, time.March, 28, 0, 0, 0, 0, time.Local),
		End:      time.Date(2026, time.March, 29, 0, 0, 0, 0, time.Local),
		Messages: 12345,
		Sessions: 2345,
		Tokens:   987654,
		Cost:     1234.56,
	}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	content := strings.Join(model.renderWindowLines(report), "\n")

	for _, snippet := range []string{"12,345", "2,345", "988k", "$1,234.56"} {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected grouped window snippet %q, got %q", snippet, content)
		}
	}
}

func TestWindowSessionRows_GroupsMessageCounts(t *testing.T) {
	report := stats.WindowReport{TopSessions: []stats.SessionUsage{{ID: "ses_big", Messages: 12345}}}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	rows := model.windowSessionRows(report)

	if got := rows[0][4]; got != "12,345" {
		t.Fatalf("expected grouped session message count, got %q", got)
	}
}

func TestRenderValueTrend_HighlightsTodayCellLikeRhythm(t *testing.T) {
	days := []stats.Day{
		{Date: time.Now().AddDate(0, 0, -2), Cost: 1},
		{Date: time.Now().AddDate(0, 0, -1), Cost: 2},
		{Date: time.Now(), Cost: 3},
	}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	trend := renderValueTrend(days, func(day stats.Day) float64 { return day.Cost })
	normalTodayCell := lipgloss.NewStyle().Foreground(lipgloss.Color("#B8B8B8")).Render("█")
	highlightedTodayCell := model.renderHeatmapCell(stats.Day{Tokens: 5_000_000, AssistantMessages: 1}, true)

	if !strings.HasSuffix(trend, highlightedTodayCell) {
		t.Fatalf("expected today trend cell to use rhythm today highlight, got %q", trend)
	}
	if strings.HasSuffix(trend, normalTodayCell) {
		t.Fatalf("expected today trend cell to avoid normal gray color, got %q", trend)
	}
}

type usageFixture struct {
	Name  string
	Count int
}

func setRankedUsageField(report *stats.Report, fieldName string, usage []usageFixture) {
	value := reflect.ValueOf(report).Elem()
	field := value.FieldByName(fieldName)
	if !field.IsValid() || !field.CanSet() {
		return
	}
	items := reflect.MakeSlice(field.Type(), 0, len(usage))
	for _, item := range usage {
		entry := reflect.New(field.Type().Elem()).Elem()
		entry.FieldByName("Name").SetString(item.Name)
		entry.FieldByName("Count").SetInt(int64(item.Count))
		items = reflect.Append(items, entry)
	}
	field.Set(items)
}

func TestUpdate_TabSwitchesToStatsAndEscReturns(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if !model.statsMode {
		t.Fatal("expected stats mode after tab")
	}
	if !strings.Contains(model.View().Content, "Overview") {
		t.Fatalf("expected stats tab view, got %q", model.View().Content)
	}
	if strings.Contains(model.View().Content, "active days(30d)") || strings.Contains(model.View().Content, "current streak") || strings.Contains(model.View().Content, "best streak") || strings.Contains(model.View().Content, "5-week heatmap") {
		t.Fatalf("expected overview cleanup to remove duplicated summary block, got %q", model.View().Content)
	}
	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	if model.statsMode {
		t.Fatal("expected esc to return to launcher")
	}
}

func TestUpdate_LeftRightMovesStatsTabs(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, nil, true)
	updated, _ := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	if model.statsTab != 1 {
		t.Fatalf("expected stats tab 1, got %d", model.statsTab)
	}
	if !strings.Contains(model.View().Content, "# Tokens used") {
		t.Fatalf("expected daily tab content, got %q", model.View().Content)
	}
	updated, _ = model.Update(mockKeyMsg("left"))
	model = updated.(Model)
	if model.statsTab != 0 {
		t.Fatalf("expected stats tab 0, got %d", model.statsTab)
	}
}

func TestRenderStatsTabs_ShowsUnderlineStyleTabsWithMetadata(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: start.AddDate(0, 0, i)}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.statsTab = 0

	rendered := model.renderStatsTabs()
	lines := strings.Split(rendered, "\n")

	if len(lines) != 2 {
		t.Fatalf("expected two-line tab render with underline row, got %d in %q", len(lines), rendered)
	}
	if got := lipgloss.Width(lines[0]); got != statsTabRowWidth {
		t.Fatalf("expected first tab row width %d, got %d in %q", statsTabRowWidth, got, lines[0])
	}
	if got := lipgloss.Width(lines[1]); got != statsTabRowWidth {
		t.Fatalf("expected underline row width %d, got %d in %q", statsTabRowWidth, got, lines[1])
	}
	for _, snippet := range []string{"Overview", "Daily", "Monthly", "Global", "2026-03-01~2026-03-30"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected %q in tab row, got %q", snippet, rendered)
		}
	}
	for _, snippet := range []string{"   Overview   ", "   Daily   ", "   Monthly   ", " Global (2026-03-01~2026-03-30)"} {
		if !strings.Contains(rendered, snippet) {
			t.Fatalf("expected padded snippet %q in tab row, got %q", snippet, rendered)
		}
	}
	if strings.Contains(rendered, "|") {
		t.Fatalf("expected conceptual tab grouping without literal pipe characters, got %q", rendered)
	}
	activeIndicator := statsTabIndicatorStyle.Render(strings.Repeat("▔", statsTabWidth))
	if !strings.Contains(lines[1], activeIndicator) {
		t.Fatalf("expected active underline indicator in %q", lines[1])
	}

	model.projectScope = true
	updated := model.renderStatsTabs()
	if rendered == updated {
		t.Fatal("expected tab row to change when scope changes")
	}
	if !strings.Contains(updated, "Project") {
		t.Fatalf("expected project scope label, got %q", updated)
	}
}

func TestAvailableStatsRows_AccountsForTwoLineTabs(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	model.height = 12

	if got := model.availableStatsRows(); got != 5 {
		t.Fatalf("expected 5 visible rows after two-line tabs, got %d", got)
	}
}

func TestRenderStatsView_RemovesBlankLineBelowTabs(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i)), Tokens: 100}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)

	rendered := model.renderStatsView()
	tabs := model.renderStatsTabs()
	if strings.Contains(rendered, tabs+"\n\n") {
		t.Fatalf("expected stats content to start immediately below tabs, got %q", rendered)
	}
	if !strings.Contains(rendered, tabs+"\n") {
		t.Fatalf("expected content to follow tabs directly, got %q", rendered)
	}
}

func TestRenderStatsTabs_UsesWindowRangeForMonthlyTab(t *testing.T) {
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: time.Date(2026, time.February, 1, 0, 0, 0, 0, time.Local).AddDate(0, 0, i)}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.statsTab = 2
	model.globalMonthly = stats.WindowReport{
		Label: "Monthly",
		Start: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local),
		End:   time.Date(2026, time.April, 1, 0, 0, 0, 0, time.Local),
	}

	rendered := model.renderStatsTabs()
	if !strings.Contains(rendered, "Global (2026-03-01~2026-03-31)") {
		t.Fatalf("expected monthly window range in tab metadata, got %q", rendered)
	}
}

func TestUpdate_GTogglesProjectScopeAndHeaders(t *testing.T) {
	globalReport := stats.Report{Days: make([]stats.Day, 30)}
	projectReport := stats.Report{Days: make([]stats.Day, 30)}
	for i := range globalReport.Days {
		globalReport.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i))}
		projectReport.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i))}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, globalReport, projectReport, config.StatsConfig{}, testVersion, true)
	updated, _ := model.Update(mockKeyMsg("g"))
	model = updated.(Model)
	if !model.projectScope {
		t.Fatal("expected project scope after g toggle")
	}
	view := model.View().Content
	if !strings.Contains(view, "[Project] Rhythm") || !strings.Contains(view, "[Project] Metrics") {
		t.Fatalf("expected project-prefixed headers, got %q", view)
	}
	updated, _ = model.Update(mockKeyMsg("g"))
	model = updated.(Model)
	if model.projectScope {
		t.Fatal("expected g to toggle back to global scope")
	}
}

func TestNewModel_UsesConfiguredDefaultProjectScope(t *testing.T) {
	globalReport := stats.Report{Days: make([]stats.Day, 30)}
	projectReport := stats.Report{Days: make([]stats.Day, 30)}
	for i := range globalReport.Days {
		globalReport.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i))}
		projectReport.Days[i] = stats.Day{Date: time.Now().AddDate(0, 0, -(29 - i))}
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, globalReport, projectReport, config.StatsConfig{DefaultScope: "project"}, testVersion, true)
	if !model.projectScope {
		t.Fatal("expected project scope from config default")
	}
	view := model.View().Content
	if !strings.Contains(view, "[Project] Rhythm") || !strings.Contains(view, "[Project] Metrics") {
		t.Fatalf("expected project-prefixed headers from default scope, got %q", view)
	}
}

func TestUpdate_StatsViewScrollsWithUpDown(t *testing.T) {
	report := stats.Report{}
	daily := stats.WindowReport{Label: "Daily"}
	for i := 0; i < 20; i++ {
		daily.TopSessions = append(daily.TopSessions, stats.SessionUsage{ID: fmt.Sprintf("ses_%02d", i), Title: fmt.Sprintf("Title %02d", i), Messages: i + 1})
	}
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)
	model.globalDaily = daily
	model.globalDailyLoaded = true
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 12})
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	before := model.View().Content
	updated, _ = model.Update(mockKeyMsg("down"))
	model = updated.(Model)
	after := model.View().Content
	if before == after {
		t.Fatalf("expected stats view to change after scrolling down")
	}
	if model.statsOffset == 0 {
		t.Fatalf("expected statsOffset to increase after scrolling, got %d", model.statsOffset)
	}
}

func TestModel_LoadsOnlyVisibleStatsViewAndCachesWithinTTL(t *testing.T) {
	var overviewLoads, dailyLoads int
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true).
		WithStatsLoaders(
			func() (stats.Report, error) {
				overviewLoads++
				return stats.Report{Days: make([]stats.Day, 30)}, nil
			},
			func() (stats.Report, error) {
				overviewLoads++
				return stats.Report{Days: make([]stats.Day, 30)}, nil
			},
			func(label string, start, end time.Time) (stats.WindowReport, error) {
				dailyLoads++
				return stats.WindowReport{Label: label}, nil
			},
			func(label string, start, end time.Time) (stats.WindowReport, error) {
				dailyLoads++
				return stats.WindowReport{Label: label}, nil
			},
		)

	if cmd := model.Init(); cmd == nil {
		t.Fatal("expected init to load overview")
	} else {
		updated, _ := model.Update(cmd())
		model = updated.(Model)
	}
	if overviewLoads != 1 {
		t.Fatalf("expected one overview load, got %d", overviewLoads)
	}
	updated, cmd := model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("expected no extra load when entering stats overview with fresh cache")
	}
	updated, cmd = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	if cmd == nil {
		t.Fatal("expected daily tab to trigger window load")
	}
	updated, _ = model.Update(cmd())
	model = updated.(Model)
	if dailyLoads != 1 {
		t.Fatalf("expected one daily load, got %d", dailyLoads)
	}
	updated, cmd = model.Update(mockKeyMsg("left"))
	model = updated.(Model)
	updated, cmd = model.Update(mockKeyMsg("right"))
	model = updated.(Model)
	if cmd != nil {
		t.Fatal("expected fresh daily cache to avoid reload")
	}
}

func TestView_AnalyticsMinimapAdaptsToNarrowWidths(t *testing.T) {
	now := time.Now()
	report := stats.Report{Days: make([]stats.Day, 30)}
	for i := range report.Days {
		report.Days[i] = stats.Day{Date: now.AddDate(0, 0, -(29 - i)), Tokens: 1000}
	}
	report.Days[len(report.Days)-1].Tokens = 160000
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, nil, SessionItem{}, report, report, config.StatsConfig{}, testVersion, true)

	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	wideView := updated.(Model).View().Content
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 50, Height: 30})
	narrowView := updated.(Model).View().Content
	updated, _ = model.Update(tea.WindowSizeMsg{Width: 35, Height: 30})
	tinyView := updated.(Model).View().Content

	if !strings.Contains(wideView, defaultTextStyle.Render("• active ")+statsValueTextStyle.Render("0/30d")) {
		t.Fatalf("expected active label in wide view, got %q", wideView)
	}
	if strings.Count(wideView, "█")+strings.Count(wideView, "▓")+strings.Count(wideView, "░")+strings.Count(wideView, "·") < 28 {
		t.Fatalf("expected 4-week minimap density in wide view, got %q", wideView)
	}
	if !strings.Contains(narrowView, defaultTextStyle.Render("• active ")+statsValueTextStyle.Render("0/30d")) {
		t.Fatalf("expected active label in narrow view, got %q", narrowView)
	}
	if !strings.Contains(tinyView, defaultTextStyle.Render("    ")+defaultTextStyle.Render("• active ")+statsValueTextStyle.Render("0/30d")) {
		t.Fatalf("expected active label to remain in tiny view, got %q", tinyView)
	}
	if strings.Contains(tinyView, "·") || strings.Contains(tinyView, "░") || strings.Contains(tinyView, "▓") || strings.Contains(tinyView, "█") {
		t.Fatalf("expected minimap cells hidden in tiny view, got %q", tinyView)
	}
	if !strings.Contains(tinyView, "streak") || !strings.Contains(tinyView, "cost") {
		t.Fatalf("expected core metrics to remain in tiny view, got %q", tinyView)
	}
}

func TestRenderHeatmapCell_TodayUsesDifferentColor(t *testing.T) {
	day := stats.Day{Tokens: 5_000_000, AssistantMessages: 1}
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
	normal := model.renderHeatmapCell(day, false)
	today := model.renderHeatmapCell(day, true)
	if normal == today {
		t.Fatalf("expected today heatmap cell to differ from normal cell: %q", today)
	}
	if !strings.Contains(today, "█") {
		t.Fatalf("expected high activity today cell to keep block rune, got %q", today)
	}
}

func TestActivityLevel_UsesTokenThresholds(t *testing.T) {
	cases := []struct {
		name string
		day  stats.Day
		want int
	}{
		{name: "inactive", day: stats.Day{}, want: 0},
		{name: "low from activity", day: stats.Day{AssistantMessages: 1}, want: 1},
		{name: "medium tokens", day: stats.Day{Tokens: 1_000_000}, want: 2},
		{name: "high tokens", day: stats.Day{Tokens: 5_000_000}, want: 3},
	}
	for _, tc := range cases {
		model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{}, testVersion, true)
		if got := model.activityLevel(tc.day); got != tc.want {
			t.Fatalf("%s: expected level %d, got %d", tc.name, tc.want, got)
		}
	}
}

func TestActivityLevel_UsesConfiguredTokenThresholds(t *testing.T) {
	model := NewModel(nil, nil, nil, SessionItem{}, stats.Report{}, stats.Report{}, config.StatsConfig{MediumTokens: 2000, HighTokens: 5000}, testVersion, true)
	if got := model.activityLevel(stats.Day{Tokens: 1999, AssistantMessages: 1}); got != 1 {
		t.Fatalf("expected low activity below medium threshold, got %d", got)
	}
	if got := model.activityLevel(stats.Day{Tokens: 2000}); got != 2 {
		t.Fatalf("expected medium activity at configured threshold, got %d", got)
	}
	if got := model.activityLevel(stats.Day{Tokens: 5000}); got != 3 {
		t.Fatalf("expected high activity at configured threshold, got %d", got)
	}
}

func TestFormatCompactTokens_UsesMillions(t *testing.T) {
	if got := formatCompactTokens(999999); got != "1000k" {
		t.Fatalf("expected 1000k below one million boundary, got %q", got)
	}
	if got := formatCompactTokens(1_000_000); got != "1.0M" {
		t.Fatalf("expected 1.0M at one million boundary, got %q", got)
	}
	if got := formatCompactTokens(12_340_000); got != "12.3M" {
		t.Fatalf("expected 12.3M for millions, got %q", got)
	}
	if got := formatCurrency(1234.56); got != "$1,234.56" {
		t.Fatalf("expected grouped currency, got %q", got)
	}
}

func TestView_ClearsOnConfirm(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	updatedModel, _ := model.Update(mockKeyMsg("enter"))

	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after confirm, got %q", got)
	}
}

func TestView_ClearsOnCancel(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	updatedModel, _ := model.Update(mockKeyMsg("ctrl+c"))

	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after cancel, got %q", got)
	}
}

func TestView_ClearsOnEditSelection(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	updatedModel, _ = updatedModel.(Model).Update(mockKeyMsg("enter"))

	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after edit selection, got %q", got)
	}
}
