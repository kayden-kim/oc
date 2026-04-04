package tui

import "testing"

func TestView_RoutesToActiveModeRenderer(t *testing.T) {
	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, []SessionItem{{ID: "ses_latest", Title: "Latest session"}}, SessionItem{}, true)

	if got, want := model.View().Content, model.viewLauncher().Content; got != want {
		t.Fatalf("expected default view to use launcher renderer\nview: %q\nwant: %q", got, want)
	}

	updated, _ := model.Update(mockKeyMsg("c"))
	model = updated.(Model)
	if got, want := model.View().Content, model.viewEditPicker().Content; got != want {
		t.Fatalf("expected edit mode to use edit renderer\nview: %q\nwant: %q", got, want)
	}

	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("s"))
	model = updated.(Model)
	if got, want := model.View().Content, model.viewSessionPicker().Content; got != want {
		t.Fatalf("expected session mode to use session renderer\nview: %q\nwant: %q", got, want)
	}

	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	if got, want := model.View().Content, model.renderStatsView(); got != want {
		t.Fatalf("expected stats mode to use stats renderer\nview: %q\nwant: %q", got, want)
	}
	if !model.statsMode {
		t.Fatal("expected stats mode after tab")
	}
}

func TestUpdate_ModeEscTransitionsReturnToLauncher(t *testing.T) {
	model := newTestModelWithSession([]PluginItem{{Name: "plugin-a"}}, []EditChoice{{Label: ".oc file", Path: "/tmp/.oc"}}, []SessionItem{{ID: "ses_latest", Title: "Latest session"}}, SessionItem{}, true)

	updated, _ := model.Update(mockKeyMsg("c"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	if model.editMode || model.sessionMode || model.statsMode {
		t.Fatalf("expected esc from edit mode to return to launcher, got edit=%v session=%v stats=%v", model.editMode, model.sessionMode, model.statsMode)
	}

	updated, _ = model.Update(mockKeyMsg("s"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	if model.editMode || model.sessionMode || model.statsMode {
		t.Fatalf("expected esc from session mode to return to launcher, got edit=%v session=%v stats=%v", model.editMode, model.sessionMode, model.statsMode)
	}

	updated, _ = model.Update(mockKeyMsg("tab"))
	model = updated.(Model)
	updated, _ = model.Update(mockKeyMsg("esc"))
	model = updated.(Model)
	if model.editMode || model.sessionMode || model.statsMode {
		t.Fatalf("expected esc from stats mode to return to launcher, got edit=%v session=%v stats=%v", model.editMode, model.sessionMode, model.statsMode)
	}
	if got, want := model.View().Content, model.viewLauncher().Content; got != want {
		t.Fatalf("expected launcher view after mode exits\nview: %q\nwant: %q", got, want)
	}
}
