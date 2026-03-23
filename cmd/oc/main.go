package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/editor"
	"github.com/kayden-kim/oc/internal/plugin"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/tui"
)

var version = "v0.1.5" // Overridden by ldflags at build time

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		os.Exit(0)
	}
	if err := run(); err != nil {
		if exitErr, ok := runner.IsExitCode(err); ok {
			os.Exit(exitErr.Code)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type runnerAPI interface {
	CheckAvailable() error
	Run(args []string) error
}

type runtimeDeps struct {
	newRunner         func() runnerAPI
	userHomeDir       func() (string, error)
	readFile          func(string) ([]byte, error)
	loadOcConfig      func(string) (*config.OcConfig, error)
	parsePlugins      func([]byte) ([]config.Plugin, string, error)
	filterByWhitelist func([]config.Plugin, []string) ([]config.Plugin, []config.Plugin)
	getwd             func() (string, error)
	listSessions      func(string) ([]tui.SessionItem, error)
	runTUI            func([]tui.PluginItem, []tui.EditChoice, []tui.SessionItem, tui.SessionItem, string, bool) (map[string]bool, bool, string, []string, tui.SessionItem, error)
	applySelections   func([]byte, map[string]bool) ([]byte, error)
	writeConfigFile   func(string, []byte) error
	openEditor        func(string, string) error
	parsePortRange    func(string) (int, int, error)
	selectPort        func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, port int, available bool)) port.SelectResult
	isPortAvailable   func(int) bool
}

func defaultDeps() runtimeDeps {
	deps := runtimeDeps{
		newRunner:         func() runnerAPI { return runner.NewRunner() },
		userHomeDir:       os.UserHomeDir,
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: plugin.FilterByWhitelist,
		getwd:             os.Getwd,
		listSessions:      listSessions,
		applySelections:   config.ApplySelections,
		writeConfigFile:   config.WriteConfigFile,
		openEditor:        editor.OpenWithConfig,
		parsePortRange:    port.ParseRange,
		selectPort:        port.Select,
		isPortAvailable:   port.IsAvailable,
	}

	deps.runTUI = func(items []tui.PluginItem, editChoices []tui.EditChoice, sessions []tui.SessionItem, session tui.SessionItem, portsRange string, allowMultiplePlugins bool) (map[string]bool, bool, string, []string, tui.SessionItem, error) {
		model := tui.NewModel(items, editChoices, sessions, session, version, allowMultiplePlugins)
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

		effectivePortsRange := ""
		if isPluginSelected(selections, ohMyOpencodePluginName) {
			effectivePortsRange = portsRange
		}

		portArgs, err := runLaunchTUI(selectedPluginNames(selections), finalModel.SelectedSession(), effectivePortsRange, deps)
		if err != nil {
			return nil, false, "", nil, tui.SessionItem{}, err
		}

		return selections, false, "", portArgs, finalModel.SelectedSession(), nil
	}

	return deps
}

const ohMyOpencodePluginName = "oh-my-opencode"

func run() error {
	return runWithDeps(os.Args[1:], defaultDeps())
}

