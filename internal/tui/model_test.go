package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const testVersion = "v0.1.5"

func newTestModel(items []PluginItem, editChoices []EditChoice, allowMultiplePlugins bool) Model {
	return NewModel(items, editChoices, nil, SessionItem{}, testVersion, allowMultiplePlugins)
}

func newTestModelWithSession(items []PluginItem, editChoices []EditChoice, sessions []SessionItem, session SessionItem, allowMultiplePlugins bool) Model {
	return NewModel(items, editChoices, sessions, session, testVersion, allowMultiplePlugins)
}

// mockKeyMsg creates a KeyPressMsg for testing
func mockKeyMsg(key string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: key}
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

	newModel, cmd := m.Update(mockKeyMsg("e"))
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
	newModel, _ := m.Update(mockKeyMsg("e"))
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
	newModel, _ := m.Update(mockKeyMsg("e"))
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

	newModel, _ := m.Update(mockKeyMsg("e"))
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

func TestRenderHeader_HighlightsBrandFragmentsOnly(t *testing.T) {
	headerLine := Model{version: testVersion}.renderHeader()
	headerAccentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	headerBaseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	headerWordStyle := lipgloss.NewStyle().Bold(true)
	defaultTextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A"))

	if !strings.Contains(headerLine, headerWordStyle.Render("Code")) {
		t.Fatalf("expected updated header copy, got %q", headerLine)
	}
	if !strings.Contains(headerLine, "Open") || !strings.Contains(headerLine, "launcher") {
		t.Fatalf("expected header text fragments to remain present, got %q", headerLine)
	}
	if strings.Contains(headerLine, "Launching") {
		t.Fatalf("expected removed launch wording, got %q", headerLine)
	}
	if strings.Contains(headerLine, "with plugins") {
		t.Fatalf("expected removed plugin wording, got %q", headerLine)
	}
	if !strings.Contains(headerLine, headerAccentStyle.Render("O")) || !strings.Contains(headerLine, headerWordStyle.Render("Open")) {
		t.Fatalf("expected accented O and bold-only Open fragment in header, got %q", headerLine)
	}
	if !strings.Contains(headerLine, defaultTextStyle.Render("⚡ ")) || !strings.Contains(headerLine, defaultTextStyle.Render(" launcher")) {
		t.Fatalf("expected default text color around highlighted header fragments, got %q", headerLine)
	}
	if !strings.Contains(headerLine, headerBaseStyle.Render("C")) || !strings.Contains(headerLine, headerWordStyle.Render("Code")) {
		t.Fatalf("expected C to stay white bold and Code to use bold-only styling, got %q", headerLine)
	}
}

func TestRenderHeader_IncludesVersion(t *testing.T) {
	headerLine := Model{version: testVersion}.renderHeader()
	headerAccentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	headerBaseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	headerWordStyle := lipgloss.NewStyle().Bold(true)
	defaultTextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A"))

	if !strings.HasPrefix(headerLine, defaultTextStyle.Render("⚡ ")+headerAccentStyle.Render("O")+headerBaseStyle.Render("C")) {
		t.Fatalf("expected header to start with accented OC fragment, got %q", headerLine)
	}
	if !strings.Contains(headerLine, testVersion) {
		t.Fatalf("expected version in header, got %q", headerLine)
	}
	if !strings.Contains(headerLine, headerWordStyle.Render("Open")) {
		t.Fatalf("expected Open to use bold-only styling, got %q", headerLine)
	}
	if !strings.Contains(headerLine, headerWordStyle.Render("Code")) || !strings.Contains(headerLine, defaultTextStyle.Render(" launcher")) {
		t.Fatalf("expected new launcher wording, got %q", headerLine)
	}
	if strings.Contains(headerLine, "Launching ") {
		t.Fatalf("expected removed launch wording, got %q", headerLine)
	}
	if strings.Contains(headerLine, "with plugins") {
		t.Fatalf("expected removed plugin wording, got %q", headerLine)
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
	helpKeyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	defaultTextStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7A7A7A"))
	helpLine := renderHelpLine()

	for _, token := range []string{"↑/↓", "space", "enter", "s", "e", "q"} {
		if !strings.Contains(helpLine, helpKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}

	for _, action := range []string{"navigate", "toggle", "confirm", "sessions", "edit config", "quit"} {
		if !strings.Contains(helpLine, action) {
			t.Fatalf("expected plain help action %q in %q", action, helpLine)
		}
		if strings.Contains(helpLine, helpKeyStyle.Render(action)) {
			t.Fatalf("expected action %q to remain unstyled in %q", action, helpLine)
		}
	}
	if !strings.Contains(helpLine, defaultTextStyle.Render(": quit")) {
		t.Fatalf("expected default text color on help copy, got %q", helpLine)
	}
}

func TestView_RendersStyledHeaderLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	headerLine := strings.Split(view, "\n")[0]

	expected := Model{version: testVersion}.renderHeader()
	if headerLine != expected {
		t.Fatalf("expected header line %q, got %q", expected, headerLine)
	}
}

func TestView_RendersPluginSelectionPrompt(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	promptLine := strings.Split(view, "\n")[4]

	expected := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render("📋 Choose plugins to enable")
	if promptLine != expected {
		t.Fatalf("expected plugin prompt line %q, got %q", expected, promptLine)
	}
}

func TestView_EditModeRendersInstructionPrompt(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("e"))
	view := updatedModel.(Model).View().Content
	promptLine := strings.Split(view, "\n")[4]

	expected := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render("📂 Choose config to edit")
	if promptLine != expected {
		t.Fatalf("expected edit prompt line %q, got %q", expected, promptLine)
	}
}

