package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/stats"
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

func (f *fakeRunner) Run(args []string, onStart func(context.Context)) error {
	f.ran = true
	f.runCalls++
	f.args = append([]string(nil), args...)
	if f.runErr == nil && onStart != nil {
		go onStart(context.Background())
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

func wrapTUI(fn tuiFunc) func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, stats.Report, stats.Report, config.StatsConfig, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
	return func(items []tui.PluginItem, editChoices []tui.EditChoice, _ []tui.SessionItem, session tui.SessionItem, _ stats.Report, _ stats.Report, _ config.StatsConfig, portsRange string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		selections, cancelled, editTarget, portArgs, err := fn(items, editChoices, portsRange, allowMultiplePlugins)
		return selections, cancelled, editTarget, portArgs, session, err
	}
}

func scriptedTUI(t *testing.T, responses ...tuiResponse) func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, stats.Report, stats.Report, config.StatsConfig, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
	t.Helper()
	call := 0
	return func(items []tui.PluginItem, editChoices []tui.EditChoice, _ []tui.SessionItem, session tui.SessionItem, _ stats.Report, _ stats.Report, _ config.StatsConfig, _ string, _ bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
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

	err := RunWithDeps([]string{"--model", "gpt-5"}, RuntimeDeps{
		NewRunner:    func() RunnerAPI { return r },
		UserHomeDir:  func() (string, error) { return tmp, nil },
		ReadFile:     os.ReadFile,
		LoadOcConfig: config.LoadOcConfig,
		ParsePlugins: config.ParsePlugins,
		FilterByWhitelist: func(p []config.Plugin, w []string) ([]config.Plugin, []config.Plugin) {
			return DefaultDeps("test").FilterByWhitelist(p, w)
		},
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, error) {
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
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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
	if len(r.args) != 1 || r.args[0] != "--continue=ses_manual" {
		t.Fatalf("expected continue flag to win without appended session args, got %#v", r.args)
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
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, error) {
			seenFlags = append(seenFlags, allowMultiplePlugins)
			if r.runCalls == 0 {
				return map[string]bool{"plugin-a": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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

	err = RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: scriptedTUI(t,
			tuiResponse{selections: map[string]bool{"plugin-a": true}},
			tuiResponse{cancelled: true},
		),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	readPipe.Close()
	if readErr != nil {
		t.Fatalf("failed to read captured output: %v", readErr)
	}
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalled = true
			return map[string]bool{}, false, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if tuiCalled {
		t.Fatal("TUI should be skipped when visible list is empty")
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 2 || r.args[0] != "--port" || r.args[1] == "" {
		t.Fatalf("expected default port selection to append --port, got %v", r.args)
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
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			return nil, true, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalls++
			if tuiCalls == 1 {
				return map[string]bool{"plugin-a": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
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
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor: func(path, configEditor string) error {
			if path != configPath {
				return errors.New("unexpected edit path")
			}
			return os.WriteFile(configPath, []byte(updated), 0o644)
		},
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if r.runCalls != 0 {
		t.Fatalf("expected runner to stay idle during edit/reload flow, got %d", r.runCalls)
	}
}

func TestRunWithDeps_CheckAvailableError(t *testing.T) {
	checkErr := errors.New("missing opencode")
	r := &fakeRunner{checkErr: checkErr}

	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       os.UserHomeDir,
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI:            DefaultDeps("test").RunTUI,
		ApplySelections:   config.ApplySelections,
		WriteConfigFile:   config.WriteConfigFile,
		OpenEditor:        func(string, string) error { return nil },
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

	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: scriptedTUI(t,
			tuiResponse{editTarget: configPath},
			tuiResponse{cancelled: true},
		),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      ed.Open,
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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

	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: scriptedTUI(t,
			tuiResponse{editTarget: configPath},
			tuiResponse{cancelled: true},
		),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      ed.Open,
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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

	jsoncPath := filepath.Join(configDir, "oh-my-openagent.jsonc")
	if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	var gotChoices []tui.EditChoice
	tuiCalls := 0

	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalls++
			gotChoices = append([]tui.EditChoice(nil), editChoices...)
			if tuiCalls == 1 {
				return nil, false, filepath.Join(tmp, ".oc"), nil, nil
			}
			return nil, true, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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
	wantOhMyPath := ResolveOhMyOpencodePath(configDir)
	if gotChoices[2].Path != wantOhMyPath {
		t.Fatalf("expected third choice to target resolved oh-my config %q, got %q", wantOhMyPath, gotChoices[2].Path)
	}
	if gotChoices[2].Path != jsoncPath {
		t.Fatalf("expected resolver to recognize openagent jsonc path %q, got %q", jsoncPath, gotChoices[2].Path)
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

		if got := ResolveOhMyOpencodePath(configDir); got != jsonPath {
			t.Fatalf("expected %q, got %q", jsonPath, got)
		}
	})

	t.Run("falls back to opencode jsonc", func(t *testing.T) {
		configDir := t.TempDir()
		jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
		if err := os.WriteFile(jsoncPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := ResolveOhMyOpencodePath(configDir); got != jsoncPath {
			t.Fatalf("expected %q, got %q", jsoncPath, got)
		}
	})

	t.Run("recognizes openagent variants after opencode variants", func(t *testing.T) {
		configDir := t.TempDir()
		openagentJSONPath := filepath.Join(configDir, "oh-my-openagent.json")
		if err := os.WriteFile(openagentJSONPath, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		if got := ResolveOhMyOpencodePath(configDir); got != openagentJSONPath {
			t.Fatalf("expected %q, got %q", openagentJSONPath, got)
		}
	})

	t.Run("defaults to json path", func(t *testing.T) {
		configDir := t.TempDir()
		want := filepath.Join(configDir, "oh-my-opencode.json")

		if got := ResolveOhMyOpencodePath(configDir); got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}

func TestDiscoverOhMyConfigPaths(t *testing.T) {
	t.Run("returns all supported filenames in stable order", func(t *testing.T) {
		configDir := t.TempDir()
		want := []string{
			filepath.Join(configDir, "oh-my-opencode.json"),
			filepath.Join(configDir, "oh-my-opencode.jsonc"),
			filepath.Join(configDir, "oh-my-openagent.json"),
			filepath.Join(configDir, "oh-my-openagent.jsonc"),
		}

		for _, path := range want {
			if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		got := DiscoverOhMyConfigPaths(configDir)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("expected %v, got %v", want, got)
		}
	})

	t.Run("returns empty when none exist", func(t *testing.T) {
		configDir := t.TempDir()
		got := DiscoverOhMyConfigPaths(configDir)
		if len(got) != 0 {
			t.Fatalf("expected no discovered configs, got %v", got)
		}
	})
}

func TestReadOptionalConfigContent_ExistingFile(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "config.json")
	expected := []byte(`{"test": "content"}`)
	if err := os.WriteFile(filePath, expected, 0o644); err != nil {
		t.Fatal(err)
	}

	deps := RuntimeDeps{
		ReadFile: os.ReadFile,
	}

	content, err := readOptionalConfigContent(deps, filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(content, expected) {
		t.Fatalf("expected %q, got %q", expected, content)
	}
}

func TestReadOptionalConfigContent_MissingFile(t *testing.T) {
	deps := RuntimeDeps{
		ReadFile: os.ReadFile,
	}

	content, err := readOptionalConfigContent(deps, "/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if content != nil {
		t.Fatalf("expected nil content for missing file, got %q", content)
	}
}

func TestReadOptionalConfigContent_OtherError(t *testing.T) {
	customErr := errors.New("permission denied")
	deps := RuntimeDeps{
		ReadFile: func(string) ([]byte, error) {
			return nil, customErr
		},
	}

	content, err := readOptionalConfigContent(deps, "/some/path")
	if err != customErr {
		t.Fatalf("expected error %v, got %v", customErr, err)
	}
	if content != nil {
		t.Fatalf("expected nil content on error, got %q", content)
	}
}

func TestLoadIterationState_UserOnlyConfig(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")

	paths := resolveRuntimePaths(home)
	userConfig := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n")

	deps := RuntimeDeps{
		Getwd: func() (string, error) { return cwd, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) {
			return nil, nil
		},
		LoadOcConfig: func(string) (*config.OcConfig, error) {
			return nil, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case paths.configPath:
				return userConfig, nil
			case filepath.Join(cwd, ".opencode", "opencode.json"):
				return nil, os.ErrNotExist
			default:
				t.Fatalf("unexpected read path: %s", path)
				return nil, nil
			}
		},
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
	}

	state, err := loadIterationState(nil, deps, paths, tui.SessionItem{})
	if err != nil {
		t.Fatalf("loadIterationState returned error: %v", err)
	}

	if state.userSource == nil {
		t.Fatal("expected userSource to be populated")
	}
	if state.projectSource != nil {
		t.Fatal("expected projectSource to be nil when project config is missing")
	}
	if len(state.mergedItems) != 2 {
		t.Fatalf("expected 2 merged items from user config, got %d", len(state.mergedItems))
	}
	if state.mergedItems[0].Name != "plugin-a" || state.mergedItems[0].SourceLabel != "" {
		t.Fatalf("expected first merged item to be user-only with empty label, got %+v", state.mergedItems[0])
	}
	if state.mergedItems[1].Name != "plugin-b" || state.mergedItems[1].SourceLabel != "" {
		t.Fatalf("expected second merged item to be user-only with empty label, got %+v", state.mergedItems[1])
	}
	if len(state.mergedPlugins) != 2 || !state.mergedPlugins[0].inUser || state.mergedPlugins[0].inProject {
		t.Fatalf("unexpected merged plugin flags: %+v", state.mergedPlugins)
	}
}

func TestLoadIterationState_DualConfigMergesAndLabelsSources(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")

	paths := resolveRuntimePaths(home)
	userPath := paths.configPath
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")

	userConfig := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n")
	projectConfig := []byte("{\n  \"plugin\": [\n    \"plugin-b\",\n    // \"plugin-c\"\n  ]\n}\n")

	deps := RuntimeDeps{
		Getwd: func() (string, error) { return cwd, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) {
			return nil, nil
		},
		LoadOcConfig: func(string) (*config.OcConfig, error) {
			return nil, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case userPath:
				return userConfig, nil
			case projectPath:
				return projectConfig, nil
			default:
				t.Fatalf("unexpected read path: %s", path)
				return nil, nil
			}
		},
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
	}

	state, err := loadIterationState(nil, deps, paths, tui.SessionItem{})
	if err != nil {
		t.Fatalf("loadIterationState returned error: %v", err)
	}

	if state.userSource == nil || state.projectSource == nil {
		t.Fatalf("expected both sources to be populated, got user=%v project=%v", state.userSource != nil, state.projectSource != nil)
	}

	expectedItems := []tui.PluginItem{
		{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: "User"},
		{Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"},
		{Name: "plugin-c", InitiallyEnabled: false, SourceLabel: "Project"},
	}
	if !reflect.DeepEqual(state.mergedItems, expectedItems) {
		t.Fatalf("merged items mismatch\nwant: %#v\ngot:  %#v", expectedItems, state.mergedItems)
	}

	expectedMerged := []mergedPlugin{
		{name: "plugin-a", inUser: true, inProject: false},
		{name: "plugin-b", inUser: true, inProject: true},
		{name: "plugin-c", inUser: false, inProject: true},
	}
	if !reflect.DeepEqual(state.mergedPlugins, expectedMerged) {
		t.Fatalf("merged metadata mismatch\nwant: %#v\ngot:  %#v", expectedMerged, state.mergedPlugins)
	}
}

func TestLoadIterationState_ProjectParseErrorFallsBackToUserOnly(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")

	paths := resolveRuntimePaths(home)
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")
	userConfig := []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\"\n  ]\n}\n")
	badProjectConfig := []byte("{\n  \"plugin\": [\n")

	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	originalStderr := os.Stderr
	os.Stderr = writePipe
	defer func() {
		os.Stderr = originalStderr
	}()

	deps := RuntimeDeps{
		Getwd: func() (string, error) { return cwd, nil },
		ListSessions: func(string) ([]tui.SessionItem, error) {
			return nil, nil
		},
		LoadOcConfig: func(string) (*config.OcConfig, error) {
			return nil, nil
		},
		ReadFile: func(path string) ([]byte, error) {
			switch path {
			case paths.configPath:
				return userConfig, nil
			case projectPath:
				return badProjectConfig, nil
			default:
				t.Fatalf("unexpected read path: %s", path)
				return nil, nil
			}
		},
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
	}

	state, loadErr := loadIterationState(nil, deps, paths, tui.SessionItem{})
	if loadErr != nil {
		t.Fatalf("loadIterationState returned error: %v", loadErr)
	}

	writePipe.Close()
	stderrOutput, readErr := io.ReadAll(readPipe)
	readPipe.Close()
	if readErr != nil {
		t.Fatalf("failed to read stderr output: %v", readErr)
	}

	if state.projectSource != nil {
		t.Fatal("expected projectSource to be nil on project parse error")
	}
	if len(state.mergedItems) != 2 {
		t.Fatalf("expected user-only merged items on parse fallback, got %d", len(state.mergedItems))
	}
	if state.mergedItems[0].SourceLabel != "" || state.mergedItems[1].SourceLabel != "" {
		t.Fatalf("expected empty labels on parse fallback, got %+v", state.mergedItems)
	}
	if !strings.Contains(string(stderrOutput), "Warning: failed to parse project config ") {
		t.Fatalf("expected parse warning in stderr, got %q", string(stderrOutput))
	}
	if !strings.Contains(string(stderrOutput), projectPath) {
		t.Fatalf("expected warning to include project path %q, got %q", projectPath, string(stderrOutput))
	}
}

func TestMergePlugins(t *testing.T) {
	plugin := func(name string, enabled bool) config.Plugin {
		return config.Plugin{Name: name, Enabled: enabled}
	}

	tests := []struct {
		name           string
		userSource     *configSource
		projectSource  *configSource
		expectedItems  []tui.PluginItem
		expectedMerged []mergedPlugin
	}{
		{
			name: "no project config keeps backward-compatible empty labels",
			userSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-a", true),
				plugin("plugin-b", false),
				plugin("plugin-c", true),
			}},
			projectSource: nil,
			expectedItems: []tui.PluginItem{
				{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: ""},
				{Name: "plugin-b", InitiallyEnabled: false, SourceLabel: ""},
				{Name: "plugin-c", InitiallyEnabled: true, SourceLabel: ""},
			},
			expectedMerged: []mergedPlugin{
				{name: "plugin-a", inUser: true, inProject: false},
				{name: "plugin-b", inUser: true, inProject: false},
				{name: "plugin-c", inUser: true, inProject: false},
			},
		},
		{
			name:       "project-only labels are project",
			userSource: &configSource{plugins: []config.Plugin{}},
			projectSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-a", true),
				plugin("plugin-b", false),
				plugin("plugin-c", true),
			}},
			expectedItems: []tui.PluginItem{
				{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: "Project"},
				{Name: "plugin-b", InitiallyEnabled: false, SourceLabel: "Project"},
				{Name: "plugin-c", InitiallyEnabled: true, SourceLabel: "Project"},
			},
			expectedMerged: []mergedPlugin{
				{name: "plugin-a", inUser: false, inProject: true},
				{name: "plugin-b", inUser: false, inProject: true},
				{name: "plugin-c", inUser: false, inProject: true},
			},
		},
		{
			name: "mixed overlap labels user project and both",
			userSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-a", true),
				plugin("plugin-b", false),
			}},
			projectSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-b", true),
				plugin("plugin-c", true),
			}},
			expectedItems: []tui.PluginItem{
				{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: "User"},
				{Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"},
				{Name: "plugin-c", InitiallyEnabled: true, SourceLabel: "Project"},
			},
			expectedMerged: []mergedPlugin{
				{name: "plugin-a", inUser: true, inProject: false},
				{name: "plugin-b", inUser: true, inProject: true},
				{name: "plugin-c", inUser: false, inProject: true},
			},
		},
		{
			name: "both enabled remains enabled",
			userSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-b", true),
			}},
			projectSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-b", true),
			}},
			expectedItems: []tui.PluginItem{
				{Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"},
			},
			expectedMerged: []mergedPlugin{{name: "plugin-b", inUser: true, inProject: true}},
		},
		{
			name: "split enabled user true project false still enabled",
			userSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-b", true),
			}},
			projectSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-b", false),
			}},
			expectedItems: []tui.PluginItem{
				{Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"},
			},
			expectedMerged: []mergedPlugin{{name: "plugin-b", inUser: true, inProject: true}},
		},
		{
			name: "split enabled user false project true still enabled",
			userSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-b", false),
			}},
			projectSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-b", true),
			}},
			expectedItems: []tui.PluginItem{
				{Name: "plugin-b", InitiallyEnabled: true, SourceLabel: "User, Project"},
			},
			expectedMerged: []mergedPlugin{{name: "plugin-b", inUser: true, inProject: true}},
		},
		{
			name: "empty project source labels user",
			userSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-a", true),
				plugin("plugin-b", false),
				plugin("plugin-c", true),
			}},
			projectSource: &configSource{plugins: []config.Plugin{}},
			expectedItems: []tui.PluginItem{
				{Name: "plugin-a", InitiallyEnabled: true, SourceLabel: "User"},
				{Name: "plugin-b", InitiallyEnabled: false, SourceLabel: "User"},
				{Name: "plugin-c", InitiallyEnabled: true, SourceLabel: "User"},
			},
			expectedMerged: []mergedPlugin{
				{name: "plugin-a", inUser: true, inProject: false},
				{name: "plugin-b", inUser: true, inProject: false},
				{name: "plugin-c", inUser: true, inProject: false},
			},
		},
		{
			name: "ordering keeps user first then project-only",
			userSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-u1", true),
				plugin("plugin-u2", false),
			}},
			projectSource: &configSource{plugins: []config.Plugin{
				plugin("plugin-u2", true),
				plugin("plugin-p1", false),
				plugin("plugin-p2", true),
			}},
			expectedItems: []tui.PluginItem{
				{Name: "plugin-u1", InitiallyEnabled: true, SourceLabel: "User"},
				{Name: "plugin-u2", InitiallyEnabled: true, SourceLabel: "User, Project"},
				{Name: "plugin-p1", InitiallyEnabled: false, SourceLabel: "Project"},
				{Name: "plugin-p2", InitiallyEnabled: true, SourceLabel: "Project"},
			},
			expectedMerged: []mergedPlugin{
				{name: "plugin-u1", inUser: true, inProject: false},
				{name: "plugin-u2", inUser: true, inProject: true},
				{name: "plugin-p1", inUser: false, inProject: true},
				{name: "plugin-p2", inUser: false, inProject: true},
			},
		},
		{
			name: "deduplicates by exact name not comparison name",
			userSource: &configSource{plugins: []config.Plugin{
				plugin("oh-my-opencode", true),
			}},
			projectSource: &configSource{plugins: []config.Plugin{
				plugin("oh-my-opencode@latest", false),
			}},
			expectedItems: []tui.PluginItem{
				{Name: "oh-my-opencode", InitiallyEnabled: true, SourceLabel: "User"},
				{Name: "oh-my-opencode@latest", InitiallyEnabled: false, SourceLabel: "Project"},
			},
			expectedMerged: []mergedPlugin{
				{name: "oh-my-opencode", inUser: true, inProject: false},
				{name: "oh-my-opencode@latest", inUser: false, inProject: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, merged := mergePlugins(tt.userSource, tt.projectSource)

			if !reflect.DeepEqual(items, tt.expectedItems) {
				t.Fatalf("items mismatch\nwant: %#v\ngot:  %#v", tt.expectedItems, items)
			}
			if !reflect.DeepEqual(merged, tt.expectedMerged) {
				t.Fatalf("merged mismatch\nwant: %#v\ngot:  %#v", tt.expectedMerged, merged)
			}
		})
	}
}

