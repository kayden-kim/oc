package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestUpdate_EditRequest(t *testing.T) {
	items := []PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}
	editChoices := []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}
	m := newTestModel(items, editChoices, true)
	newModel, cmd := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	if m.EditRequested() || !m.editMode || m.cancelled || m.confirmed || cmd != nil {
		t.Fatalf("unexpected edit request state after opening edit mode: requested=%v editMode=%v cancelled=%v confirmed=%v cmdNil=%v", m.EditRequested(), m.editMode, m.cancelled, m.confirmed, cmd == nil)
	}
}

func TestUpdate_EditModeEnterSelectsTarget(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}, {Label: "opencode.json", Path: "/tmp/opencode.json"}}, true)
	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	newModel, _ = m.Update(mockKeyMsg("down"))
	m = newModel.(Model)
	newModel, cmd := m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)
	if !m.EditRequested() || m.EditTarget() != "/tmp/opencode.json" || cmd == nil || cmd() != tea.Quit() {
		t.Fatalf("expected edit target selection to request /tmp/opencode.json and quit, got requested=%v target=%q", m.EditRequested(), m.EditTarget())
	}
}

func TestUpdate_EditModeEscReturnsToPluginList(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	newModel, cmd := m.Update(mockKeyMsg("esc"))
	m = newModel.(Model)
	if m.editMode || m.Cancelled() || cmd != nil {
		t.Fatalf("expected esc to leave edit mode without cancelling, got editMode=%v cancelled=%v cmd=%v", m.editMode, m.Cancelled(), cmd)
	}
}

func TestEditRequested_Method(t *testing.T) {
	m := newTestModel([]PluginItem{{Name: "plugin-a", InitiallyEnabled: false}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	if m.EditRequested() {
		t.Fatal("expected EditRequested()=false initially")
	}
	newModel, _ := m.Update(mockKeyMsg("c"))
	m = newModel.(Model)
	if m.EditRequested() {
		t.Fatal("expected EditRequested()=false while picker is open")
	}
	newModel, _ = m.Update(mockKeyMsg("enter"))
	m = newModel.(Model)
	if !m.EditRequested() || m.EditTarget() != "/tmp/.oc" {
		t.Fatalf("expected edit target /tmp/.oc after selection, got requested=%v target=%q", m.EditRequested(), m.EditTarget())
	}
}

func TestView_EditModeRendersInstructionPrompt(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	view := updatedModel.(Model).View().Content
	expected := renderSectionHeader("📂 Choose config to edit", maxLayoutWidth)
	if !strings.Contains(view, expected) {
		t.Fatalf("expected edit prompt line %q in %q", expected, view)
	}
}

func TestViewEditPicker_MatchesEditModeView(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	m := updatedModel.(Model)
	if got, want := m.viewEditPicker().Content, m.View().Content; got != want {
		t.Fatalf("expected edit helper to match edit mode view\nhelper: %q\nview:   %q", got, want)
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

func TestView_ClearsOnEditSelection(t *testing.T) {
	model := newTestModel([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, true)
	updatedModel, _ := model.Update(mockKeyMsg("c"))
	updatedModel, _ = updatedModel.(Model).Update(mockKeyMsg("enter"))
	if got := updatedModel.(Model).View().Content; got != "" {
		t.Fatalf("expected empty view after edit selection, got %q", got)
	}
}
