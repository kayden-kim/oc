package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

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

	if m.confirmed {
		t.Error("expected confirmed=false for empty list")
	}
	if m.cancelled {
		t.Error("expected cancelled=false for empty list")
	}
}

func TestView_EmptyListStillShowsOcScreen(t *testing.T) {
	m := newTestModel([]PluginItem{}, nil, true)
	view := m.View().Content

	if !strings.Contains(view, renderSectionHeader("📋 Choose plugins", maxLayoutWidth)) {
		t.Fatalf("expected empty plugin list to still render oc screen, got %q", view)
	}
	if !strings.Contains(view, renderHelpLine(maxLayoutWidth)) {
		t.Fatalf("expected empty plugin list to still render help line, got %q", view)
	}
	if !strings.Contains(view, "Press enter to launch opencode") {
		t.Fatalf("expected empty plugin list to show launch hint, got %q", view)
	}
}

func TestUpdate_EmptyListEnterConfirms(t *testing.T) {
	m := newTestModel([]PluginItem{}, nil, true)
	updated, cmd := m.Update(mockKeyMsg("enter"))
	m = updated.(Model)

	if !m.confirmed {
		t.Fatal("expected enter to confirm even with empty plugin list")
	}
	if cmd == nil {
		t.Fatal("expected enter on empty plugin list to quit")
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
	items := []PluginItem{{Name: "plugin-a"}, {Name: "plugin-b"}, {Name: "plugin-c"}}
	m := newTestModel(items, nil, true)

	newModel, _ := m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1, got %d", m.cursor)
	}

	newModel, _ = m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	if m.cursor != 2 {
		t.Errorf("expected cursor=2, got %d", m.cursor)
	}
}

func TestUpdate_ArrowDownBoundary(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a"}, {Name: "plugin-b"}}, nil, true)
	m.cursor = 1

	newModel, _ := m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor to stay at 1 (boundary), got %d", m.cursor)
	}
}

func TestUpdate_ArrowUp(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a"}, {Name: "plugin-b"}, {Name: "plugin-c"}}, nil, true)
	m.cursor = 2

	newModel, _ := m.Update(mockKeyMsg("up"))
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1, got %d", m.cursor)
	}

	newModel, _ = m.Update(mockKeyMsg("up"))
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", m.cursor)
	}
}

func TestUpdate_ArrowUpBoundary(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a"}, {Name: "plugin-b"}}, nil, true)

	newModel, _ := m.Update(mockKeyMsg("up"))
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor to stay at 0 (boundary), got %d", m.cursor)
	}
}

func TestUpdate_VimBindings(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a"}, {Name: "plugin-b"}, {Name: "plugin-c"}}, nil, true)

	newModel, _ := m.Update(mockKeyMsg("j"))
	m = newModel.(Model)
	if m.cursor != 1 {
		t.Errorf("expected cursor=1 after 'j', got %d", m.cursor)
	}

	newModel, _ = m.Update(mockKeyMsg("k"))
	m = newModel.(Model)
	if m.cursor != 0 {
		t.Errorf("expected cursor=0 after 'k', got %d", m.cursor)
	}
}

func TestUpdate_SpaceToggle(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a"}, {Name: "plugin-b"}}, nil, true)

	selections := m.Selections()
	if selections["plugin-a"] {
		t.Error("expected plugin-a to be unselected initially")
	}

	newModel, _ := m.Update(mockKeyMsg("space"))
	m = newModel.(Model)
	selections = m.Selections()
	if !selections["plugin-a"] {
		t.Error("expected plugin-a to be selected after space")
	}

	newModel, _ = m.Update(mockKeyMsg("space"))
	m = newModel.(Model)
	selections = m.Selections()
	if selections["plugin-a"] {
		t.Error("expected plugin-a to be unselected after second space")
	}
}

func TestUpdate_SpaceToggleSingleSelectMode(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: true}, {Name: "plugin-b"}}
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
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: true}, {Name: "plugin-b"}}
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
	m := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
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
	m := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
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
	for _, tt := range []struct {
		key  string
		name string
	}{{"q", "q key"}, {"esc", "esc key"}} {
		m := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
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

func TestSelections_Output(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: true}, {Name: "plugin-b"}, {Name: "plugin-c", InitiallyEnabled: true}}
	m := newTestModel(items, nil, true)

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
	m := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	if m.Cancelled() {
		t.Error("expected Cancelled()=false initially")
	}

	newModel, _ := m.Update(mockKeyMsg("ctrl+c"))
	m = newModel.(Model)
	if !m.Cancelled() {
		t.Error("expected Cancelled()=true after ctrl+c")
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

func TestView_RenderPluginWithoutSourceLabel(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "oh-my-opencode", InitiallyEnabled: true, SourceLabel: ""}}, nil, true).View().Content

	expected := stylePluginRow("> ✔  oh-my-opencode", true, true)
	if !strings.Contains(view, expected) {
		t.Fatalf("expected plugin row without source label %q in %q", expected, view)
	}
	if strings.Contains(view, "[]") {
		t.Fatalf("expected no empty brackets for missing source label, got %q", view)
	}
}