func TestPersistSelections_UserOnlyPluginWritesOnlyUserFile(t *testing.T) {
	const userPath = "/tmp/user-opencode.json"
	const projectPath = "/tmp/project-opencode.json"

	state := iterationState{
		userSource: &configSource{
			path:    userPath,
			content: []byte("{\n  \"plugin\": [\n    \"user-only\"\n  ]\n}\n"),
		},
		projectSource: &configSource{
			path:    projectPath,
			content: []byte("{\n  \"plugin\": [\n    \"project-only\"\n  ]\n}\n"),
		},
		mergedPlugins: []mergedPlugin{
			{name: "user-only", inUser: true, inProject: false},
			{name: "project-only", inUser: false, inProject: true},
		},
	}

	var writtenPaths []string
	var writtenContents [][]byte
	deps := RuntimeDeps{
		ApplySelections: config.ApplySelections,
		WriteConfigFile: func(path string, content []byte) error {
			writtenPaths = append(writtenPaths, path)
			writtenContents = append(writtenContents, append([]byte(nil), content...))
			return nil
		},
	}

	err := persistSelections(deps, state, map[string]bool{"user-only": false})
	if err != nil {
		t.Fatalf("persistSelections returned error: %v", err)
	}

	if len(writtenPaths) != 1 || writtenPaths[0] != userPath {
		t.Fatalf("expected only user file write, got paths=%v", writtenPaths)
	}
	if !strings.Contains(string(writtenContents[0]), "// \"user-only\"") {
		t.Fatalf("expected user-only to be disabled in user file, got:\n%s", string(writtenContents[0]))
	}
}

