package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/launch"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/stats"
	"github.com/kayden-kim/oc/internal/tui"
)

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

func TestRunWithDeps_FullHappyPath(t *testing.T) {
	tmp := t.TempDir()
	initial := "{\n  \"plugin\": [\n    \"plugin-a\",\n    // \"plugin-b\",\n    \"plugin-c\"\n  ]\n}\n"
	configPath := setupConfigFiles(t, tmp, initial, "allow_multiple_plugins = true\n")
	r := &fakeRunner{}
	calledTUI := 0
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

func TestRunWithDeps_DefaultsAllowMultiplePluginsToFalse(t *testing.T) {
	tmp := t.TempDir()
	setupConfigFiles(t, tmp, "{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n", "")
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
	setupConfigFiles(t, tmp, "{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n", "")
	r := &fakeRunner{}
	var err error
	outputStr := captureOutput(t, false, func() {
		err = RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return tmp, nil }, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, RunTUI: scriptedTUI(t, tuiResponse{selections: map[string]bool{"plugin-a": true}}, tuiResponse{cancelled: true}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }})
	})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	for _, unwanted := range []string{"Selected plugins:", "Port selection:", "Using port", "Launching opencode without --port flag."} {
		if strings.Contains(outputStr, unwanted) {
			t.Fatalf("expected no direct pre-launch output %q, got %q", unwanted, outputStr)
		}
	}
}

func TestRunWithDeps_EmptyPluginArrayStillShowsTUI(t *testing.T) {
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
	tuiCallCount := 0
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return tmp, nil }, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
		tuiCallCount++
		tuiCalled = true
		if tuiCallCount > 1 {
			return nil, true, "", nil, nil
		}
		return map[string]bool{}, false, "", []string{"--port", "55501"}, nil
	}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }, SendToast: func(_ context.Context, _ int, _ []string) error { return nil }})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !tuiCalled {
		t.Fatal("TUI should still be shown when visible list is empty")
	}
	if !r.ran {
		t.Fatal("runner.Run should be called")
	}
	if len(r.args) != 2 || r.args[0] != "--port" || r.args[1] != "55501" {
		t.Fatalf("expected --port 55501 args, got %v", r.args)
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
	if err := os.WriteFile(filepath.Join(tmp, ".oc"), []byte("plugins = [\"plugin-a\", \"plugin-b\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return tmp, nil }, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
	}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }})
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
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return tmp, nil }, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
		return nil, true, "", nil, nil
	}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }})
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
	setupConfigFiles(t, tmp, "{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n", "")
	r := &fakeRunner{runErr: &runner.ExitCodeError{Code: 17}}
	tuiCalls := 0
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return tmp, nil }, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
		tuiCalls++
		if tuiCalls == 1 {
			return map[string]bool{"plugin-a": true}, false, "", nil, nil
		}
		return nil, true, "", nil, nil
	}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }})
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
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return tmp, nil }, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
	}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(path, configEditor string) error {
		if path != configPath {
			return errors.New("unexpected edit path")
		}
		return os.WriteFile(configPath, []byte(updated), 0o644)
	}})
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
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: os.UserHomeDir, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, RunTUI: DefaultDeps("test").RunTUI, ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }})
	if err == nil {
		t.Fatal("expected availability error")
	}
	if !strings.Contains(err.Error(), "opencode not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithDeps_SendsToastAfterLaunchWithPortAndPlugins(t *testing.T) {
	tmp := t.TempDir()
	setupPortTestFiles(t, tmp, "{\n  \"plugin\": [\n    \"oh-my-opencode\",\n    \"superpowers\"\n  ]\n}\n", "[oc]\nports = \"50000-55000\"\n")
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
			fmt.Fprintf(w, "true")
		}
	}))
	serverPort := loopbackServerPort(server)
	r := &fakeRunner{}
	deps := baseDepsWithPort(tmp, r)
	deps.NewRunner = func() RunnerAPI { return r }
	deps.SendToast = launch.SendToast
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
	setupPortTestFiles(t, tmp, "{\n  \"plugin\": [\n    \"oh-my-opencode\",\n    \"superpowers\"\n  ]\n}\n", "[oc]\nports = \"50000-55000\"\n")
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
	deps.SendToast = launch.SendToast
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
	setupPortTestFiles(t, tmp, "{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n", "[oc]\nports = \"50000-55000\"\n")
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
	deps.SendToast = launch.SendToast
	deps.RunTUI = scriptedTUI(t, tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: []string{"--port", serverPort}}, tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: nil}, tuiResponse{cancelled: true})
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
	setupPortTestFiles(t, tmp, "{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n", "plugins = [\"oh-my-opencode\"]\n")
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
	setupPortTestFiles(t, tmp, "{\n  \"plugin\": [\n    \"oh-my-opencode\"\n  ]\n}\n", "[oc]\nports = \"50000-55000\"\n")
	done := make(chan struct{})
	const serverPort = "51234"
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
	deps.RunTUI = scriptedTUI(t, tuiResponse{selections: map[string]bool{"oh-my-opencode": true}, portArgs: []string{"--port", serverPort}}, tuiResponse{cancelled: true})
	var err error
	output := captureOutput(t, true, func() { err = RunWithDeps([]string{"--model", "gpt-5"}, deps) })
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("expected SendToast to be invoked")
	}
	time.Sleep(50 * time.Millisecond)
	if !strings.Contains(output, "oc: error: show-toast failed on port") {
		t.Fatalf("expected show-toast error log, got %q", output)
	}
}

