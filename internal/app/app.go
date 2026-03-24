package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/editor"
	"github.com/kayden-kim/oc/internal/launch"
	"github.com/kayden-kim/oc/internal/plugin"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/session"
	"github.com/kayden-kim/oc/internal/tui"
)

type RunnerAPI interface {
	CheckAvailable() error
	Run(args []string, onStart func()) error
}

type RuntimeDeps struct {
	Version           string
	NewRunner         func() RunnerAPI
	UserHomeDir       func() (string, error)
	ReadFile          func(string) ([]byte, error)
	LoadOcConfig      func(string) (*config.OcConfig, error)
	ParsePlugins      func([]byte) ([]config.Plugin, string, error)
	FilterByWhitelist func([]config.Plugin, []string) ([]config.Plugin, []config.Plugin)
	Getwd             func() (string, error)
	ListSessions      func(string) ([]tui.SessionItem, error)
	RunTUI            func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error)
	ApplySelections   func([]byte, map[string]bool) ([]byte, error)
	WriteConfigFile   func(string, []byte) error
	OpenEditor        func(string, string) error
	ParsePortRange    func(string) (int, int, error)
	SelectPort        func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, port int, available bool)) port.SelectResult
	IsPortAvailable   func(int) bool
	SendToast         func(int, []string) error
}

type runtimePaths struct {
	ocConfigPath string
	configDir    string
	configPath   string
}

type iterationState struct {
	cwd                  string
	sessions             []tui.SessionItem
	selectedSession      tui.SessionItem
	content              []byte
	visible              []config.Plugin
	configEditor         string
	effectivePortsRange  string
	allowMultiplePlugins bool
}

func DefaultDeps(version string) RuntimeDeps {
	deps := RuntimeDeps{
		Version:           version,
		NewRunner:         func() RunnerAPI { return runner.NewRunner() },
		UserHomeDir:       os.UserHomeDir,
		ReadFile:          os.ReadFile,
		LoadOcConfig:      config.LoadOcConfig,
		ParsePlugins:      config.ParsePlugins,
		FilterByWhitelist: plugin.FilterByWhitelist,
		Getwd:             os.Getwd,
		ListSessions:      session.List,
		ApplySelections:   config.ApplySelections,
		WriteConfigFile:   config.WriteConfigFile,
		OpenEditor:        editor.OpenWithConfig,
		ParsePortRange:    port.ParseRange,
		SelectPort:        port.Select,
		IsPortAvailable:   port.IsAvailable,
		SendToast:         launch.SendToast,
	}

	deps.RunTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice, sessions []tui.SessionItem, selectedSession tui.SessionItem, portsRange string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		model := tui.NewModel(items, editChoices, sessions, selectedSession, version, allowMultiplePlugins)
		result, err := tea.NewProgram(model).Run()
		if err != nil {
			return nil, false, "", nil, tui.SessionItem{}, err
		}
		finalModel, ok := result.(tui.Model)
		if !ok {
			return nil, false, "", nil, tui.SessionItem{}, fmt.Errorf("unexpected TUI model type %T", result)
		}

		selections := finalModel.Selections()
		if finalModel.Cancelled() || finalModel.EditTarget() != "" {
			return selections, finalModel.Cancelled(), finalModel.EditTarget(), nil, finalModel.SelectedSession(), nil
		}

		portArgs, err := runLaunchTUI(selectedPluginNames(selections), finalModel.SelectedSession(), portsRange, deps, version)
		if err != nil {
			return nil, false, "", nil, tui.SessionItem{}, err
		}

		return selections, false, "", portArgs, finalModel.SelectedSession(), nil
	}

	return deps
}

func Run(args []string, version string) error {
	return RunWithDeps(args, DefaultDeps(version))
}