func TestView_RenderPluginWithUserSourceLabel(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "oh-my-opencode", InitiallyEnabled: true, SourceLabel: "User"}}, nil, true).View().Content

	labelPart := "[User]"
	if !strings.Contains(view, "oh-my-opencode") || !strings.Contains(view, labelPart) {
		t.Fatalf("expected plugin name and source label [User] in %q", view)
	}
	if !strings.Contains(view, dimmedLabelStyle.Render(labelPart)) {
		t.Fatalf("expected source label to be dimmed-styled, got %q", view)
	}
}

func TestView_RenderPluginWithUserProjectSourceLabel(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "oh-my-opencode", InitiallyEnabled: true, SourceLabel: "User, Project"}}, nil, true).View().Content

	labelPart := "[User, Project]"
	if !strings.Contains(view, "oh-my-opencode") || !strings.Contains(view, labelPart) {
		t.Fatalf("expected plugin name and source label [User, Project] in %q", view)
	}
	if !strings.Contains(view, dimmedLabelStyle.Render(labelPart)) {
		t.Fatalf("expected source label to be dimmed-styled, got %q", view)
	}
}

func TestView_SourceLabelPlacedAfterPluginName(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a"}, {Name: "plugin-b", SourceLabel: "User"}, {Name: "plugin-c", SourceLabel: "User, Project"}}
	model := newTestModel(items, nil, true)
	view := model.View().Content
	lines := strings.Split(view, "\n")

	pluginALine := ""
	pluginBLine := ""
	pluginCLine := ""
	for _, line := range lines {
		if strings.Contains(line, "plugin-a") {
			pluginALine = line
		} else if strings.Contains(line, "plugin-b") {
			pluginBLine = line
		} else if strings.Contains(line, "plugin-c") {
			pluginCLine = line
		}
	}

	if strings.Contains(pluginALine, "[]") {
		t.Fatalf("expected plugin-a to have no brackets, got %q", pluginALine)
	}
	if !strings.Contains(pluginBLine, "plugin-b") || !strings.Contains(pluginBLine, "[User]") {
		t.Fatalf("expected plugin-b to show [User] label, got %q", pluginBLine)
	}
	if !strings.Contains(pluginCLine, "plugin-c") || !strings.Contains(pluginCLine, "[User, Project]") {
		t.Fatalf("expected plugin-c to show [User, Project] label, got %q", pluginCLine)
	}
}

func TestView_RendersFocusedSelectedRowLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: true}}, nil, true).View().Content
	expected := stylePluginRow("> ✔  plugin-a", true, true)

	if !strings.Contains(view, expected) {
		t.Fatalf("expected row line %q in %q", expected, view)
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
	helpLine := renderHelpLine(maxLayoutWidth)
	for _, token := range []string{"↑/↓", "space", "enter", "s", "c", "q"} {
		if !strings.Contains(helpLine, helpBgKeyStyle.Render(token)) {
			t.Fatalf("expected styled help token %q in %q", token, helpLine)
		}
	}
	for _, action := range []string{"navigate", "toggle", "confirm", "sessions", "config", "quit"} {
		if !strings.Contains(helpLine, action) || strings.Contains(helpLine, helpBgKeyStyle.Render(action)) {
			t.Fatalf("expected unstyled action %q in %q", action, helpLine)
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
	expected := renderSectionHeader("📋 Choose plugins", maxLayoutWidth)
	if !strings.Contains(view, expected) {
		t.Fatalf("expected plugin prompt line %q in %q", expected, view)
	}
}

func TestViewLauncher_MatchesDefaultView(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true)
	if got, want := m.viewLauncher().Content, m.View().Content; got != want {
		t.Fatalf("expected launcher helper to match default view\nhelper: %q\nview:   %q", got, want)
	}
}

func TestView_RendersStyledHelpLine(t *testing.T) {
	view := newTestModel([]PluginItem{{Name: "plugin-a"}}, nil, true).View().Content
	if !strings.Contains(view, renderHelpLine(maxLayoutWidth)) {
		t.Fatalf("expected help line %q in %q", renderHelpLine(maxLayoutWidth), view)
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
