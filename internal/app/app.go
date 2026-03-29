package app

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/editor"
	"github.com/kayden-kim/oc/internal/launch"
	"github.com/kayden-kim/oc/internal/plugin"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/session"
	"github.com/kayden-kim/oc/internal/stats"
	"github.com/kayden-kim/oc/internal/tui"
)

type RunnerAPI interface {
	CheckAvailable() error
	Run(args []string, onStart func(context.Context)) error
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
	LoadGlobalStats   func(config.StatsConfig) (stats.Report, error)
	LoadProjectStats  func(string, config.StatsConfig) (stats.Report, error)
	LoadGlobalWindow  func(string, time.Time, time.Time) (stats.WindowReport, error)
	LoadProjectWindow func(string, string, time.Time, time.Time) (stats.WindowReport, error)
	RunTUI            func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, stats.Report, stats.Report, config.StatsConfig, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error)
	ApplySelections   func([]byte, map[string]bool) ([]byte, error)
	WriteConfigFile   func(string, []byte) error
	OpenEditor        func(string, string) error
	ParsePortRange    func(string) (int, int, error)
	SelectPort        func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, port int, available bool)) port.SelectResult
	IsPortAvailable   func(int) bool
	SendToast         func(context.Context, int, []string) error
}

type runtimePaths struct {
	ocConfigPath      string
	configDir         string
	configPath        string
	projectConfigPath string
}

type iterationState struct {
	cwd                  string
	sessions             []tui.SessionItem
	selectedSession      tui.SessionItem
	statsConfig          config.StatsConfig
	content              []byte
	visible              []config.Plugin
	userSource           *configSource
	projectSource        *configSource
	mergedItems          []tui.PluginItem
	mergedPlugins        []mergedPlugin
	configEditor         string
	effectivePortsRange  string
	allowMultiplePlugins bool
}

type configSource struct {
	path    string
	content []byte
	plugins []config.Plugin
}

type mergedPlugin struct {
	name      string
	inUser    bool
	inProject bool
}