func runWithDeps(args []string, deps runtimeDeps) error {
	if deps.getwd == nil {
		deps.getwd = os.Getwd
	}
	if deps.listSessions == nil {
		deps.listSessions = func(string) ([]tui.SessionItem, error) {
			return nil, nil
		}
	}

	r := deps.newRunner()
	if err := r.CheckAvailable(); err != nil {
		return fmt.Errorf("opencode not found: %w", err)
	}

	var lastExitErr *runner.ExitCodeError

	homeDir, err := deps.userHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	ocConfigPath := filepath.Join(homeDir, ".oc")
	configDir := filepath.Join(homeDir, ".config", "opencode")
	configPath := filepath.Join(configDir, "opencode.json")
	selectedSession := tui.SessionItem{}

	for {
		cwd, err := deps.getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		sessions, err := deps.listSessions(cwd)
		if err != nil {
			sessions = nil
		}
		if selectedSession.ID == "" {
			selectedSession = latestSession(sessions)
		}

		ocConfig, err := deps.loadOcConfig(ocConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load whitelist: %w", err)
		}

		var whitelist []string
		var configEditor string
		var pluginConfigs map[string]config.PluginConfig
		allowMultiplePlugins := false
		if ocConfig != nil {
			whitelist = ocConfig.Plugins
			configEditor = ocConfig.Editor
			pluginConfigs = ocConfig.PluginConfigs
			allowMultiplePlugins = ocConfig.AllowMultiplePlugins
		}

		content, err := deps.readFile(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("opencode.json not found at %s", configPath)
			}
			return fmt.Errorf("failed to read opencode.json: %w", err)
		}

		plugins, _, err := deps.parsePlugins(content)
		if err != nil {
			return fmt.Errorf("failed to parse plugins: %w", err)
		}

		visible, _ := deps.filterByWhitelist(plugins, whitelist)
		if len(visible) == 0 {
			portArgs, err := runLaunchTUI(nil, selectedSession, "", deps)
			if err != nil {
				return fmt.Errorf("launch TUI error: %w", err)
			}
			return runOpencode(r, args, portArgs, selectedSession)
		}

		items := make([]tui.PluginItem, len(visible))
		for i, p := range visible {
			items[i] = tui.PluginItem{Name: p.Name, InitiallyEnabled: p.Enabled}
		}
		editChoices := []tui.EditChoice{
			{Label: "1) .oc file", Path: ocConfigPath},
			{Label: "2) opencode.json file", Path: configPath},
			{Label: "3) oh-my-opencode.json file", Path: resolveOhMyOpencodePath(configDir)},
		}

		omoPortsRange := ohMyOpencodePortsRange(pluginConfigs)

		var selections map[string]bool
		var cancelled bool
		var editTarget string
		var portArgs []string
		selections, cancelled, editTarget, portArgs, selectedSession, err = deps.runTUI(items, editChoices, sessions, selectedSession, omoPortsRange, allowMultiplePlugins)
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
			if err := deps.openEditor(editTarget, configEditor); err != nil {
				return fmt.Errorf("failed to open editor for %s: %w", editTarget, err)
			}
			continue
		}
		if !isPluginSelected(selections, ohMyOpencodePluginName) {
			portArgs = nil
		}

		modified, err := deps.applySelections(content, selections)
		if err != nil {
			return fmt.Errorf("failed to apply selections: %w", err)
		}
		if err := deps.writeConfigFile(configPath, modified); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		err = runOpencode(r, args, portArgs, selectedSession)
		cwd, cwdErr := deps.getwd()
		if cwdErr == nil {
			refreshedSessions, listErr := deps.listSessions(cwd)
			if listErr == nil {
				selectedSession = latestSession(refreshedSessions)
			}
		}
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

