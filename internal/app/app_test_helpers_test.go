package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/port"
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

type fakeEditor struct {
	openErr      error
	opened       bool
	path         string
	configEditor string
}

func (f *fakeEditor) Open(path string, configEditor string) error {
	f.opened = true
	f.path = path
	f.configEditor = configEditor
	return f.openErr
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

func setupConfigFiles(t *testing.T, tmp string, pluginContent string, ocContent string) string {
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
	return configPath
}

func captureOutput(t *testing.T, stderrOnly bool, fn func()) string {
	t.Helper()
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	if !stderrOnly {
		os.Stdout = writePipe
	}
	os.Stderr = writePipe
	defer func() {
		os.Stdout = originalStdout
		os.Stderr = originalStderr
	}()

	fn()
	writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	readPipe.Close()
	if readErr != nil {
		t.Fatalf("failed to read captured output: %v", readErr)
	}
	return string(output)
}

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
