package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
	"github.com/kayden-kim/oc/internal/tui"
)

func TestRunWithDeps_RefreshesFromManualSelectionToLatestSessionAcrossLoop(t *testing.T) {
	tmp := t.TempDir()
	setupConfigFiles(t, tmp, "{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n", "")

	r := &fakeRunner{}
	call := 0
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		Getwd:             func() (string, error) { return tmp, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) {
			if call == 0 {
				return []tui.SessionItem{{ID: "ses_latest", Title: "Latest session"}, {ID: "ses_manual", Title: "Manual session"}}, nil
			}
			return []tui.SessionItem{{ID: "ses_after", Title: "After session"}, {ID: "ses_manual", Title: "Manual session"}}, nil
		},
		RunTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice, sessions []tui.SessionItem, session tui.SessionItem, _ stats.Report, _ stats.Report, _ config.StatsConfig, _ string, _ bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
			call++
			switch call {
			case 1:
				if session.ID != "ses_latest" {
					t.Fatalf("expected first TUI call to default to latest session, got %+v", session)
				}
				return map[string]bool{"plugin-a": true}, false, "", nil, tui.SessionItem{ID: "ses_manual", Title: "Manual session"}, nil
			case 2:
				if session.ID != "ses_after" {
					t.Fatalf("expected second TUI call to refresh to latest session, got %+v", session)
				}
				return nil, true, "", nil, session, nil
			default:
				t.Fatalf("unexpected TUI call %d", call)
				return nil, false, "", nil, tui.SessionItem{}, nil
			}
		},
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 2 || r.args[0] != "-s" || r.args[1] != "ses_manual" {
		t.Fatalf("expected first launch to use manually selected session args, got %#v", r.args)
	}
}

func TestRefreshSelectedSession_UsesCurrentProjectPath(t *testing.T) {
	projectPath := filepath.Join("workspace", "current-project")
	calledWith := ""
	got := refreshSelectedSession(RuntimeDeps{
		ListSessions: func(cwd string) ([]tui.SessionItem, error) {
			calledWith = cwd
			if cwd != projectPath {
				return nil, fmt.Errorf("unexpected cwd %q", cwd)
			}
			return []tui.SessionItem{{ID: "ses_project_latest", Title: "Project latest"}}, nil
		},
	}, projectPath, tui.SessionItem{ID: "ses_previous", Title: "Previous"})

	if calledWith != projectPath {
		t.Fatalf("expected refreshSelectedSession to use current project path %q, got %q", projectPath, calledWith)
	}
	if got.ID != "ses_project_latest" {
		t.Fatalf("expected latest session for current project, got %+v", got)
	}
}

func TestRunWithDeps_UserProvidedSessionFlagWins(t *testing.T) {
	tmp := t.TempDir()
	setupConfigFiles(t, tmp, "{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n", "")

	r := &fakeRunner{}
	err := RunWithDeps([]string{"--session", "ses_manual"}, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		Getwd:             func() (string, error) { return tmp, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) {
			return []tui.SessionItem{{ID: "ses_latest", Title: "Latest session"}}, nil
		},
		RunTUI: scriptedTUI(t,
			tuiResponse{selections: map[string]bool{"plugin-a": true}},
			tuiResponse{cancelled: true},
		),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 2 || r.args[0] != "--session" || r.args[1] != "ses_manual" {
		t.Fatalf("expected manual session args to win, got %#v", r.args)
	}
}

func TestRunWithDeps_SessionListFailureFallsBackToNoSession(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		Getwd:             func() (string, error) { return tmp, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) {
			return nil, errors.New("session list failed")
		},
		RunTUI: scriptedTUI(t,
			tuiResponse{selections: map[string]bool{"plugin-a": true}},
			tuiResponse{cancelled: true},
		),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 0 {
		t.Fatalf("expected launch without session args, got %#v", r.args)
	}
}