func RunWithDeps(args []string, deps RuntimeDeps) error {
	deps = normalizeDeps(deps)

	r := deps.NewRunner()
	if err := r.CheckAvailable(); err != nil {
		return fmt.Errorf("opencode not found: %w", err)
	}

	var lastExitErr *runner.ExitCodeError

	homeDir, err := deps.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	paths := resolveRuntimePaths(homeDir)
	selectedSession := tui.SessionItem{}

	for {
		state, err := loadIterationState(args, deps, paths, selectedSession)
		if err != nil {
			return err
		}
		selectedSession = state.selectedSession

		if len(state.visible) == 0 {
			portArgs := launch.ResolvePortArgs(state.effectivePortsRange, deps.ParsePortRange, deps.SelectPort, deps.IsPortAvailable, nil)
			return runOpencode(r, args, portArgs, state.selectedSession, nil, deps.SendToast)
		}

		selections, cancelled, editTarget, portArgs, nextSession, err := deps.RunTUI(
			buildPluginItems(state.visible),
			buildEditChoices(paths),
			state.sessions,
			state.selectedSession,
			state.effectivePortsRange,
			state.allowMultiplePlugins,
		)
		selectedSession = nextSession
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		if cancelled {
			if lastExitErr != nil {
				return lastExitErr
			}
			return nil
		}
		if editTarget != "" {
			if err := deps.OpenEditor(editTarget, state.configEditor); err != nil {
				return fmt.Errorf("failed to open editor for %s: %w", editTarget, err)
			}
			continue
		}

		if err := persistSelections(deps, paths.configPath, state.content, selections); err != nil {
			return err
		}

		err = runOpencode(r, args, portArgs, selectedSession, selections, deps.SendToast)
		selectedSession = refreshSelectedSession(deps, state.cwd, selectedSession)
		if exitErr, ok := runner.IsExitCode(err); ok {
			lastExitErr = exitErr
			fmt.Fprintf(os.Stderr, "opencode exited with code %d\n\n", exitErr.Code)
			continue
		}
		if err != nil {
			return err
		}
		lastExitErr = nil
	}
}

func normalizeDeps(deps RuntimeDeps) RuntimeDeps {
	if deps.Getwd == nil {
		deps.Getwd = os.Getwd
	}
	if deps.ListSessions == nil {
		deps.ListSessions = func(string) ([]tui.SessionItem, error) {
			return nil, nil
		}
	}
	if deps.SendToast == nil {
		deps.SendToast = launch.SendToast
	}
	return deps
}

func resolveRuntimePaths(homeDir string) runtimePaths {
	return runtimePaths{
		ocConfigPath: filepath.Join(homeDir, ".oc"),
		configDir:    filepath.Join(homeDir, ".config", "opencode"),
		configPath:   filepath.Join(homeDir, ".config", "opencode", "opencode.json"),
	}
}

func loadIterationState(args []string, deps RuntimeDeps, paths runtimePaths, selectedSession tui.SessionItem) (iterationState, error) {
	cwd, err := deps.Getwd()
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	sessions, err := deps.ListSessions(cwd)
	if err != nil {
		sessions = nil
	}
	if selectedSession.ID == "" {
		selectedSession = session.Latest(sessions)
	}

	ocConfig, err := deps.LoadOcConfig(paths.ocConfigPath)
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to load whitelist: %w", err)
	}

	whitelist, configEditor, effectivePortsRange, allowMultiplePlugins := extractRuntimeConfig(args, ocConfig)
	content, err := readConfigContent(deps, paths.configPath)
	if err != nil {
		return iterationState{}, err
	}
	visible, err := visiblePlugins(deps, content, whitelist)
	if err != nil {
		return iterationState{}, err
	}

	return iterationState{
		cwd:                  cwd,
		sessions:             sessions,
		selectedSession:      selectedSession,
		content:              content,
		visible:              visible,
		configEditor:         configEditor,
		effectivePortsRange:  effectivePortsRange,
		allowMultiplePlugins: allowMultiplePlugins,
	}, nil
}

func extractRuntimeConfig(args []string, ocConfig *config.OcConfig) ([]string, string, string, bool) {
	var whitelist []string
	var configEditor string
	var portsRange string
	allowMultiplePlugins := false
	if ocConfig != nil {
		whitelist = ocConfig.Plugins
		configEditor = ocConfig.Editor
		portsRange = ocConfig.Ports
		allowMultiplePlugins = ocConfig.AllowMultiplePlugins
	}

	if launch.HasPortFlag(args) {
		portsRange = ""
	}

	return whitelist, configEditor, portsRange, allowMultiplePlugins
}

func readConfigContent(deps RuntimeDeps, configPath string) ([]byte, error) {
	content, err := deps.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("opencode.json not found at %s", configPath)
		}
		return nil, fmt.Errorf("failed to read opencode.json: %w", err)
	}
	return content, nil
}

func visiblePlugins(deps RuntimeDeps, content []byte, whitelist []string) ([]config.Plugin, error) {
	plugins, _, err := deps.ParsePlugins(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plugins: %w", err)
	}
	visible, _ := deps.FilterByWhitelist(plugins, whitelist)
	return visible, nil
}

