package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/stats"
	"github.com/kayden-kim/oc/internal/tui"
)

func baseDepsWithPort(tmp string, r *fakeRunner) RuntimeDeps {
	return RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			if r.runCalls > 0 {
				return nil, true, "", nil, nil
			}
			return map[string]bool{}, false, "", []string{"--port", "51234"}, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
		ParsePortRange:  port.ParseRange,
		SelectPort: func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, p int, available bool)) port.SelectResult {
			return port.SelectResult{Port: 51234, Attempts: 1, Found: true}
		},
		IsPortAvailable: func(int) bool { return true },
		SendToast:       func(_ context.Context, _ int, _ []string) error { return nil },
	}
}

func setupPortTestFiles(t *testing.T, tmp string, pluginContent string, ocContent string) {
	t.Helper()
	setupConfigFiles(t, tmp, pluginContent, ocContent)
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

func TestRunWithDeps_PortSelectionAddsPortFlag(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "50000-55000" {
			t.Fatalf("expected ports range to reach TUI, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", []string{"--port", "51234"}, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(r.args), r.args)
	}
	if r.args[2] != "--port" || r.args[3] != "51234" {
		t.Fatalf("expected --port 51234 appended, got %v", r.args)
	}
}

func TestRunWithDeps_PortSelectionFailsFallback(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "50000-55000" {
			t.Fatalf("expected ports range to reach TUI, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called even when port selection fails")
	}
	if len(r.args) != 2 {
		t.Fatalf("expected 2 args (no --port), got %d: %v", len(r.args), r.args)
	}
}

func TestRunWithDeps_NoPortsConfigNoPortFlag(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"plugins = [\"oh-my-opencode\"]\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != DefaultPortsRange {
			t.Fatalf("expected default ports range %q, got %q", DefaultPortsRange, portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 2 {
		t.Fatalf("expected no --port to be appended by the TUI stub, got %v", r.args)
	}
}

func TestRunWithDeps_DefaultPortRangeWhenNoOcConfig(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != DefaultPortsRange {
			t.Fatalf("expected default ports range %q, got %q", DefaultPortsRange, portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if len(r.args) != 2 {
		t.Fatalf("expected no --port to be appended by the TUI stub, got %v", r.args)
	}
}

func TestRunWithDeps_PortSelectionRunsWithoutVisiblePlugins(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": []\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)

	err := RunWithDeps([]string{"--verbose"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 3 || r.args[0] != "--verbose" || r.args[1] != "--port" || r.args[2] != "51234" {
		t.Fatalf("expected --port to be appended even without visible plugins, got %v", r.args)
	}
}

func TestRunWithDeps_UsesOcSectionPortsForSingleVisiblePlugin(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode@latest\",\n    \"superpowers\"\n  ]\n}\n",
		"[oc]\nplugins = [\"oh-my-opencode\"]\nports = \"55000-55500\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "55000-55500" {
			t.Fatalf("expected [oc] ports to reach TUI, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode@latest": true}, false, "", []string{"--port", "51234"}, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 4 || r.args[2] != "--port" || r.args[3] != "51234" {
		t.Fatalf("expected [oc] port to be appended, got %v", r.args)
	}
}

func TestRunWithDeps_PortSelectionStillAppliesWhenOhMyOpencodeNotSelected(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\",\n    \"superpowers\"\n  ]\n}\n",
		"[oc]\nports = \"55000-55500\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "55000-55500" {
			t.Fatalf("expected [oc] ports to reach TUI, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": false, "superpowers": true}, false, "", []string{"--port", "51234"}, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 4 || r.args[2] != "--port" || r.args[3] != "51234" {
		t.Fatalf("expected port to be preserved even when oh-my-opencode is not selected, got %v", r.args)
	}
}

func TestRunWithDeps_IgnoresLegacyPluginSectionPorts(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"[plugin.oh-my-opencode]\nports = \"55000-55500\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != DefaultPortsRange {
			t.Fatalf("expected legacy plugin-section ports to fall back to %q, got %q", DefaultPortsRange, portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if len(r.args) != 2 {
		t.Fatalf("expected no --port to be appended by the TUI stub, got %v", r.args)
	}
}

func TestRunWithDeps_IgnoresTopLevelPortsConfig(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"ports = \"55000-55500\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != DefaultPortsRange {
			t.Fatalf("expected top-level ports to fall back to %q, got %q", DefaultPortsRange, portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if len(r.args) != 2 {
		t.Fatalf("expected no --port to be appended by the TUI stub, got %v", r.args)
	}
}

func TestRunLaunchTUI_ReturnsPortArgsWithoutRenderer(t *testing.T) {
	tmp := t.TempDir()
	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)

	portArgs, err := runLaunchTUI([]string{"oh-my-opencode"}, tui.SessionItem{}, "50000-55000", deps, "test")
	if err != nil {
		t.Fatalf("runLaunchTUI returned error: %v", err)
	}
	if len(portArgs) != 2 || portArgs[0] != "--port" || portArgs[1] != "51234" {
		t.Fatalf("expected --port 51234, got %v", portArgs)
	}
}
