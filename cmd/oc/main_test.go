package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/tui"
)

type fakeRunner struct {
	checkErr error
	runErr   error
	ran      bool
	args     []string
}

func (f *fakeRunner) CheckAvailable() error {
	return f.checkErr
}

func (f *fakeRunner) Run(args []string) error {
	f.ran = true
	f.args = append([]string(nil), args...)
	return f.runErr
}

func TestRunWithDeps_FullHappyPath(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	initial := "{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\",\n    \"plugin-c\"\n  ]\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	calledTUI := false

	err := runWithDeps([]string{"--model", "gpt-5"}, runtimeDeps{
		newRunner:    func() runnerAPI { return r },
		userHomeDir:  func() (string, error) { return tmp, nil },
		readFile:     os.ReadFile,
		loadOcConfig: config.LoadOcConfig,
		parsePlugins: config.ParsePlugins,
		filterByWhitelist: func(p []config.Plugin, w []string) ([]config.Plugin, []config.Plugin) {
			return defaultDeps().filterByWhitelist(p, w)
		},
		runTUI: func(items []tui.PluginItem) (map[string]bool, bool, error) {
			calledTUI = true
			if len(items) != 3 {
				t.Fatalf("expected 3 visible items, got %d", len(items))
			}
			return map[string]bool{"plugin-a": true, "plugin-b": true, "plugin-c": false}, false, nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	if !calledTUI {
		t.Fatal("expected TUI to be called")
	}
	if !r.ran {
		t.Fatal("expected runner.Run to be called")
	}
	if len(r.args) != 2 || r.args[0] != "--model" || r.args[1] != "gpt-5" {
		t.Fatalf("args mismatch: %#v", r.args)
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(updated)
	if !strings.Contains(out, "\"plugin-a\",") {
		t.Fatalf("plugin-a should be enabled, got:\n%s", out)
	}
	if !strings.Contains(out, "\"plugin-b\",") || strings.Contains(out, "// \"plugin-b\"") {
		t.Fatalf("plugin-b should be enabled, got:\n%s", out)
	}
	if !strings.Contains(out, "// \"plugin-c\"") {
		t.Fatalf("plugin-c should be disabled, got:\n%s", out)
	}
}

func TestRunWithDeps_MissingOpencodeJSON(t *testing.T) {
	tmp := t.TempDir()
	r := &fakeRunner{}

	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI:            defaultDeps().runTUI,
		applySelections:   config.ApplySelections,
		writeConfigFile:   config.WriteConfigFile,
	})
	if err == nil {
		t.Fatal("expected missing file error")
	}
	if !strings.Contains(err.Error(), "opencode.json not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ran {
		t.Fatal("runner.Run should not be called")
	}
}

func TestRunWithDeps_EmptyPluginArraySkipsTUI(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	initial := "{\n  \"plugin\": []\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	tuiCalled := false
	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: func(items []tui.PluginItem) (map[string]bool, bool, error) {
			tuiCalled = true
			return map[string]bool{}, false, nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if tuiCalled {
		t.Fatal("TUI should be skipped when visible list is empty")
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != initial {
		t.Fatalf("config should not change for empty plugin array\nwant:\n%s\ngot:\n%s", initial, string(updated))
	}
}

func TestRunWithDeps_WhitelistFiltersVisibleAndPreservesHidden(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	initial := "{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\",\n    // \"plugin-hidden\"\n  ]\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	ocConfigPath := filepath.Join(tmp, ".oc")
	if err := os.WriteFile(ocConfigPath, []byte("plugins = [\"plugin-a\", \"plugin-b\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: func(items []tui.PluginItem) (map[string]bool, bool, error) {
			if len(items) != 2 {
				t.Fatalf("expected only whitelisted plugins in TUI, got %d", len(items))
			}
			if items[0].Name != "plugin-a" || items[1].Name != "plugin-b" {
				t.Fatalf("unexpected visible plugins: %+v", items)
			}
			return map[string]bool{"plugin-a": false, "plugin-b": true}, false, nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(updated)
	if !strings.Contains(out, "// \"plugin-a\"") {
		t.Fatalf("plugin-a should be disabled, got:\n%s", out)
	}
	if !strings.Contains(out, "\"plugin-b\"") || strings.Contains(out, "// \"plugin-b\"") {
		t.Fatalf("plugin-b should remain enabled, got:\n%s", out)
	}
	if !strings.Contains(out, "// \"plugin-hidden\"") {
		t.Fatalf("hidden plugin should remain unchanged, got:\n%s", out)
	}
}

func TestRunWithDeps_CancelledTUIDoesNotModifyFile(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	initial := "{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: func(items []tui.PluginItem) (map[string]bool, bool, error) {
			return nil, true, nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if r.ran {
		t.Fatal("runner should not execute when TUI is cancelled")
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != initial {
		t.Fatalf("file should remain unchanged on cancel\nwant:\n%s\ngot:\n%s", initial, string(updated))
	}
}

func TestRunWithDeps_CheckAvailableError(t *testing.T) {
	checkErr := errors.New("missing opencode")
	r := &fakeRunner{checkErr: checkErr}

	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       os.UserHomeDir,
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI:            defaultDeps().runTUI,
		applySelections:   config.ApplySelections,
		writeConfigFile:   config.WriteConfigFile,
	})
	if err == nil {
		t.Fatal("expected availability error")
	}
	if !strings.Contains(err.Error(), "opencode not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