func TestPersistSelections_ProjectOnlyPluginWritesOnlyProjectFile(t *testing.T) {
	const userPath = "/tmp/user-opencode.json"
	const projectPath = "/tmp/project-opencode.json"

	state := iterationState{
		userSource: &configSource{
			path:    userPath,
			content: []byte("{\n  \"plugin\": [\n    \"user-only\"\n  ]\n}\n"),
		},
		projectSource: &configSource{
			path:    projectPath,
			content: []byte("{\n  \"plugin\": [\n    // \"project-only\"\n  ]\n}\n"),
		},
		mergedPlugins: []mergedPlugin{
			{name: "user-only", inUser: true, inProject: false},
			{name: "project-only", inUser: false, inProject: true},
		},
	}

	var writtenPaths []string
	var writtenContents [][]byte
	deps := RuntimeDeps{
		ApplySelections: config.ApplySelections,
		WriteConfigFile: func(path string, content []byte) error {
			writtenPaths = append(writtenPaths, path)
			writtenContents = append(writtenContents, append([]byte(nil), content...))
			return nil
		},
	}

	err := persistSelections(deps, state, map[string]bool{"project-only": true})
	if err != nil {
		t.Fatalf("persistSelections returned error: %v", err)
	}

	if len(writtenPaths) != 1 || writtenPaths[0] != projectPath {
		t.Fatalf("expected only project file write, got paths=%v", writtenPaths)
	}
	content := string(writtenContents[0])
	if strings.Contains(content, "// \"project-only\"") || !strings.Contains(content, "\"project-only\"") {
		t.Fatalf("expected project-only to be enabled in project file, got:\n%s", content)
	}
}

