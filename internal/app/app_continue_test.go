package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/stats"
	"github.com/kayden-kim/oc/internal/tui"
)

func TestRunContinuePath_LoadsOcConfigAndUsesResolvedPortArgs(t *testing.T) {
	r := &fakeRunner{}
	loadCalls := 0
	parsedRange := ""

	err := runContinuePath([]string{"--continue=ses_manual"}, RuntimeDeps{
		LoadOcConfig: func(path string) (*config.OcConfig, error) {
			loadCalls++
			return &config.OcConfig{Ports: "60000-60010"}, nil
		},
		ParsePortRange: func(raw string) (int, int, error) {
			parsedRange = raw
			return 60000, 60010, nil
		},
		SelectPort: func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(int, int, bool)) port.SelectResult {
			return port.SelectResult{Port: 60005, Attempts: 1, Found: true}
		},
		IsPortAvailable: func(int) bool { return true },
	}, resolveRuntimePaths(t.TempDir()), r)
	if err != nil {
		t.Fatalf("runContinuePath returned error: %v", err)
	}
	if loadCalls != 1 {
		t.Fatalf("expected LoadOcConfig to be called once, got %d", loadCalls)
	}
	if parsedRange != "60000-60010" {
		t.Fatalf("expected continue path to parse ports from oc config, got %q", parsedRange)
	}
	if !r.ran {
		t.Fatal("expected runner to execute in continue path")
	}
	if len(r.args) != 3 || r.args[0] != "--continue=ses_manual" || r.args[1] != "--port" || r.args[2] != "60005" {
		t.Fatalf("unexpected continue args: %#v", r.args)
	}
	if r.runCalls != 1 {
		t.Fatalf("expected runner to execute once, got %d", r.runCalls)
	}
}

func TestRunContinuePath_LoadOcConfigErrorMatchesExistingWrap(t *testing.T) {
	want := errors.New("bad config")
	err := runContinuePath(nil, RuntimeDeps{
		LoadOcConfig: func(string) (*config.OcConfig, error) {
			return nil, want
		},
	}, resolveRuntimePaths(t.TempDir()), &fakeRunner{})
	if !errors.Is(err, want) {
		t.Fatalf("expected wrapped load error, got %v", err)
	}
	if err == nil || err.Error() != "failed to load whitelist: bad config" {
		t.Fatalf("expected preserved error text, got %v", err)
	}
}

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