const DefaultPortsRange = "55500-55555"

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
		LoadGlobalStats: func(statsConfig config.StatsConfig) (stats.Report, error) {
			return stats.LoadGlobalWithOptions(statsOptions(statsConfig))
		},
		LoadProjectStats: func(dir string, statsConfig config.StatsConfig) (stats.Report, error) {
			return stats.LoadForDirWithOptions(dir, statsOptions(statsConfig))
		},
		LoadGlobalWindow: func(label string, start, end time.Time) (stats.WindowReport, error) {
			return stats.LoadWindowReport("", label, start, end)
		},
		LoadProjectWindow: func(dir string, label string, start, end time.Time) (stats.WindowReport, error) {
			return stats.LoadWindowReport(dir, label, start, end)
		},
		ApplySelections: config.ApplySelections,
		WriteConfigFile: config.WriteConfigFile,
		OpenEditor:      editor.OpenWithConfig,
		ParsePortRange:  port.ParseRange,
		SelectPort:      port.Select,
		IsPortAvailable: port.IsAvailable,
		SendToast:       launch.SendToast,
	}

	deps.RunTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice, sessions []tui.SessionItem, selectedSession tui.SessionItem, globalStats stats.Report, projectStats stats.Report, statsConfig config.StatsConfig, portsRange string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		model := tui.NewModel(items, editChoices, sessions, selectedSession, globalStats, projectStats, statsConfig, version, allowMultiplePlugins).
			WithStatsLoaders(
				func() (stats.Report, error) {
					return deps.LoadGlobalStats(statsConfig)
				},
				func() (stats.Report, error) {
					cwd, err := deps.Getwd()
					if err != nil {
						return stats.Report{}, err
					}
					return deps.LoadProjectStats(cwd, statsConfig)
				},
				deps.LoadGlobalWindow,
				func(label string, start, end time.Time) (stats.WindowReport, error) {
					cwd, err := deps.Getwd()
					if err != nil {
						return stats.WindowReport{}, err
					}
					return deps.LoadProjectWindow(cwd, label, start, end)
				},
			)
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

		if len(state.mergedItems) == 0 {
			portArgs := launch.ResolvePortArgs(state.effectivePortsRange, deps.ParsePortRange, deps.SelectPort, deps.IsPortAvailable, nil)
			return runOpencode(r, args, portArgs, state.selectedSession, nil, deps.SendToast)
		}

		selections, cancelled, editTarget, portArgs, nextSession, err := deps.RunTUI(
			state.mergedItems,
			buildEditChoices(paths, paths.projectConfigPath, state.projectSource != nil),
			state.sessions,
			state.selectedSession,
			stats.Report{},
			stats.Report{},
			state.statsConfig,
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

		if err := persistSelections(deps, state, selections); err != nil {
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
	if deps.LoadGlobalStats == nil {
		deps.LoadGlobalStats = func(config.StatsConfig) (stats.Report, error) {
			return stats.Report{}, nil
		}
	}
	if deps.LoadProjectStats == nil {
		deps.LoadProjectStats = func(string, config.StatsConfig) (stats.Report, error) {
			return stats.Report{}, nil
		}
	}
	if deps.LoadGlobalWindow == nil {
		deps.LoadGlobalWindow = func(string, time.Time, time.Time) (stats.WindowReport, error) {
			return stats.WindowReport{}, nil
		}
	}
	if deps.LoadProjectWindow == nil {
		deps.LoadProjectWindow = func(string, string, time.Time, time.Time) (stats.WindowReport, error) {
			return stats.WindowReport{}, nil
		}
	}
	if deps.SendToast == nil {
		deps.SendToast = launch.SendToast
	}
	if deps.ParsePortRange == nil {
		deps.ParsePortRange = port.ParseRange
	}
	if deps.SelectPort == nil {
		deps.SelectPort = port.Select
	}
	if deps.IsPortAvailable == nil {
		deps.IsPortAvailable = port.IsAvailable
	}
	return deps
}

func resolveRuntimePaths(homeDir string) runtimePaths {
	return runtimePaths{
		ocConfigPath:      filepath.Join(homeDir, ".oc"),
		configDir:         filepath.Join(homeDir, ".config", "opencode"),
		configPath:        filepath.Join(homeDir, ".config", "opencode", "opencode.json"),
		projectConfigPath: "", // will be set in loadIterationState with cwd
	}
}

func readOptionalConfigContent(deps RuntimeDeps, path string) ([]byte, error) {
	content, err := deps.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return content, nil
}

func loadIterationState(args []string, deps RuntimeDeps, paths runtimePaths, selectedSession tui.SessionItem) (iterationState, error) {
	cwd, err := deps.Getwd()
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Set projectConfigPath now that we have cwd
	paths.projectConfigPath = filepath.Join(cwd, ".opencode", "opencode.json")

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

	whitelist, configEditor, effectivePortsRange, allowMultiplePlugins, statsConfig := extractRuntimeConfig(args, ocConfig)
	userContent, err := readConfigContent(deps, paths.configPath)
	if err != nil {
		return iterationState{}, err
	}
	projectContent, err := readOptionalConfigContent(deps, paths.projectConfigPath)
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to read project opencode.json: %w", err)
	}

	userPlugins, _, err := deps.ParsePlugins(userContent)
	if err != nil {
		return iterationState{}, fmt.Errorf("failed to parse plugins: %w", err)
	}
	visible, _ := deps.FilterByWhitelist(userPlugins, whitelist)

	userSource := &configSource{
		path:    paths.configPath,
		content: userContent,
		plugins: visible,
	}

	var projectSource *configSource
	if projectContent != nil {
		projectPlugins, _, parseErr := deps.ParsePlugins(projectContent)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse project config %s: %v\n", paths.projectConfigPath, parseErr)
		} else {
			projectVisible, _ := deps.FilterByWhitelist(projectPlugins, whitelist)
			projectSource = &configSource{
				path:    paths.projectConfigPath,
				content: projectContent,
				plugins: projectVisible,
			}
		}
	}

	mergedItems, mergedPlugins := mergePlugins(userSource, projectSource)

	return iterationState{
		cwd:                  cwd,
		sessions:             sessions,
		selectedSession:      selectedSession,
		statsConfig:          statsConfig,
		content:              userContent,
		visible:              visible,
		userSource:           userSource,
		projectSource:        projectSource,
		mergedItems:          mergedItems,
		mergedPlugins:        mergedPlugins,
		configEditor:         configEditor,
		effectivePortsRange:  effectivePortsRange,
		allowMultiplePlugins: allowMultiplePlugins,
	}, nil
}

func extractRuntimeConfig(args []string, ocConfig *config.OcConfig) ([]string, string, string, bool, config.StatsConfig) {
	var whitelist []string
	var configEditor string
	var portsRange string
	var statsConfig config.StatsConfig
	allowMultiplePlugins := false
	if ocConfig != nil {
		whitelist = ocConfig.Plugins
		configEditor = ocConfig.Editor
		portsRange = ocConfig.Ports
		allowMultiplePlugins = ocConfig.AllowMultiplePlugins
		statsConfig = ocConfig.Stats
	}

	if launch.HasPortFlag(args) {
		portsRange = ""
	} else if portsRange == "" {
		portsRange = DefaultPortsRange
	}

	return whitelist, configEditor, portsRange, allowMultiplePlugins, statsConfig
}