func TestPersistSelections_SharedPluginWritesBothFiles(t *testing.T) {
	const userPath = "/tmp/user-opencode.json"
	const projectPath = "/tmp/project-opencode.json"

	state := iterationState{
		userSource: &configSource{
			path:    userPath,
			content: []byte("{\n  \"plugin\": [\n    \"shared\"\n  ]\n}\n"),
		},
		projectSource: &configSource{
			path:    projectPath,
			content: []byte("{\n  \"plugin\": [\n    \"shared\"\n  ]\n}\n"),
		},
		mergedPlugins: []mergedPlugin{
			{name: "shared", inUser: true, inProject: true},
		},
	}

	var writtenPaths []string
	var writtenContents [][]byte
	deps := RuntimeDeps{
		ApplySelections: config.ApplySelections,
		WriteConfigFile: func(path string, content []byte) error {
			writtenPaths = append(writtenPaths, path)
			writtenContents = append(writtenContents, append([]byte(nil), content...))
			return nil
		},
	}

	err := persistSelections(deps, state, map[string]bool{"shared": false})
	if err != nil {
		t.Fatalf("persistSelections returned error: %v", err)
	}

	if len(writtenPaths) != 2 {
		t.Fatalf("expected dual writes, got paths=%v", writtenPaths)
	}
	if writtenPaths[0] != userPath || writtenPaths[1] != projectPath {
		t.Fatalf("expected user then project writes, got paths=%v", writtenPaths)
	}
	if !strings.Contains(string(writtenContents[0]), "// \"shared\"") {
		t.Fatalf("expected shared plugin to be disabled in user file, got:\n%s", string(writtenContents[0]))
	}
	if !strings.Contains(string(writtenContents[1]), "// \"shared\"") {
		t.Fatalf("expected shared plugin to be disabled in project file, got:\n%s", string(writtenContents[1]))
	}
}