func TestRunWithDeps_PassesStatsConfigToTUI(t *testing.T) {
	tmp := t.TempDir()
	configDir := filepath.Join(tmp, ".config", "opencode")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "opencode.json"), []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, ".oc"), []byte("[oc.stats]\nmedium_tokens = 2000\nhigh_tokens = 5000\nscope = \"project\"\nsession_gap_minutes = 15\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	called := false
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return tmp, nil }, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: func(p []config.Plugin, w []string) ([]config.Plugin, []config.Plugin) {
		return DefaultDeps("test").FilterByWhitelist(p, w)
	}, RunTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice, _ []tui.SessionItem, _ tui.SessionItem, _ stats.Report, _ stats.Report, statsConfig config.StatsConfig, _ string, _ bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		called = true
		if statsConfig.MediumTokens != 2000 || statsConfig.HighTokens != 5000 || statsConfig.DefaultScope != "project" || statsConfig.SessionGapMinutes != 15 {
			t.Fatalf("expected stats config {2000,5000,project,15}, got %#v", statsConfig)
		}
		return nil, true, "", nil, tui.SessionItem{}, nil
	}, LoadGlobalStats: func(config.StatsConfig) (stats.Report, error) { return stats.Report{}, nil }, LoadProjectStats: func(string, config.StatsConfig) (stats.Report, error) { return stats.Report{}, nil }, ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }, ParsePortRange: port.ParseRange, SelectPort: port.Select, IsPortAvailable: port.IsAvailable, SendToast: func(context.Context, int, []string) error { return nil }})
	if err != nil {
		t.Fatalf("RunWithDeps returned error: %v", err)
	}
	if !called {
		t.Fatal("expected TUI to be called")
	}
}

func TestRunWithDeps_PersistsMergedPluginSelectionToUserAndProjectConfig(t *testing.T) {
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
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return home, nil }, Getwd: func() (string, error) { return cwd, nil }, ReadFile: os.ReadFile, LoadOcConfig: func(string) (*config.OcConfig, error) { return nil, nil }, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, ListSessions: func(string) ([]tui.SessionItem, error) { return nil, nil }, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
	}), ApplySelections: config.ApplySelections, WriteConfigFile: func(path string, content []byte) error {
		writes = append(writes, writeCall{path, append([]byte(nil), content...)})
		return nil
	}, OpenEditor: func(string, string) error { return nil }})
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
	if err := os.WriteFile(userPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("{\n  \"plugin\": [\n    \"plugin-b\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tuiCalls := 0
	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return home, nil }, Getwd: func() (string, error) { return cwd, nil }, ReadFile: os.ReadFile, LoadOcConfig: func(string) (*config.OcConfig, error) { return nil, nil }, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, ListSessions: func(string) ([]tui.SessionItem, error) { return nil, nil }, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
	}), ApplySelections: config.ApplySelections, WriteConfigFile: func(path string, content []byte) error { return os.WriteFile(path, content, 0o644) }, OpenEditor: func(string, string) error { return nil }})
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
	if err := os.WriteFile(userPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-c\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".oc"), []byte("plugins = [\"plugin-a\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return home, nil }, Getwd: func() (string, error) { return cwd, nil }, ReadFile: os.ReadFile, LoadOcConfig: config.LoadOcConfig, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, ListSessions: func(string) ([]tui.SessionItem, error) { return nil, nil }, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
	}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }})
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
	if err := os.WriteFile(userPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("{\n  \"plugin\": []\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &fakeRunner{}
	err := RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return home, nil }, Getwd: func() (string, error) { return cwd, nil }, ReadFile: os.ReadFile, LoadOcConfig: func(string) (*config.OcConfig, error) { return nil, nil }, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, ListSessions: func(string) ([]tui.SessionItem, error) { return nil, nil }, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
	}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }})
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
	if err := os.WriteFile(userPath, []byte("{\n  \"plugin\": [\n    \"plugin-a\",\n    \"plugin-b\"\n  ]\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projectPath, []byte("{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stderr pipe: %v", err)
	}
	originalStderr := os.Stderr
	os.Stderr = writePipe
	defer func() { os.Stderr = originalStderr }()
	r := &fakeRunner{}
	err = RunWithDeps(nil, RuntimeDeps{NewRunner: func() RunnerAPI { return r }, UserHomeDir: func() (string, error) { return home, nil }, Getwd: func() (string, error) { return cwd, nil }, ReadFile: os.ReadFile, LoadOcConfig: func(string) (*config.OcConfig, error) { return nil, nil }, ParsePlugins: config.ParsePlugins, FilterByWhitelist: DefaultDeps("test").FilterByWhitelist, ListSessions: func(string) ([]tui.SessionItem, error) { return nil, nil }, RunTUI: wrapTUI(func(items []tui.PluginItem, editChoices []tui.EditChoice, _ string, _ bool) (map[string]bool, bool, string, []string, error) {
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
	}), ApplySelections: config.ApplySelections, WriteConfigFile: config.WriteConfigFile, OpenEditor: func(string, string) error { return nil }})
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
