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

type fakeEditor struct {
	openErr error
	opened  bool
	path    string
}

func (f *fakeRunner) CheckAvailable() error {
	return f.checkErr
}

func (f *fakeRunner) Run(args []string) error {
	f.ran = true
	f.args = append([]string(nil), args...)
	return f.runErr
}

func (f *fakeEditor) Open(path string) error {
	f.opened = true
	f.path = path
	return f.openErr
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
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
			calledTUI = true
			if len(items) != 3 {
				t.Fatalf("expected 3 visible items, got %d", len(items))
			}
			if len(editChoices) != 3 {
				t.Fatalf("expected 3 edit choices, got %d", len(editChoices))
			}
			return map[string]bool{"plugin-a": true, "plugin-b": true, "plugin-c": false}, false, "", nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string) error { return nil },
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
		openEditor:        func(string) error { return nil },
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
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
			tuiCalled = true
			return map[string]bool{}, false, "", nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string) error { return nil },
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
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
			if len(items) != 2 {
				t.Fatalf("expected only whitelisted plugins in TUI, got %d", len(items))
			}
			if items[0].Name != "plugin-a" || items[1].Name != "plugin-b" {
				t.Fatalf("unexpected visible plugins: %+v", items)
			}
			return map[string]bool{"plugin-a": false, "plugin-b": true}, false, "", nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string) error { return nil },
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
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
			return nil, true, "", nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string) error { return nil },
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
		openEditor:        func(string) error { return nil },
	})
	if err == nil {
		t.Fatal("expected availability error")
	}
	if !strings.Contains(err.Error(), "opencode not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithDeps_EditRequestOpensEditorAndExits(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	initial := "{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	ed := &fakeEditor{}

	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
			return nil, false, configPath, nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      ed.Open,
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !ed.opened {
		t.Fatal("expected editor to be opened")
	}
	if ed.path != configPath {
		t.Fatalf("expected editor path %q, got %q", configPath, ed.path)
	}
	if r.ran {
		t.Fatal("runner should not execute when edit is requested")
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != initial {
		t.Fatalf("file should remain unchanged on edit request\nwant:\n%s\ngot:\n%s", initial, string(updated))
	}
}

func TestRunWithDeps_PassesResolvedEditChoicesToTUI(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
	if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	var gotChoices []tui.EditChoice

	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
			gotChoices = append([]tui.EditChoice(nil), editChoices...)
			return nil, false, filepath.Join(tmp, ".oc"), nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	if len(gotChoices) != 3 {
		t.Fatalf("expected 3 edit choices, got %d", len(gotChoices))
	}
	if gotChoices[0].Path != filepath.Join(tmp, ".oc") {
		t.Fatalf("expected first choice to target .oc, got %q", gotChoices[0].Path)
	}
	if gotChoices[1].Path != configPath {
		t.Fatalf("expected second choice to target opencode.json, got %q", gotChoices[1].Path)
	}
	if gotChoices[2].Path != jsoncPath {
		t.Fatalf("expected third choice to target oh-my jsonc, got %q", gotChoices[2].Path)
	}
	if r.ran {
		t.Fatal("runner should not execute when edit is requested")
	}
}

func TestResolveOhMyOpencodePath(t *testing.T) {
	t.Run("prefers json when present", func(t *testing.T) {
		configDir := t.TempDir()
		jsonPath := filepath.Join(configDir, "oh-my-opencode.json")
		jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
		if err := os.WriteFile(jsonPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := resolveOhMyOpencodePath(configDir); got != jsonPath {
			t.Fatalf("expected %q, got %q", jsonPath, got)
		}
	})

	t.Run("falls back to jsonc", func(t *testing.T) {
		configDir := t.TempDir()
		jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
		if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := resolveOhMyOpencodePath(configDir); got != jsoncPath {
			t.Fatalf("expected %q, got %q", jsoncPath, got)
		}
	})

	t.Run("defaults to json path", func(t *testing.T) {
		configDir := t.TempDir()
		want := filepath.Join(configDir, "oh-my-opencode.json")

		if got := resolveOhMyOpencodePath(configDir); got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}