func TestView_SessionModeRendersInstructionPrompt(t *testing.T) {
	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, []SessionItem{{ID: "ses_latest", Title: "Latest session"}}, SessionItem{}, true)
	updatedModel, _ := model.Update(mockKeyMsg("s"))
	view := updatedModel.(Model).View().Content
	promptLine := strings.Split(view, "\n")[4]

	expected := lipgloss.NewStyle().Foreground(lipgloss.Color("#999999")).Render("🕘 Choose session")
	if promptLine != expected {
		t.Fatalf("expected session prompt line %q, got %q", expected, promptLine)
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
	rowLine := strings.Split(view, "\n")[7]
	expected := stylePluginRow("> "+sessionTimestampPrefix(now, now)+"Latest session (ses_latest)", true, true)

	if rowLine != expected {
		t.Fatalf("expected unboxed session row %q, got %q", expected, rowLine)
	}
}

func TestView_RendersFocusedSelectedRowLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, true).View().Content
	rowLine := strings.Split(view, "\n")[6]
	expected := stylePluginRow("> ✔  plugin-a", true, true)

	if rowLine != expected {
		t.Fatalf("expected row line %q, got %q", expected, rowLine)
	}
}

func TestView_EditModeRendersStyledHeaderLine(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("e"))
	view := updatedModel.(Model).View().Content
	headerLine := strings.Split(view, "\n")[0]

	expected := Model{version: testVersion}.renderHeader()
	if headerLine != expected {
		t.Fatalf("expected edit-mode header line %q, got %q", expected, headerLine)
	}
}

func TestView_RendersStyledHelpLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	helpLine := strings.Split(view, "\n")[8]

	if helpLine != renderHelpLine() {
		t.Fatalf("expected help line %q, got %q", renderHelpLine(), helpLine)
	}
}

func TestView_RendersSelectedSessionLineAfterHeader(t *testing.T) {
	session := SessionItem{ID: "ses_latest", Title: "Latest session", UpdatedAt: time.Date(2026, time.March, 23, 9, 8, 7, 0, time.Local)}
	view := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, nil, []SessionItem{session}, session, true).View().Content
	line := strings.Split(view, "\n")[2]

	if line != renderSelectedSession(session) {
		t.Fatalf("expected selected session line %q, got %q", renderSelectedSession(session), line)
	}
}

func TestRenderSelectedSession_PlacesTimestampAfterLabel(t *testing.T) {
	now := time.Now()
	session := SessionItem{ID: "ses_latest", Title: "Latest session", UpdatedAt: now.Add(-2 * time.Hour)}
	rendered := renderSelectedSession(session)
	expectedPrefix := sessionLabelStyle.Render("Session") + ": " + sessionStyle.Render(sessionTimestampPrefix(session.UpdatedAt, now))

	if !strings.HasPrefix(rendered, expectedPrefix) {
		t.Fatalf("expected selected session prefix %q, got %q", expectedPrefix, rendered)
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
	updatedModel, _ := model.Update(mockKeyMsg("e"))
	updatedModel, _ = updatedModel.(Model).Update(mockKeyMsg("enter"))

	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after edit selection, got %q", got)
	}
}