func statsOptions(cfg config.StatsConfig) stats.Options {
	return stats.Options{SessionGapMinutes: cfg.SessionGapMinutes}
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

func mergePlugins(userSource, projectSource *configSource) (items []tui.PluginItem, merged []mergedPlugin) {
	if userSource == nil {
		userSource = &configSource{}
	}

	if projectSource == nil {
		items = make([]tui.PluginItem, 0, len(userSource.plugins))
		merged = make([]mergedPlugin, 0, len(userSource.plugins))
		seen := make(map[string]int, len(userSource.plugins))
		for _, p := range userSource.plugins {
			if idx, ok := seen[p.Name]; ok {
				if p.Enabled {
					items[idx].InitiallyEnabled = true
				}
				continue
			}

			seen[p.Name] = len(items)
			items = append(items, tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled})
			merged = append(merged, mergedPlugin{name: p.Name, inUser: true})
		}
		return items, merged
	}

	capacity := len(userSource.plugins) + len(projectSource.plugins)
	items = make([]tui.PluginItem, 0, capacity)
	merged = make([]mergedPlugin, 0, capacity)
	seen := make(map[string]int, capacity)

	for _, p := range userSource.plugins {
		if idx, ok := seen[p.Name]; ok {
			merged[idx].inUser = true
			if p.Enabled {
				items[idx].InitiallyEnabled = true
			}
			continue
		}

		seen[p.Name] = len(items)
		items = append(items, tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled})
		merged = append(merged, mergedPlugin{name: p.Name, inUser: true})
	}

	for _, p := range projectSource.plugins {
		if idx, ok := seen[p.Name]; ok {
			merged[idx].inProject = true
			if p.Enabled {
				items[idx].InitiallyEnabled = true
			}
			continue
		}

		seen[p.Name] = len(items)
		items = append(items, tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled})
		merged = append(merged, mergedPlugin{name: p.Name, inProject: true})
	}

	for i, p := range merged {
		switch {
		case p.inUser && p.inProject:
			items[i].SourceLabel = "User, Project"
		case p.inUser:
			items[i].SourceLabel = "User"
		case p.inProject:
			items[i].SourceLabel = "Project"
		}
	}

	return items, merged
}

func buildEditChoices(paths runtimePaths, projectConfigPath string, projectConfigExists bool) []tui.EditChoice {
	choices := []tui.EditChoice{
		{Label: "1) .oc file", Path: paths.ocConfigPath},
		{Label: "2) opencode.json file", Path: paths.configPath},
		{Label: "3) oh-my-opencode.json file", Path: ResolveOhMyOpencodePath(paths.configDir)},
	}
	if projectConfigExists {
		choices = append(choices, tui.EditChoice{
			Label: "4) project opencode.json file",
			Path:  projectConfigPath,
		})
	}
	return choices
}

func persistSelections(deps RuntimeDeps, state iterationState, selections map[string]bool) error {
	userSelections := make(map[string]bool)
	projectSelections := make(map[string]bool)

	for _, plugin := range state.mergedPlugins {
		selected, ok := selections[plugin.name]
		if !ok {
			continue
		}

		if plugin.inUser {
			userSelections[plugin.name] = selected
		}
		if plugin.inProject {
			projectSelections[plugin.name] = selected
		}
	}

	if state.projectSource == nil && len(state.mergedPlugins) == 0 {
		maps.Copy(userSelections, selections)
	}

	if state.userSource != nil && len(userSelections) > 0 {
		modified, err := deps.ApplySelections(state.userSource.content, userSelections)
		if err != nil {
			return fmt.Errorf("failed to apply user selections: %w", err)
		}
		if err := deps.WriteConfigFile(state.userSource.path, modified); err != nil {
			return fmt.Errorf("failed to write user config: %w", err)
		}
	}

	if state.projectSource != nil && len(projectSelections) > 0 {
		modified, err := deps.ApplySelections(state.projectSource.content, projectSelections)
		if err != nil {
			return fmt.Errorf("failed to apply project selections: %w", err)
		}
		if err := deps.WriteConfigFile(state.projectSource.path, modified); err != nil {
			return fmt.Errorf("failed to write project config: %w", err)
		}
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

func runOpencode(r RunnerAPI, args []string, portArgs []string, selectedSession tui.SessionItem, selections map[string]bool, sendToast func(context.Context, int, []string) error) error {
	args = appendSessionArgs(args, selectedSession)
	args = append(args, portArgs...)
	plugins := selectedPluginNames(selections)

	var onStart func(context.Context)
	if port, ok := launch.Port(args); ok && sendToast != nil {
		onStart = func(ctx context.Context) {
			if err := sendToast(ctx, port, plugins); err != nil {
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

	launchResult, err := tea.NewProgram(launchModel, tea.WithoutRenderer(), tea.WithInput(nil)).Run()
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
	fmt.Fprintf(os.Stderr, "oc: error: show-toast failed on port %d: %v\n", port, err)
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
