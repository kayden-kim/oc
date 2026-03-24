package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func newTestLaunchModel(plugins []string, session SessionItem, executor launchExecutor) LaunchModel {
	return NewLaunchModel(plugins, session, testVersion, executor)
}

func TestNewLaunchModelCopiesPlugins(t *testing.T) {
	plugins := []string{"plugin-a", "plugin-b"}
	m := newTestLaunchModel(plugins, SessionItem{}, nil)
	plugins[0] = "mutated"

	if got := m.plugins[0]; got != "plugin-a" {
		t.Fatalf("expected launch model to keep original plugins, got %q", got)
	}
}

func TestLaunchModelInitWithoutExecutorReturnsNil(t *testing.T) {
	m := newTestLaunchModel([]string{"plugin-a"}, SessionItem{}, nil)
	if cmd := m.Init(); cmd != nil {
		t.Fatal("expected nil init command when executor is absent")
	}
}

func TestLaunchModelViewAlwaysEmpty(t *testing.T) {
	m := newTestLaunchModel([]string{"plugin-a"}, SessionItem{ID: "ses_latest", Title: "Latest session"}, nil)
	if view := m.View().Content; view != "" {
		t.Fatalf("expected empty launch view, got %q", view)
	}
}

func TestLaunchModelUpdateLogContinuesWaiting(t *testing.T) {
	m := newTestLaunchModel([]string{"plugin-a"}, SessionItem{}, nil)
	m.msgCh = make(chan tea.Msg, 1)
	updated, cmd := m.Update(LaunchLogMsg{Line: "Using port 51234"})

	if cmd == nil {
		t.Fatal("expected wait command after launch log")
	}
	if view := updated.(LaunchModel).View().Content; view != "" {
		t.Fatalf("expected log updates to keep empty view, got %q", view)
	}
}

func TestLaunchModelUpdateReadyStoresPortArgsAndQuits(t *testing.T) {
	m := newTestLaunchModel([]string{"plugin-a"}, SessionItem{}, nil)
	updated, cmd := m.Update(LaunchReadyMsg{PortArgs: []string{"--port", "51234"}})
	launchModel := updated.(LaunchModel)

	if cmd == nil || cmd() != tea.Quit() {
		t.Fatal("expected tea.Quit when ready message arrives")
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
	m := newTestLaunchModel([]string{"plugin-a"}, SessionItem{}, nil)
	updated, cmd := m.Update(mockKeyMsg("q"))

	if cmd != nil {
		t.Fatal("expected no command for key presses")
	}
	if view := updated.(LaunchModel).View().Content; view != "" {
		t.Fatalf("expected empty view to remain unchanged, got %q", view)
	}
}
