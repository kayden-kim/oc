package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNewLaunchModelCopiesPlugins(t *testing.T) {
	plugins := []string{"plugin-a", "plugin-b"}
	m := NewLaunchModel(plugins, testVersion, nil)
	plugins[0] = "mutated"

	view := m.View().Content
	if strings.Contains(view, "mutated") {
		t.Fatalf("expected launch model to keep original plugins, got %q", view)
	}
}

func TestLaunchModelInitWithoutExecutorReturnsNil(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a"}, testVersion, nil)
	if cmd := m.Init(); cmd != nil {
		t.Fatal("expected nil init command when executor is absent")
	}
}

func TestLaunchModelViewRendersPluginsAndPlaceholder(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a", "plugin-b"}, testVersion, nil)
	view := m.View().Content

	for _, want := range []string{
		Model{version: testVersion}.renderHeader(),
		"⠋ Launching opencode",
		"Plugins",
		"  - plugin-a",
		"  - plugin-b",
		"Preparing launch...",
		"Progress",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected view to contain %q, got %q", want, view)
		}
	}
}

func TestLaunchModelViewRendersNoPlugins(t *testing.T) {
	m := NewLaunchModel(nil, testVersion, nil)
	if view := m.View().Content; !strings.Contains(view, "No selectable plugins in this view; continuing with the current configuration.") {
		t.Fatalf("expected empty plugin copy in view, got %q", view)
	}
}

func TestLaunchModelTickAdvancesSpinner(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a"}, testVersion, nil)
	updated, cmd := m.Update(launchTickMsg{})
	launchModel := updated.(LaunchModel)

	if cmd == nil {
		t.Fatal("expected tick command after spinner update")
	}
	if view := launchModel.View().Content; !strings.Contains(view, "⠙ Launching opencode") {
		t.Fatalf("expected spinner frame to advance, got %q", view)
	}
}

func TestLaunchModelSpinnerWrapsAround(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a"}, testVersion, nil)
	updated := tea.Model(m)
	for i := 0; i < len(launchFrames); i++ {
		var cmd tea.Cmd
		updated, cmd = updated.Update(launchTickMsg{})
		if cmd == nil {
			t.Fatal("expected tick command while advancing spinner")
		}
	}

	if view := updated.(LaunchModel).View().Content; !strings.Contains(view, "⠋ Launching opencode") {
		t.Fatalf("expected spinner to wrap to first frame, got %q", view)
	}
}

func TestLaunchModelUpdateAppendsLogs(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a"}, testVersion, nil)
	updated, cmd := m.Update(LaunchLogMsg{Line: "Using port 51234"})
	launchModel := updated.(LaunchModel)

	if cmd == nil {
		t.Fatal("expected wait command after launch log")
	}
	if view := launchModel.View().Content; !strings.Contains(view, "Using port 51234") {
		t.Fatalf("expected log line in view, got %q", view)
	}
}

func TestLaunchModelViewEmphasizesNewestLogLine(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a"}, testVersion, nil)
	updated, _ := m.Update(LaunchLogMsg{Line: "checking config"})
	updated, _ = updated.(LaunchModel).Update(LaunchLogMsg{Line: "resolving port"})
	updated, _ = updated.(LaunchModel).Update(LaunchLogMsg{Line: "Using port 51234"})
	view := updated.(LaunchModel).View().Content

	if !strings.Contains(view, launchLogOldStyle.Render("checking config")) {
		t.Fatalf("expected oldest log to use old style, got %q", view)
	}
	if !strings.Contains(view, launchLogMidStyle.Render("resolving port")) {
		t.Fatalf("expected middle log to use mid style, got %q", view)
	}
	if !strings.Contains(view, launchLogNewStyle.Render("Using port 51234")) {
		t.Fatalf("expected newest log to use new style, got %q", view)
	}
}

func TestLaunchModelViewSingleLogLineIsBright(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a"}, testVersion, nil)
	updated, _ := m.Update(LaunchLogMsg{Line: "Using port 51234"})
	view := updated.(LaunchModel).View().Content

	if !strings.Contains(view, launchLogNewStyle.Render("Using port 51234")) {
		t.Fatalf("expected single log line to use new style, got %q", view)
	}
}

func TestLaunchModelUpdateReadyQuitsAndStoresPortArgs(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a"}, testVersion, nil)
	updated, cmd := m.Update(LaunchReadyMsg{PortArgs: []string{"--port", "51234"}})
	launchModel := updated.(LaunchModel)

	if cmd == nil || cmd() != tea.Quit() {
		t.Fatal("expected tea.Quit command after launch ready")
	}
	args := launchModel.PortArgs()
	if len(args) != 2 || args[0] != "--port" || args[1] != "51234" {
		t.Fatalf("expected port args to be stored, got %#v", args)
	}
	args[0] = "mutated"
	if got := launchModel.PortArgs()[0]; got != "--port" {
		t.Fatalf("expected stored args to be copied, got %q", got)
	}
}

func TestLaunchModelIgnoresKeyPresses(t *testing.T) {
	m := NewLaunchModel([]string{"plugin-a"}, testVersion, nil)
	updated, cmd := m.Update(mockKeyMsg("q"))

	if cmd != nil {
		t.Fatal("expected no command for key presses")
	}
	if view := updated.(LaunchModel).View().Content; !strings.Contains(view, "Preparing launch...") {
		t.Fatalf("expected view to remain unchanged, got %q", view)
	}
}