func runOpencode(r runnerAPI, args []string, portArgs []string, session tui.SessionItem) error {
	args = appendSessionArgs(args, session)
	args = append(args, portArgs...)
	return r.Run(args)
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

func pluginNameKey(name string) string {
	return plugin.ComparisonName(name)
}

func ohMyOpencodePortsRange(pluginConfigs map[string]config.PluginConfig) string {
	if pluginConfig, ok := pluginConfigs[ohMyOpencodePluginName]; ok {
		return pluginConfig.Ports
	}

	return ""
}

func isPluginSelected(selections map[string]bool, pluginName string) bool {
	for name, selected := range selections {
		if selected && pluginNameKey(name) == pluginName {
			return true
		}
	}

	return false
}

func runLaunchTUI(plugins []string, session tui.SessionItem, portsRange string, deps runtimeDeps) ([]string, error) {
	launchModel := tui.NewLaunchModel(plugins, session, version, func(msgCh chan<- tea.Msg) {
		defer close(msgCh)
		portArgs := resolvePortArgs(portsRange, deps, func(line string) {
			msgCh <- tui.LaunchLogMsg{Line: line}
		})
		msgCh <- tui.LaunchReadyMsg{PortArgs: portArgs}
	})

	launchResult, err := tea.NewProgram(launchModel).Run()
	if err != nil {
		return nil, err
	}

	finalLaunchModel, ok := launchResult.(tui.LaunchModel)
	if !ok {
		return nil, fmt.Errorf("unexpected launch TUI model type %T", launchResult)
	}

	return finalLaunchModel.PortArgs(), nil
}

func resolvePortArgs(portsRange string, deps runtimeDeps, logFn func(string)) []string {
	if portsRange == "" {
		return nil
	}

	minPort, maxPort, err := deps.parsePortRange(portsRange)
	if err != nil {
		if logFn != nil {
			logFn(fmt.Sprintf("Warning: invalid ports config %q: %v", portsRange, err))
			logFn("Launching opencode without --port flag.")
		}
		return nil
	}

	if logFn != nil {
		logFn(fmt.Sprintf("Port selection: range %d-%d", minPort, maxPort))
	}

	result := deps.selectPort(minPort, maxPort, deps.isPortAvailable, func(attempt, p int, available bool) {
		if logFn == nil {
			return
		}

		status := "in use"
		if available {
			status = "available"
		}
		logFn(fmt.Sprintf("  [%2d/15] port %d ... %s", attempt, p, status))
	})
	if !result.Found {
		if logFn != nil {
			logFn("Warning: no available port found after 15 attempts.")
			logFn("Launching opencode without --port flag.")
		}
		return nil
	}

	if logFn != nil {
		logFn(fmt.Sprintf("Using port %d", result.Port))
	}

	return []string{"--port", strconv.Itoa(result.Port)}
}

func resolveOhMyOpencodePath(configDir string) string {
	jsonPath := filepath.Join(configDir, "oh-my-opencode.json")
	if _, err := os.Stat(jsonPath); err == nil {
		return jsonPath
	}

	jsoncPath := filepath.Join(configDir, "oh-my-opencode.jsonc")
	if _, err := os.Stat(jsoncPath); err == nil {
		return jsoncPath
	}

	return jsonPath
}

type sessionRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Updated   int64  `json:"updated"`
	Directory string `json:"directory"`
}

func listSessions(dir string) ([]tui.SessionItem, error) {
	cmd := exec.Command("opencode", "session", "list", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var rows []sessionRow
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, err
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Updated > rows[j].Updated
	})

	var result []tui.SessionItem
	for _, row := range rows {
		if !sameDir(row.Directory, dir) {
			continue
		}
		result = append(result, tui.SessionItem{ID: row.ID, Title: row.Title, UpdatedAt: unixTimestampToTime(row.Updated)})
	}

	return result, nil
}

func unixTimestampToTime(value int64) time.Time {
	switch {
	case value >= 1_000_000_000_000_000_000 || value <= -1_000_000_000_000_000_000:
		return time.Unix(0, value).Local()
	case value >= 1_000_000_000_000_000 || value <= -1_000_000_000_000_000:
		return time.UnixMicro(value).Local()
	case value >= 1_000_000_000_000 || value <= -1_000_000_000_000:
		return time.UnixMilli(value).Local()
	default:
		return time.Unix(value, 0).Local()
	}
}

func latestSession(items []tui.SessionItem) tui.SessionItem {
	if len(items) == 0 {
		return tui.SessionItem{}
	}
	return items[0]
}

func sameDir(left string, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func appendSessionArgs(args []string, session tui.SessionItem) []string {
	if session.ID == "" || hasSessionArgs(args) {
		return append([]string(nil), args...)
	}

	result := append([]string(nil), args...)
	result = append(result, "-s", session.ID)
	return result
}

func hasSessionArgs(args []string) bool {
	for i, arg := range args {
		if arg == "-s" || arg == "--session" || arg == "-c" || arg == "--continue" {
			return true
		}
		if strings.HasPrefix(arg, "--session=") {
			return true
		}
		if arg == "-s=" || arg == "--continue=" {
			return true
		}
		if arg == "-s" && i < len(args)-1 {
			return true
		}
	}
	return false
}