func buildPluginItems(visible []config.Plugin) []tui.PluginItem {
	items := make([]tui.PluginItem, len(visible))
	for i, p := range visible {
		items[i] = tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled}
	}
	return items
}

func buildEditChoices(paths runtimePaths) []tui.EditChoice {
	return []tui.EditChoice{
		{Label: "1) .oc file", Path: paths.ocConfigPath},
		{Label: "2) opencode.json file", Path: paths.configPath},
		{Label: "3) oh-my-opencode.json file", Path: ResolveOhMyOpencodePath(paths.configDir)},
	}
}

func persistSelections(deps RuntimeDeps, configPath string, content []byte, selections map[string]bool) error {
	modified, err := deps.ApplySelections(content, selections)
	if err != nil {
		return fmt.Errorf("failed to apply selections: %w", err)
	}
	if err := deps.WriteConfigFile(configPath, modified); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

func refreshSelectedSession(deps RuntimeDeps, cwd string, current tui.SessionItem) tui.SessionItem {
	refreshedSessions, err := deps.ListSessions(cwd)
	if err != nil {
		return current
	}
	latest := session.Latest(refreshedSessions)
	if latest.ID == "" {
		return current
	}
	return latest
}

func runOpencode(r RunnerAPI, args []string, portArgs []string, selectedSession tui.SessionItem, selections map[string]bool, sendToast func(int, []string) error) error {
	args = appendSessionArgs(args, selectedSession)
	args = append(args, portArgs...)
	plugins := selectedPluginNames(selections)

	var onStart func()
	if port, ok := launch.Port(args); ok && sendToast != nil {
		onStart = func() {
			if err := sendToast(port, plugins); err != nil {
				logToastFailure(port, err)
			}
		}
	}

	return r.Run(args, onStart)
}

func selectedPluginNames(selections map[string]bool) []string {
	var enabled []string
	for name, selected := range selections {
		if selected {
			enabled = append(enabled, name)
		}
	}
	sort.Strings(enabled)
	return enabled
}

func runLaunchTUI(plugins []string, selectedSession tui.SessionItem, portsRange string, deps RuntimeDeps, version string) ([]string, error) {
	launchModel := tui.NewLaunchModel(plugins, selectedSession, version, func(msgCh chan<- tea.Msg) {
		defer close(msgCh)
		portArgs := launch.ResolvePortArgs(portsRange, deps.ParsePortRange, deps.SelectPort, deps.IsPortAvailable, func(line string) {
			msgCh <- tui.LaunchLogMsg{Line: line}
		})
		msgCh <- tui.LaunchReadyMsg{PortArgs: portArgs}
	})

	launchResult, err := tea.NewProgram(launchModel, tea.WithoutRenderer()).Run()
	if err != nil {
		return nil, err
	}

	finalLaunchModel, ok := launchResult.(tui.LaunchModel)
	if !ok {
		return nil, fmt.Errorf("unexpected launch TUI model type %T", launchResult)
	}

	return finalLaunchModel.PortArgs(), nil
}

func ResolveOhMyOpencodePath(configDir string) string {
	return resolveOhMyOpencodePath(configDir, os.Stat)
}

func resolveOhMyOpencodePath(configDir string, statFn func(string) (os.FileInfo, error)) string {
	jsonPath := filepath.Join(configDir, "oh-my-opencode.json")
	if _, err := statFn(jsonPath); err == nil {
		return jsonPath
	}

	jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
	if _, err := statFn(jsoncPath); err == nil {
		return jsoncPath
	}

	return jsonPath
}

func logToastFailure(port int, err error) {
	fmt.Fprintf(os.Stderr, "oc: toast failed on port %d: %v\n", port, err)
}

func appendSessionArgs(args []string, selectedSession tui.SessionItem) []string {
	if selectedSession.ID == "" || hasSessionArgs(args) {
		return append([]string(nil), args...)
	}

	result := append([]string(nil), args...)
	result = append(result, "-s", selectedSession.ID)
	return result
}

func hasSessionArgs(args []string) bool {
	for _, arg := range args {
		if arg == "-s" || arg == "--session" || arg == "-c" || arg == "--continue" {
			return true
		}
		if strings.HasPrefix(arg, "--session=") || strings.HasPrefix(arg, "-s=") {
			return true
		}
		if strings.HasPrefix(arg, "--continue=") || strings.HasPrefix(arg, "-c=") {
			return true
		}
	}
	return false
}
