package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/tui"
)

type fakeRunner struct {
	checkErr error
	runErr   error
	ran      bool
	args     []string
}

type fakeEditor struct {
	openErr      error
	opened       bool
	path         string
	configEditor string
}

func (f *fakeRunner) CheckAvailable() error {
	return f.checkErr
}

func (f *fakeRunner) Run(args []string) error {
	f.ran = true
	f.args = append([]string(nil), args...)
	return f.runErr
}

func (f *fakeEditor) Open(path string, configEditor string) error {
	f.opened = true
	f.path = path
	f.configEditor = configEditor
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
		openEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !calledTUI {
		t.Fatal("expected TUI to be called")
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 2 || r.args[0] != "--model" || r.args[1] != "gpt-5" {
		t.Fatalf("unexpected runner args: %#v", r.args)
	}

	updated, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	out := string(updated)
	if !strings.Contains(out, "\"plugin-b\"") || strings.Contains(out, "// \"plugin-b\"") {
		t.Fatalf("plugin-b should be enabled, got:\n%s", out)
	}
	if !strings.Contains(out, "// \"plugin-c\"") {
		t.Fatalf("plugin-c should be disabled, got:\n%s", out)
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
		openEditor:      func(string, string) error { return nil },
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
		openEditor:      func(string, string) error { return nil },
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
		openEditor:      func(string, string) error { return nil },
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
		openEditor:        func(string, string) error { return nil },
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
	if ed.configEditor != "" {
		t.Fatalf("expected empty config editor, got %q", ed.configEditor)
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

func TestRunWithDeps_PassesOcConfigEditorToEditor(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ocConfigPath := filepath.Join(tmp, ".oc")
	if err := os.WriteFile(ocConfigPath, []byte("editor = \"code --goto\"\n"), 0o644); err != nil {
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
	if ed.configEditor != "code --goto" {
		t.Fatalf("expected config editor to be passed through, got %q", ed.configEditor)
	}
	if r.ran {
		t.Fatal("runner should not execute when edit is requested")
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
		openEditor:      func(string, string) error { return nil },
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

// --- Port selection tests ---

func baseDepsWithPort(tmp string, r *fakeRunner) runtimeDeps {
	return runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
			return map[string]bool{}, false, "", nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string, string) error { return nil },
		parsePortRange:  port.ParseRange,
		selectPort: func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, p int, available bool)) port.SelectResult {
			return port.SelectResult{Port: 51234, Attempts: 1, Found: true}
		},
		isPortAvailable: func(int) bool { return true },
	}
}

func setupPortTestFiles(t *testing.T, tmp string, pluginContent string, ocContent string) {
	t.Helper()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configDir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(pluginContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if ocContent != "" {
		ocConfigPath := filepath.Join(tmp, ".oc")
		if err := os.WriteFile(ocConfigPath, []byte(ocContent), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRunWithDeps_PortSelectionAddsPortFlag(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n",
		"plugins = [\"plugin-a\"]\nports = \"50000-55000\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
		return map[string]bool{"plugin-a": true}, false, "", nil
	}
	deps.selectPort = func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, p int, available bool)) port.SelectResult {
		if minPort != 50000 || maxPort != 55000 {
			t.Fatalf("expected range 50000-55000, got %d-%d", minPort, maxPort)
		}
		return port.SelectResult{Port: 51234, Attempts: 1, Found: true}
	}

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	// Args should include original args plus --port 51234
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
		"{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n",
		"plugins = [\"plugin-a\"]\nports = \"50000-55000\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
		return map[string]bool{"plugin-a": true}, false, "", nil
	}
	deps.selectPort = func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, p int, available bool)) port.SelectResult {
		return port.SelectResult{Found: false, Attempts: 15}
	}

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called even when port selection fails")
	}
	// Args should NOT include --port
	if len(r.args) != 2 {
		t.Fatalf("expected 2 args (no --port), got %d: %v", len(r.args), r.args)
	}
}

func TestRunWithDeps_NoPortsConfigNoPortFlag(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n",
		"plugins = [\"plugin-a\"]\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
		return map[string]bool{"plugin-a": true}, false, "", nil
	}

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	// No --port should be added
	if len(r.args) != 2 {
		t.Fatalf("expected 2 args (no --port), got %d: %v", len(r.args), r.args)
	}
}

func TestRunWithDeps_InvalidPortsConfigFallback(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n",
		"plugins = [\"plugin-a\"]\nports = \"invalid\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
		return map[string]bool{"plugin-a": true}, false, "", nil
	}

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called even with invalid port config")
	}
	// No --port should be added
	if len(r.args) != 2 {
		t.Fatalf("expected 2 args (no --port), got %d: %v", len(r.args), r.args)
	}
}

func TestRunWithDeps_PortSelectionSkipsTUIPath(t *testing.T) {
	// When there are no visible plugins (TUI is skipped), port selection should still work
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": []\n}\n",
		"ports = \"50000-55000\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.selectPort = func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, p int, available bool)) port.SelectResult {
		return port.SelectResult{Port: 52000, Attempts: 1, Found: true}
	}

	err := runWithDeps([]string{"--verbose"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 3 || r.args[1] != "--port" || r.args[2] != "52000" {
		t.Fatalf("expected --port 52000 appended, got %v", r.args)
	}
}
