package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/tui"
)

type fakeRunner struct {
	checkErr error
	runErr   error
	ran      bool
	runCalls int
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

func (f *fakeRunner) Run(args []string, onStart func()) error {
	f.ran = true
	f.runCalls++
	f.args = append([]string(nil), args...)
	if f.runErr == nil && onStart != nil {
		go onStart()
	}
	return f.runErr
}

type tuiResponse struct {
	selections map[string]bool
	cancelled  bool
	editTarget string
	portArgs   []string
	session    tui.SessionItem
	err        error
}

type tuiFunc func([]tui.PluginItem, []tui.EditChoice, string, bool) (map[string]bool, bool, string, []string, error)

func wrapTUI(fn tuiFunc) func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
	return func(items []tui.PluginItem, editChoices []tui.EditChoice, _ []tui.SessionItem, session tui.SessionItem, portsRange string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		selections, cancelled, editTarget, portArgs, err := fn(items, editChoices, portsRange, allowMultiplePlugins)
		return selections, cancelled, editTarget, portArgs, session, err
	}
}

func scriptedTUI(t *testing.T, responses ...tuiResponse) func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
	t.Helper()
	call := 0
	return func(items []tui.PluginItem, editChoices []tui.EditChoice, _ []tui.SessionItem, session tui.SessionItem, _ string, _ bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		if call >= len(responses) {
			t.Fatalf("unexpected TUI call %d", call+1)
		}
		resp := responses[call]
		call++
		if resp.session.ID != "" {
			session = resp.session
		}
		return resp.selections, resp.cancelled, resp.editTarget, resp.portArgs, session, resp.err
	}
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
	calledTUI := 0
	ocConfigPath := filepath.Join(tmp, ".oc")
	if err := os.WriteFile(ocConfigPath, []byte("allow_multiple_plugins = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runWithDeps([]string{"--model", "gpt-5"}, runtimeDeps{
		newRunner:    func() runnerAPI { return r },
		userHomeDir:  func() (string, error) { return tmp, nil },
		readFile:     os.ReadFile,
		loadOcConfig: config.LoadOcConfig,
		parsePlugins: config.ParsePlugins,
		filterByWhitelist: func(p []config.Plugin, w []string) ([]config.Plugin, []config.Plugin) {
			return defaultDeps().filterByWhitelist(p, w)
		},
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, error) {
			calledTUI++
			if !allowMultiplePlugins {
				t.Fatal("expected allow_multiple_plugins=true from .oc config")
			}
			if len(items) != 3 {
				t.Fatalf("expected 3 visible items, got %d", len(items))
			}
			if len(editChoices) != 3 {
				t.Fatalf("expected 3 edit choices, got %d", len(editChoices))
			}
			if calledTUI == 1 {
				return map[string]bool{"plugin-a": true, "plugin-b": true, "plugin-c": false}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if calledTUI != 2 {
		t.Fatal("expected TUI to be called")
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if r.runCalls != 1 {
		t.Fatalf("expected runner to be called once, got %d", r.runCalls)
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

func TestRunWithDeps_RefreshesFromManualSelectionToLatestSessionAcrossLoop(t *testing.T) {
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
	call := 0
	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		getwd:             func() (string, error) { return tmp, nil },
		listSessions: func(string) ([]tui.SessionItem, error) {
			if call == 0 {
				return []tui.SessionItem{{ID: "ses_latest", Title: "Latest session"}, {ID: "ses_manual", Title: "Manual session"}}, nil
			}
			return []tui.SessionItem{{ID: "ses_after", Title: "After session"}, {ID: "ses_manual", Title: "Manual session"}}, nil
		},
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice, sessions []tui.SessionItem, session tui.SessionItem, _ string, _ bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
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
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 2 || r.args[0] != "-s" || r.args[1] != "ses_manual" {
		t.Fatalf("expected first launch to use manually selected session args, got %#v", r.args)
	}
}

func TestRunWithDeps_UserProvidedSessionFlagWins(t *testing.T) {
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
	err := runWithDeps([]string{"--session", "ses_manual"}, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		getwd:             func() (string, error) { return tmp, nil },
		listSessions: func(string) ([]tui.SessionItem, error) {
			return []tui.SessionItem{{ID: "ses_latest", Title: "Latest session"}}, nil
		},
		runTUI: scriptedTUI(t,
			tuiResponse{selections: map[string]bool{"plugin-a": true}},
			tuiResponse{cancelled: true},
		),
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
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
	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		getwd:             func() (string, error) { return tmp, nil },
		listSessions: func(string) ([]tui.SessionItem, error) {
			return nil, errors.New("session list failed")
		},
		runTUI: scriptedTUI(t,
			tuiResponse{selections: map[string]bool{"plugin-a": true}},
			tuiResponse{cancelled: true},
		),
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 0 {
		t.Fatalf("expected launch without session args, got %#v", r.args)
	}
}

func TestRunWithDeps_DefaultsAllowMultiplePluginsToFalse(t *testing.T) {
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
	seenFlags := []bool{}
	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, error) {
			seenFlags = append(seenFlags, allowMultiplePlugins)
			if r.runCalls == 0 {
				return map[string]bool{"plugin-a": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if len(seenFlags) != 2 {
		t.Fatalf("expected TUI to be shown twice, got %d", len(seenFlags))
	}
	for _, flag := range seenFlags {
		if flag {
			t.Fatal("expected allow_multiple_plugins to default to false")
		}
	}
}

func TestRunWithDeps_DoesNotPrintPreLaunchText(t *testing.T) {
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
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	os.Stdout = writePipe
	os.Stderr = writePipe
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	err = runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: scriptedTUI(t,
			tuiResponse{selections: map[string]bool{"plugin-a": true}},
			tuiResponse{cancelled: true},
		),
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string, string) error { return nil },
	})
	writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	readPipe.Close()
	if readErr != nil {
		t.Fatalf("failed to read captured output: %v", readErr)
	}
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	outputStr := string(output)
	for _, unwanted := range []string{"Selected plugins:", "Port selection:", "Using port", "Launching opencode without --port flag."} {
		if strings.Contains(outputStr, unwanted) {
			t.Fatalf("expected no direct pre-launch output %q, got %q", unwanted, outputStr)
		}
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
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalled = true
			return map[string]bool{}, false, "", nil, nil
		}),
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
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			if len(items) != 2 {
				t.Fatalf("expected only whitelisted plugins in TUI, got %d", len(items))
			}
			if items[0].Name != "plugin-a" || items[1].Name != "plugin-b" {
				t.Fatalf("unexpected visible plugins: %+v", items)
			}
			if r.runCalls == 0 {
				return map[string]bool{"plugin-a": false, "plugin-b": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
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
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			return nil, true, "", nil, nil
		}),
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

func TestRunWithDeps_ContinuesAfterOpencodeExitCode(t *testing.T) {
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

	r := &fakeRunner{runErr: &runner.ExitCodeError{Code: 17}}
	tuiCalls := 0
	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalls++
			if tuiCalls == 1 {
				return map[string]bool{"plugin-a": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      func(string, string) error { return nil },
	})
	var exitErr *runner.ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected exit code error, got %v", err)
	}
	if exitErr.Code != 17 {
		t.Fatalf("expected exit code 17, got %d", exitErr.Code)
	}
	if tuiCalls != 2 {
		t.Fatalf("expected TUI to be shown twice, got %d", tuiCalls)
	}
	if r.runCalls != 1 {
		t.Fatalf("expected runner to execute once, got %d", r.runCalls)
	}
}

func TestRunWithDeps_ReloadsConfigBeforeReturningToTUI(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(configDir, "opencode.json")
	initial := "{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n"
	updated := "{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	tuiCalls := 0
	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalls++
			switch tuiCalls {
			case 1:
				if len(items) != 2 || items[1].InitiallyEnabled {
					t.Fatalf("expected plugin-b to start disabled, got %+v", items)
				}
				return nil, false, configPath, nil, nil
			case 2:
				if len(items) != 2 || !items[1].InitiallyEnabled {
					t.Fatalf("expected plugin-b to be reloaded as enabled, got %+v", items)
				}
				return nil, true, "", nil, nil
			default:
				t.Fatalf("unexpected TUI call %d", tuiCalls)
				return nil, true, "", nil, nil
			}
		}),
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor: func(path, configEditor string) error {
			if path != configPath {
				return errors.New("unexpected edit path")
			}
			return os.WriteFile(configPath, []byte(updated), 0o644)
		},
	})
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if r.runCalls != 0 {
		t.Fatalf("expected runner to stay idle during edit/reload flow, got %d", r.runCalls)
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

func TestRunWithDeps_EditRequestOpensEditorAndReturnsToTUI(t *testing.T) {
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
		runTUI: scriptedTUI(t,
			tuiResponse{editTarget: configPath},
			tuiResponse{cancelled: true},
		),
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
		runTUI: scriptedTUI(t,
			tuiResponse{editTarget: configPath},
			tuiResponse{cancelled: true},
		),
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
	tuiCalls := 0

	err := runWithDeps(nil, runtimeDeps{
		newRunner:         func() runnerAPI { return r },
		userHomeDir:       func() (string, error) { return tmp, nil },
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: defaultDeps().filterByWhitelist,
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalls++
			gotChoices = append([]tui.EditChoice(nil), editChoices...)
			if tuiCalls == 1 {
				return nil, false, filepath.Join(tmp, ".oc"), nil, nil
			}
			return nil, true, "", nil, nil
		}),
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
		runTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			return map[string]bool{}, false, "", nil, nil
		}),
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

func newLoopbackServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on loopback: %v", err)
	}
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}

