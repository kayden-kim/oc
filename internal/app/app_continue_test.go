package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/stats"
	"github.com/kayden-kim/oc/internal/tui"
)

func TestRunWithDeps_UserProvidedContinueEqualsFlagWins(t *testing.T) {
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
	err := RunWithDeps([]string{"--continue=ses_manual"}, RuntimeDeps{
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
	if len(r.args) != 3 || r.args[0] != "--continue=ses_manual" || r.args[1] != "--port" || r.args[2] == "" {
		t.Fatalf("expected continue flag to win and fast path to add port args, got %#v", r.args)
	}
}

func TestRunWithDeps_ContinueSkipsSessionDiscoveryAndTUI(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "opencode.json"), []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	listSessionsCalled := false
	runTUICalled := false
	err := RunWithDeps([]string{"--continue=ses_manual"}, RuntimeDeps{
		NewRunner:    func() RunnerAPI { return r },
		UserHomeDir:  func() (string, error) { return tmp, nil },
		ReadFile:     os.ReadFile,
		LoadOcConfig: config.LoadOcConfig,
		ListSessions: func(string) ([]tui.SessionItem, error) {
			listSessionsCalled = true
			return []tui.SessionItem{{ID: "ses_latest", Title: "Latest session"}}, nil
		},
		RunTUI: func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, stats.Report, stats.Report, config.StatsConfig, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
			runTUICalled = true
			return nil, false, "", nil, tui.SessionItem{}, nil
		},
		SendToast: func(_ context.Context, _ int, _ []string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if listSessionsCalled {
		t.Fatal("--continue should skip session discovery")
	}
	if runTUICalled {
		t.Fatal("--continue should skip launcher TUI")
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) < 3 || r.args[0] != "--continue=ses_manual" || r.args[1] != "--port" || r.args[2] == "" {
		t.Fatalf("expected continue fast path to preserve continue arg and add port, got %#v", r.args)
	}
}