func TestPersistSelections_NoProjectSourceRemainsBackwardCompatible(t *testing.T) {
	const userPath = "/tmp/user-opencode.json"

	state := iterationState{
		userSource: &configSource{
			path:    userPath,
			content: []byte("{\n  \"plugin\": [\n    // \"user-only\"\n  ]\n}\n"),
		},
		projectSource: nil,
		mergedPlugins: []mergedPlugin{
			{name: "user-only", inUser: true, inProject: false},
		},
	}

	var writtenPaths []string
	var writtenContents [][]byte
	deps := RuntimeDeps{
		ApplySelections: config.ApplySelections,
		WriteConfigFile: func(path string, content []byte) error {
			writtenPaths = append(writtenPaths, path)
			writtenContents = append(writtenContents, append([]byte(nil), content...))
			return nil
		},
	}

	err := persistSelections(deps, state, map[string]bool{"user-only": true})
	if err != nil {
		t.Fatalf("persistSelections returned error: %v", err)
	}

	if len(writtenPaths) != 1 || writtenPaths[0] != userPath {
		t.Fatalf("expected single user write with no project source, got paths=%v", writtenPaths)
	}
	content := string(writtenContents[0])
	if strings.Contains(content, "// \"user-only\"") || !strings.Contains(content, "\"user-only\"") {
		t.Fatalf("expected user-only to be enabled in user file, got:\n%s", content)
	}
}

// --- Port selection tests ---

func baseDepsWithPort(tmp string, r *fakeRunner) RuntimeDeps {
	return RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return tmp, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			return map[string]bool{}, false, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
		ParsePortRange:  port.ParseRange,
		SelectPort: func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, p int, available bool)) port.SelectResult {
			return port.SelectResult{Port: 51234, Attempts: 1, Found: true}
		},
		IsPortAvailable: func(int) bool { return true },
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