func loopbackServerPort(server *httptest.Server) string {
	serverURL := strings.TrimPrefix(server.URL, "http://")
	return serverURL[strings.LastIndex(serverURL, ":")+1:]
}

func TestRunWithDeps_PortSelectionAddsPortFlag(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "50000-55000" {
			t.Fatalf("expected ports range to reach TUI, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", []string{"--port", "51234"}, nil
	})

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
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "50000-55000" {
			t.Fatalf("expected ports range to reach TUI, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

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
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"plugins = [\"oh-my-opencode\"]\n",
	)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "" {
			t.Fatalf("expected empty ports range, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

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

func TestRunWithDeps_PortSelectionRunsWithoutVisiblePlugins(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": []\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
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
	if len(r.args) != 3 || r.args[0] != "--verbose" || r.args[1] != "--port" || r.args[2] != "52000" {
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
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "55000-55500" {
			t.Fatalf("expected [oc] ports to reach TUI, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode@latest": true}, false, "", []string{"--port", "51234"}, nil
	})

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
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
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "55000-55500" {
			t.Fatalf("expected [oc] ports to reach TUI, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": false, "superpowers": true}, false, "", []string{"--port", "51234"}, nil
	})

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
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
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "" {
			t.Fatalf("expected legacy plugin-section ports to be ignored, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if len(r.args) != 2 {
		t.Fatalf("expected no auto-selected port when only legacy plugin ports are configured, got %v", r.args)
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
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "" {
			t.Fatalf("expected top-level ports to be ignored, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}
	if len(r.args) != 2 {
		t.Fatalf("expected no auto-selected port when only top-level ports are configured, got %v", r.args)
	}
}

func TestRunWithDeps_SendsToastAfterLaunchWithPortAndPlugins(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\",\n    \"superpowers\"\n  ]\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	toastCalled := make(chan string, 1)
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/global/health" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"healthy":true,"version":"test"}`)
			return
		}
		if r.URL.Path == "/tui/show-toast" && r.Method == "POST" {
			body := make([]byte, 1024)
			n, _ := r.Body.Read(body)
			toastCalled <- string(body[:n])
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "true")
		}
	}))

	serverPort := loopbackServerPort(server)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.newRunner = func() runnerAPI { return r }
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true, "superpowers": false}, false, "", []string{"--port", serverPort}, nil
	})

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	if !r.ran {
		t.Fatal("runner.Run should be called")
	}

	hasPort := false
	for i, arg := range r.args {
		if arg == "--port" && i+1 < len(r.args) && r.args[i+1] == serverPort {
			hasPort = true
			break
		}
	}
	if !hasPort {
		t.Fatalf("expected args to contain --port %s, got %v", serverPort, r.args)
	}

	select {
	case body := <-toastCalled:
		if !strings.Contains(body, "OC Launcher") {
			t.Fatalf("expected toast body to contain title OC Launcher, got %q", body)
		}
		if !strings.Contains(body, "oh-my-opencode") {
			t.Fatalf("expected toast body to contain oh-my-opencode, got %q", body)
		}
		if !strings.Contains(body, serverPort) {
			t.Fatalf("expected toast body to contain port %s, got %q", serverPort, body)
		}
	case <-time.After(5 * time.Second):
		t.Logf("Runner was called with args: %v", r.args)
		t.Fatal("toast endpoint was not called within timeout")
	}
}

