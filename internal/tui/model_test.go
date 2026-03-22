package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const testVersion = "v0.1.1"

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

	m := NewModel(items, nil, testVersion, true)

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
}

func TestNewModel_EmptyList(t *testing.T) {
	m := NewModel([]PluginItem{}, nil, testVersion, true)

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

	m := NewModel(items, nil, testVersion, false)
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

	m := NewModel(items, nil, testVersion, true)

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

	m := NewModel(items, nil, testVersion, true)
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

	m := NewModel(items, nil, testVersion, true)
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

	m := NewModel(items, nil, testVersion, true)
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

	m := NewModel(items, nil, testVersion, true)

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

	m := NewModel(items, nil, testVersion, true)

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

	m := NewModel(items, nil, testVersion, false)
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

	m := NewModel(items, nil, testVersion, true)
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

	m := NewModel(items, nil, testVersion, true)

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

	m := NewModel(items, nil, testVersion, true)

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
		m := NewModel(items, nil, testVersion, true)
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

	m := NewModel(items, editChoices, testVersion, true)

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

	m := NewModel(items, editChoices, testVersion, true)
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

	m := NewModel(items, editChoices, testVersion, true)
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

func TestSelections_Output(t *testing.T) {
	items := []PluginItem{
		{Name: "plugin-a", InitiallyEnabled: true},
		{Name: "plugin-b", InitiallyEnabled: false},
		{Name: "plugin-c", InitiallyEnabled: true},
	}

	m := NewModel(items, nil, testVersion, true)

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

	m := NewModel(items, nil, testVersion, true)
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

	m := NewModel(items, editChoices, testVersion, true)
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
	headerBaseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	if !strings.Contains(headerLine, "Open"+headerBaseStyle.Render("Code")+" launcher") {
		t.Fatalf("expected updated header copy, got %q", headerLine)
	}
	if strings.Contains(headerLine, "Launching") {
		t.Fatalf("expected removed launch wording, got %q", headerLine)
	}
	if strings.Contains(headerLine, "with plugins") {
		t.Fatalf("expected removed plugin wording, got %q", headerLine)
	}
	if !strings.Contains(headerLine, "⚡ "+headerAccentStyle.Render("O")+headerBaseStyle.Render("C")+" "+testVersion+" - ") {
		t.Fatalf("expected accented OC fragment in header, got %q", headerLine)
	}
	if !strings.Contains(headerLine, "Open"+headerBaseStyle.Render("Code")+" launcher") {
		t.Fatalf("expected explicit white style for Code in header, got %q", headerLine)
	}
	if strings.Contains(headerLine, headerAccentStyle.Render("OpenCode")) {
		t.Fatalf("expected OpenCode to be only partially accented, got %q", headerLine)
	}
}

func TestRenderHeader_IncludesVersion(t *testing.T) {
	headerLine := Model{version: testVersion}.renderHeader()
	headerAccentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	headerBaseStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))

	expectedPrefix := "⚡ " + headerAccentStyle.Render("O") + headerBaseStyle.Render("C") + " " + testVersion + " - "
	if !strings.HasPrefix(headerLine, expectedPrefix) {
		t.Fatalf("expected header to start with %q, got %q", expectedPrefix, headerLine)
	}
	if !strings.Contains(headerLine, "Open"+headerBaseStyle.Render("Code")+" launcher") {
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
	cursorSelectedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	rowLine := stylePluginRow("> ✔  plugin-a", true, true)
	expected := cursorSelectedStyle.Render("> ✔  plugin-a")

	if !strings.Contains(rowLine, expected) {
		t.Fatalf("expected focused+selected style %q in %q", expected, rowLine)
	}
}

func TestRenderHelpLine_IncludesStyledKeyTokens(t *testing.T) {
	helpKeyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFCC00")).Bold(true)
	helpLine := renderHelpLine()

	for _, token := range []string{"↑/↓", "space", "enter", "e", "q"} {
		if !strings.Contains(helpLine, helpKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}

	for _, action := range []string{"navigate", "toggle", "confirm", "edit config", "quit"} {
		if !strings.Contains(helpLine, action) {
			t.Fatalf("expected plain help action %q in %q", action, helpLine)
		}
		if strings.Contains(helpLine, helpKeyStyle.Render(action)) {
			t.Fatalf("expected action %q to remain unstyled in %q", action, helpLine)
		}
	}
}

func TestView_RendersStyledHeaderLine(t *testing.T) {
	view := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, testVersion, true).View().Content
	headerLine := strings.Split(view, "\n")[0]

	expected := Model{version: testVersion}.renderHeader()
	if headerLine != expected {
		t.Fatalf("expected header line %q, got %q", expected, headerLine)
	}
}

func TestView_RendersPluginSelectionPrompt(t *testing.T) {
	view := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, testVersion, true).View().Content
	promptLine := strings.Split(view, "\n")[2]

	if promptLine != "📋 Choose plugins to enable" {
		t.Fatalf("expected plugin prompt line %q, got %q", "📋 Choose plugins to enable", promptLine)
	}
}

func TestView_RendersFocusedSelectedRowLine(t *testing.T) {
	view := NewModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, testVersion, true).View().Content
	rowLine := strings.Split(view, "\n")[4]
	expected := stylePluginRow("> ✔  plugin-a", true, true)

	if rowLine != expected {
		t.Fatalf("expected row line %q, got %q", expected, rowLine)
	}
}

func TestView_EditModeRendersStyledHeaderLine(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, testVersion, true)
	updatedModel, _ := model.Update(mockKeyMsg("e"))
	view := updatedModel.(Model).View().Content
	headerLine := strings.Split(view, "\n")[0]

	expected := Model{version: testVersion}.renderHeader()
	if headerLine != expected {
		t.Fatalf("expected edit-mode header line %q, got %q", expected, headerLine)
	}
}

func TestView_RendersStyledHelpLine(t *testing.T) {
	view := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, testVersion, true).View().Content
	helpLine := strings.Split(view, "\n")[6]

	if helpLine != renderHelpLine() {
		t.Fatalf("expected help line %q, got %q", renderHelpLine(), helpLine)
	}
}

func TestView_ClearsOnConfirm(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, testVersion, true)
	updatedModel, _ := model.Update(mockKeyMsg("enter"))

	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after confirm, got %q", got)
	}
}

func TestView_ClearsOnCancel(t *testing.T) {
	model := NewModel([]PluginItem{{Name: "plugin-a"}}, nil, testVersion, true)
	updatedModel, _ := model.Update(mockKeyMsg("ctrl+c"))

	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after cancel, got %q", got)
	}
}

func TestView_ClearsOnEditSelection(t *testing.T) {
	model := NewModel(
		[]PluginItem{{Name: "plugin-a"}},
		[]EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}},
		testVersion,
		true,
	)
	updatedModel, _ := model.Update(mockKeyMsg("e"))
	updatedModel, _ = updatedModel.(Model).Update(mockKeyMsg("enter"))

	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after edit selection, got %q", got)
	}
}