func TestRunWithDeps_PassesStatsConfigToTUI(t *testing.T) {
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
	if err := os.WriteFile(ocConfigPath, []byte("[oc.stats]\nmedium_tokens = 2000\nhigh_tokens = 5000\nscope = \"project\"\nsession_gap_minutes = 15\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	called := false
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:    func() RunnerAPI { return r },
		UserHomeDir:  func() (string, error) { return tmp, nil },
		ReadFile:     os.ReadFile,
		LoadOcConfig: config.LoadOcConfig,
		ParsePlugins: config.ParsePlugins,
		FilterByWhitelist: func(p []config.Plugin, w []string) ([]config.Plugin, []config.Plugin) {
			return DefaultDeps("test").FilterByWhitelist(p, w)
		},
		RunTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice, _ []tui.SessionItem, _ tui.SessionItem, _ stats.Report, _ stats.Report, statsConfig config.StatsConfig, _ string, _ bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
			called = true
			if statsConfig.MediumTokens != 2000 || statsConfig.HighTokens != 5000 || statsConfig.DefaultScope != "project" || statsConfig.SessionGapMinutes != 15 {
				t.Fatalf("expected stats config {2000,5000,project,15}, got %#v", statsConfig)
			}
			return nil, true, "", nil, tui.SessionItem{}, nil
		},
		LoadGlobalStats:  func(config.StatsConfig) (stats.Report, error) { return stats.Report{}, nil },
		LoadProjectStats: func(string, config.StatsConfig) (stats.Report, error) { return stats.Report{}, nil },
		ApplySelections:  config.ApplySelections,
		WriteConfigFile:  config.WriteConfigFile,
		OpenEditor:       func(string, string) error { return nil },
		ParsePortRange:   port.ParseRange,
		SelectPort:       port.Select,
		IsPortAvailable:  port.IsAvailable,
		SendToast:        func(context.Context, int, []string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !called {
		t.Fatal("expected TUI to be called")
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
	deps.SelectPort = func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, p int, available bool)) port.SelectResult {
		return port.SelectResult{Port: 52000, Attempts: 1, Found: true}
	}

	err := RunWithDeps([]string{"--verbose"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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
	deps.NewRunner = func() RunnerAPI { return r }
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true, "superpowers": false}, false, "", []string{"--port", serverPort}, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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
	case <-time.After(7 * time.Second):
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
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if portsRange != "" {
			t.Fatalf("expected auto port selection to be skipped when user passed --port, got %q", portsRange)
		}
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true, "superpowers": false}, false, "", nil, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5", "--port", serverPort}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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
	case <-time.After(7 * time.Second):
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
	deps.RunTUI = scriptedTUI(t,
		tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: []string{"--port", serverPort}},
		tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: nil},
		tuiResponse{cancelled: true},
	)

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}

	select {
	case <-toastCalls:
	case <-time.After(7 * time.Second):
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
	deps.RunTUI = wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, portsRange string, _ bool) (map[string]bool, bool, string, []string, error) {
		if r.runCalls > 0 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{"oh-my-opencode": true}, false, "", nil, nil
	})

	err := RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
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

	done := make(chan struct{})
	const serverPort = "51234"

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
	deps.SendToast = func(_ context.Context, port int, plugins []string) error {
		if port != 51234 {
			t.Fatalf("expected SendToast port 51234, got %d", port)
		}
		if len(plugins) != 1 || plugins[0] != "oh-my-opencode" {
			t.Fatalf("unexpected toast plugins: %v", plugins)
		}
		close(done)
		return errors.New("toast failed")
	}
	deps.RunTUI = scriptedTUI(t,
		tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: []string{"--port", serverPort}},
		tuiResponse{cancelled: true},
	)

	err = RunWithDeps([]string{"--model", "gpt-5"}, deps)
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected SendToast to be invoked")
	}
	time.Sleep(50 * time.Millisecond)
	writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	readPipe.Close()
	if readErr != nil {
		t.Fatalf("failed to read captured stderr: %v", readErr)
	}

	if !strings.Contains(string(output), "oc: error: show-toast failed on port") {
		t.Fatalf("expected show-toast error log, got %q", string(output))
	}
}