func TestRunWithDeps_SendsToastForUserProvidedPortFlag(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\",\n    \"superpowers\"\n  ]\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	toastCalled := make(chan string, 1)
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/global/health" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"healthy":true,"version":"test"}`)
			return
		}
		if r.URL.Path == "/tui/show-toast" && r.Method == http.MethodPost {
			body := make([]byte, 1024)
			n, _ := r.Body.Read(body)
			toastCalled <- string(body[:n])
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "true")
		}
	}))

	serverPort := loopbackServerPort(server)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "" {
			t.Fatalf("expected auto port selection to be skipped when user passed --port, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true, "superpowers": false}, false, "", nil, nil
	})

	err := runWithDeps([]string{"--model", "gpt-5", "--port", serverPort}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	portFlags := 0
	for _, arg := range r.args {
		if arg == "--port" || strings.HasPrefix(arg, "--port=") {
			portFlags++
		}
	}
	if portFlags != 1 {
		t.Fatalf("expected exactly one --port flag in runner args, got %v", r.args)
	}

	select {
	case body := <-toastCalled:
		if !strings.Contains(body, serverPort) {
			t.Fatalf("expected toast body to contain port %s, got %q", serverPort, body)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("toast endpoint was not called for user-provided --port")
	}
}

func TestRunWithDeps_ClearsStaleToastCallbackWhenNextLaunchHasNoPort(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	toastCalls := make(chan string, 2)
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/global/health" && r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"healthy":true,"version":"test"}`)
			return
		}
		if r.URL.Path == "/tui/show-toast" && r.Method == http.MethodPost {
			body := make([]byte, 1024)
			n, _ := r.Body.Read(body)
			toastCalls <- string(body[:n])
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "true")
		}
	}))

	serverPort := loopbackServerPort(server)

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = scriptedTUI(t,
		tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: []string{"--port", serverPort}},
		tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: nil},
		tuiResponse{cancelled: true},
	)

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	select {
	case <-toastCalls:
	case <-time.After(5 * time.Second):
		t.Fatal("expected first launch to send a toast")
	}

	select {
	case body := <-toastCalls:
		t.Fatalf("expected stale toast callback to be cleared, got extra toast %q", body)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRunWithDeps_SkipsToastWhenNoPortSelected(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"plugins = [\"oh-my-opencode\"]\n",
	)

	toastCalled := false
	_ = newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/global/health" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"healthy":true,"version":"test"}`)
			return
		}
		if r.URL.Path == "/tui/show-toast" {
			toastCalled = true
		}
	}))

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if toastCalled {
		t.Fatal("toast should not be sent when no port is selected")
	}
}

func TestRunWithDeps_LogsToastFailureWithoutBreakingLaunch(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp,
		"{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n",
		"[oc]\nports = \"50000-55000\"\n",
	)

	var attempts atomic.Int32
	done := make(chan struct{})
	server := newLoopbackServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/global/health":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"healthy":true,"version":"test"}`)
		case "/tui/show-toast":
			if attempts.Add(1) == 5 {
				close(done)
			}
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "false")
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	serverPort := loopbackServerPort(server)

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	originalStderr := os.Stderr
	os.Stderr = writePipe
	defer func() {
		os.Stderr = originalStderr
	}()

	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.runTUI = scriptedTUI(t,
		tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: []string{"--port", serverPort}},
		tuiResponse{cancelled: true},
	)

	err = runWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("runWithDeps returned error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("expected toast retries to finish")
	}
	time.Sleep(50 * time.Millisecond)
	writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	readPipe.Close()
	if readErr != nil {
		t.Fatalf("failed to read captured stderr: %v", readErr)
	}

	if !strings.Contains(string(output), "oc: toast failed on port") {
		t.Fatalf("expected toast failure log, got %q", string(output))
	}
}