func TestBuildEditChoices_ProjectConfigExists(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	projectDir := filepath.Join(tmp, ".opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userOhMyPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
	projectOhMyPath := filepath.Join(projectDir, "oh-my-openagent.json")
	if err := os.WriteFile(userOhMyPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectOhMyPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ocConfigPath := filepath.Join(tmp, ".oc")
	projectConfigPath := filepath.Join(projectDir, "opencode.json")

	paths := runtimePaths{
		ocConfigPath: ocConfigPath,
		configPath:   filepath.Join(configDir, "opencode.json"),
		configDir:    configDir,
	}

	choices := buildEditChoices(paths, projectConfigPath, true)

	if len(choices) != 5 {
		t.Fatalf("expected 5 edit choices when project config and oh-my configs exist, got %d", len(choices))
	}

	// Verify first 3 choices are unchanged
	if choices[0].Label != "1) .oc file" {
		t.Fatalf("expected first choice label '1) .oc file', got %q", choices[0].Label)
	}
	if choices[0].Path != ocConfigPath {
		t.Fatalf("expected first choice path %q, got %q", ocConfigPath, choices[0].Path)
	}

	if choices[1].Label != "2) opencode.json file" {
		t.Fatalf("expected second choice label '2) opencode.json file', got %q", choices[1].Label)
	}

	if choices[2].Path != userOhMyPath {
		t.Fatalf("expected third choice path %q, got %q", userOhMyPath, choices[2].Path)
	}

	if choices[3].Label != "4) project opencode.json file" {
		t.Fatalf("expected fourth choice label '4) project opencode.json file', got %q", choices[3].Label)
	}
	if choices[3].Path != projectConfigPath {
		t.Fatalf("expected fourth choice path %q, got %q", projectConfigPath, choices[3].Path)
	}

	if choices[4].Path != projectOhMyPath {
		t.Fatalf("expected fifth choice path %q, got %q", projectOhMyPath, choices[4].Path)
	}
	if choices[4].Label != "5) project oh-my-openagent.json file" {
		t.Fatalf("expected fifth choice label '5) project oh-my-openagent.json file', got %q", choices[4].Label)
	}
}

func TestBuildEditChoices_ProjectConfigAbsent(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userOhMyPath := filepath.Join(configDir, "oh-my-openagent.json")
	if err := os.WriteFile(userOhMyPath, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ocConfigPath := filepath.Join(tmp, ".oc")

	paths := runtimePaths{
		ocConfigPath: ocConfigPath,
		configPath:   filepath.Join(configDir, "opencode.json"),
		configDir:    configDir,
	}
	projectPath := "/tmp/project-opencode.json"

	choices := buildEditChoices(paths, projectPath, false)

	if len(choices) != 3 {
		t.Fatalf("expected 3 edit choices when project config is absent and one user oh-my config exists, got %d", len(choices))
	}

	if choices[0].Label != "1) .oc file" {
		t.Fatalf("expected first choice label '1) .oc file', got %q", choices[0].Label)
	}
	if choices[0].Path != ocConfigPath {
		t.Fatalf("expected first choice path %q, got %q", ocConfigPath, choices[0].Path)
	}
	if choices[2].Path != userOhMyPath {
		t.Fatalf("expected third choice path %q, got %q", userOhMyPath, choices[2].Path)
	}
	if choices[2].Label != "3) oh-my-openagent.json file" {
		t.Fatalf("expected third choice label '3) oh-my-openagent.json file', got %q", choices[2].Label)
	}
}

// --- Dual-config integration tests ---

func TestIntegration_FullDualConfigCycle(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	userPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}

	userInitial := "{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n"
	projectInitial := "{\n  \"plugin\": [\n    // \"plugin-b\",\n    \"plugin-c\"\n  ]\n}\n"

	if err := os.WriteFile(userPath, []byte(userInitial), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte(projectInitial), 0o644); err != nil {
		t.Fatal(err)
	}

	type writeCall struct {
		path    string
		content []byte
	}
	var writes []writeCall

	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:   func() RunnerAPI { return r },
		UserHomeDir: func() (string, error) { return home, nil },
		Getwd:       func() (string, error) { return cwd, nil },
		ReadFile:    os.ReadFile,
		LoadOcConfig: func(string) (*config.OcConfig, error) {
			return nil, nil
		},
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		ListSessions:      func(string) ([]tui.SessionItem, error) { return nil, nil },
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			if len(items) != 3 {
				t.Fatalf("expected 3 merged items [A:User, B:User+Project, C:Project], got %d: %#v", len(items), items)
			}
			if items[0].Name != "plugin-a" || items[0].SourceLabel != "User" || !items[0].InitiallyEnabled {
				t.Fatalf("expected first item A enabled from user, got %#v", items[0])
			}
			if items[1].Name != "plugin-b" || items[1].SourceLabel != "User, Project" || !items[1].InitiallyEnabled {
				t.Fatalf("expected second item B enabled (merged from user), got %#v", items[1])
			}
			if items[2].Name != "plugin-c" || items[2].SourceLabel != "Project" || !items[2].InitiallyEnabled {
				t.Fatalf("expected third item C enabled from project, got %#v", items[2])
			}
			if r.runCalls == 0 {
				return map[string]bool{"plugin-a": true, "plugin-b": false, "plugin-c": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: func(path string, content []byte) error {
			writes = append(writes, writeCall{path, append([]byte(nil), content...)})
			return nil
		},
		OpenEditor: func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}

	if len(writes) != 2 {
		t.Fatalf("expected 2 writes (user + project), got %d", len(writes))
	}

	var userWrite, projectWrite *writeCall
	for i := range writes {
		if writes[i].path == userPath {
			userWrite = &writes[i]
		} else if writes[i].path == projectPath {
			projectWrite = &writes[i]
		}
	}

	if userWrite == nil || projectWrite == nil {
		t.Fatalf("expected user and project writes, got paths: %v", writes)
	}

	userOut := string(userWrite.content)
	if !strings.Contains(userOut, "\"plugin-a\"") || strings.Contains(userOut, "// \"plugin-a\"") {
		t.Fatalf("expected plugin-a enabled in user file, got:\n%s", userOut)
	}
	if !strings.Contains(userOut, "// \"plugin-b\"") {
		t.Fatalf("expected plugin-b disabled in user file, got:\n%s", userOut)
	}

	projectOut := string(projectWrite.content)
	if !strings.Contains(projectOut, "// \"plugin-b\"") {
		t.Fatalf("expected plugin-b disabled in project file, got:\n%s", projectOut)
	}
	if !strings.Contains(projectOut, "\"plugin-c\"") || strings.Contains(projectOut, "// \"plugin-c\"") {
		t.Fatalf("expected plugin-c enabled in project file, got:\n%s", projectOut)
	}
}

func TestIntegration_ReentrantLoopConsistency(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	userPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}

	userConfig := "{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"
	projectConfig := "{\n  \"plugin\": [\n    \"plugin-b\"\n  ]\n}\n"

	if err := os.WriteFile(userPath, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	tuiCalls := 0
	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:   func() RunnerAPI { return r },
		UserHomeDir: func() (string, error) { return home, nil },
		Getwd:       func() (string, error) { return cwd, nil },
		ReadFile:    os.ReadFile,
		LoadOcConfig: func(string) (*config.OcConfig, error) {
			return nil, nil
		},
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		ListSessions:      func(string) ([]tui.SessionItem, error) { return nil, nil },
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			tuiCalls++
			if tuiCalls == 1 {
				if len(items) != 2 {
					t.Fatalf("first TUI call: expected 2 merged items, got %d", len(items))
				}
				if items[0].Name != "plugin-a" || items[0].SourceLabel != "User" {
					t.Fatalf("first TUI call: expected plugin-a with User label, got %#v", items[0])
				}
				if items[1].Name != "plugin-b" || items[1].SourceLabel != "Project" {
					t.Fatalf("first TUI call: expected plugin-b with Project label, got %#v", items[1])
				}
				return map[string]bool{"plugin-a": true, "plugin-b": true}, false, "", nil, nil
			} else if tuiCalls == 2 {
				if len(items) != 2 {
					t.Fatalf("second TUI call: expected 2 merged items, got %d", len(items))
				}
				if items[0].Name != "plugin-a" || items[0].SourceLabel != "User" || !items[0].InitiallyEnabled {
					t.Fatalf("second TUI call: expected plugin-a consistent, got %#v", items[0])
				}
				if items[1].Name != "plugin-b" || items[1].SourceLabel != "Project" || !items[1].InitiallyEnabled {
					t.Fatalf("second TUI call: expected plugin-b consistent, got %#v", items[1])
				}
				return nil, true, "", nil, nil
			}
			t.Fatalf("unexpected TUI call %d", tuiCalls)
			return nil, false, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: func(path string, content []byte) error {
			return os.WriteFile(path, content, 0o644)
		},
		OpenEditor: func(string, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}

	if tuiCalls != 2 {
		t.Fatalf("expected TUI to be called twice (launch + re-entry), got %d", tuiCalls)
	}
}

func TestIntegration_WhitelistFiltersAcrossBothSources(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	userPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}

	userConfig := "{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n"
	projectConfig := "{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-c\"\n  ]\n}\n"

	if err := os.WriteFile(userPath, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	ocPath := filepath.Join(home, ".oc")
	if err := os.WriteFile(ocPath, []byte("plugins = [\"plugin-a\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return home, nil },
		Getwd:             func() (string, error) { return cwd, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		ListSessions:      func(string) ([]tui.SessionItem, error) { return nil, nil },
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			if len(items) != 1 {
				t.Fatalf("expected only whitelisted plugin-a, got %d items: %#v", len(items), items)
			}
			if items[0].Name != "plugin-a" || items[0].SourceLabel != "User, Project" {
				t.Fatalf("expected plugin-a with User,Project label, got %#v", items[0])
			}
			if r.runCalls == 0 {
				return map[string]bool{"plugin-a": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
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
}

func TestIntegration_EmptyProjectPluginArray(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	userPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}

	userConfig := "{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n"
	projectConfig := "{\n  \"plugin\": []\n}\n"

	if err := os.WriteFile(userPath, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return home, nil },
		Getwd:             func() (string, error) { return cwd, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      func(string) (*config.OcConfig, error) { return nil, nil },
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		ListSessions:      func(string) ([]tui.SessionItem, error) { return nil, nil },
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			if len(items) != 2 {
				t.Fatalf("expected 2 user plugins shown, got %d", len(items))
			}
			if items[0].SourceLabel != "User" || items[1].SourceLabel != "User" {
				t.Fatalf("expected User labels for user-only plugins, got %#v", items)
			}
			if r.runCalls == 0 {
				return map[string]bool{"plugin-a": true, "plugin-b": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
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
}

func TestIntegration_NoPluginKeyInProjectConfig(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(home, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}

	userPath := filepath.Join(home, ".config", "opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	projectPath := filepath.Join(cwd, ".opencode", "opencode.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatal(err)
	}

	userConfig := "{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n"
	projectConfig := "{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"

	if err := os.WriteFile(userPath, []byte(userConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte(projectConfig), 0o644); err != nil {
		t.Fatal(err)
	}

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
	err = RunWithDeps(nil, RuntimeDeps{
		NewRunner:         func() RunnerAPI { return r },
		UserHomeDir:       func() (string, error) { return home, nil },
		Getwd:             func() (string, error) { return cwd, nil },
		ReadFile:          os.ReadFile,
		LoadOcConfig:      func(string) (*config.OcConfig, error) { return nil, nil },
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: DefaultDeps("test").FilterByWhitelist,
		ListSessions:      func(string) ([]tui.SessionItem, error) { return nil, nil },
		RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
			if len(items) != 2 {
				t.Fatalf("expected 2 user plugins shown on project parse fallback, got %d", len(items))
			}
			if items[0].SourceLabel != "" || items[1].SourceLabel != "" {
				t.Fatalf("expected empty labels on project parse fallback, got %#v", items)
			}
			if r.runCalls == 0 {
				return map[string]bool{"plugin-a": true, "plugin-b": true}, false, "", nil, nil
			}
			return nil, true, "", nil, nil
		}),
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      func(string, string) error { return nil },
	})

	writePipe.Close()
	stderrOutput, readErr := io.ReadAll(readPipe)
	readPipe.Close()
	if readErr != nil {
		t.Fatalf("failed to read stderr output: %v", readErr)
	}

	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}

	if !r.ran {
		t.Fatal("runner.Run should be called")
	}

	if !strings.Contains(string(stderrOutput), "Warning: failed to parse project config") {
		t.Fatalf("expected parse warning in stderr, got %q", string(stderrOutput))
	}
	if !strings.Contains(string(stderrOutput), projectPath) {
		t.Fatalf("expected warning to include project path %q, got %q", projectPath, string(stderrOutput))
	}
}
