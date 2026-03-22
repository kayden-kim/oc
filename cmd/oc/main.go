package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/editor"
	"github.com/kayden-kim/oc/internal/plugin"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/tui"
)

var version = "v0.1.1" // Overridden by ldflags at build time

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
	runTUI            func([]tui.PluginItem, []tui.EditChoice) (map[string]bool, bool, string, error)
	applySelections   func([]byte, map[string]bool) ([]byte, error)
	writeConfigFile   func(string, []byte) error
	openEditor        func(string, string) error
	parsePortRange    func(string) (int, int, error)
	selectPort        func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(attempt, port int, available bool)) port.SelectResult
	isPortAvailable   func(int) bool
}

func defaultDeps() runtimeDeps {
	return runtimeDeps{
		newRunner:         func() runnerAPI { return runner.NewRunner() },
		userHomeDir:       os.UserHomeDir,
		readFile:          os.ReadFile,
		loadOcConfig:      config.LoadOcConfig,
		parsePlugins:      config.ParsePlugins,
		filterByWhitelist: plugin.FilterByWhitelist,
		runTUI: func(items []tui.PluginItem, editChoices []tui.EditChoice) (map[string]bool, bool, string, error) {
			model := tui.NewModel(items, editChoices, version)
			result, err := tea.NewProgram(model).Run()
			if err != nil {
				return nil, false, "", err
			}
			finalModel, ok := result.(tui.Model)
			if !ok {
				return nil, false, "", fmt.Errorf("unexpected TUI model type %T", result)
			}
			return finalModel.Selections(), finalModel.Cancelled(), finalModel.EditTarget(), nil
		},
		applySelections: config.ApplySelections,
		writeConfigFile: config.WriteConfigFile,
		openEditor:      editor.OpenWithConfig,
		parsePortRange:  port.ParseRange,
		selectPort:      port.Select,
		isPortAvailable: port.IsAvailable,
	}
}

func run() error {
	return runWithDeps(os.Args[1:], defaultDeps())
}

func runWithDeps(args []string, deps runtimeDeps) error {
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

	for {
		ocConfig, err := deps.loadOcConfig(ocConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load whitelist: %w", err)
		}

		var whitelist []string
		var configEditor string
		var portsRange string
		if ocConfig != nil {
			whitelist = ocConfig.Plugins
			configEditor = ocConfig.Editor
			portsRange = ocConfig.Ports
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
			return runOpencode(r, args, portsRange, deps)
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

		selections, cancelled, editTarget, err := deps.runTUI(items, editChoices)
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

		modified, err := deps.applySelections(content, selections)
		if err != nil {
			return fmt.Errorf("failed to apply selections: %w", err)
		}
		if err := deps.writeConfigFile(configPath, modified); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		printSelectedPlugins(selections)

		err = runOpencode(r, args, portsRange, deps)
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

func runOpencode(r runnerAPI, args []string, portsRange string, deps runtimeDeps) error {
	if portsRange == "" {
		return r.Run(args)
	}

	minPort, maxPort, err := deps.parsePortRange(portsRange)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: invalid ports config %q: %v\n", portsRange, err)
		fmt.Fprintln(os.Stderr, "Launching opencode without --port flag.")
		fmt.Println()
		return r.Run(args)
	}

	fmt.Printf("Port selection: range %d-%d\n", minPort, maxPort)

	result := deps.selectPort(minPort, maxPort, deps.isPortAvailable, func(attempt, p int, available bool) {
		status := "in use"
		if available {
			status = "available"
		}
		fmt.Printf("  [%2d/15] port %d ... %s\n", attempt, p, status)
	})

	if !result.Found {
		fmt.Fprintln(os.Stderr, "Warning: no available port found after 15 attempts.")
		fmt.Fprintln(os.Stderr, "Launching opencode without --port flag.")
		fmt.Println()
		return r.Run(args)
	}

	fmt.Printf("Using port %d\n", result.Port)
	fmt.Println()

	args = append(args, "--port", strconv.Itoa(result.Port))
	return r.Run(args)
}

func printSelectedPlugins(selections map[string]bool) {
	var enabled []string
	for name, selected := range selections {
		if selected {
			enabled = append(enabled, name)
		}
	}
	sort.Strings(enabled)

	if len(enabled) == 0 {
		fmt.Println("Selected plugins: (none)")
	} else {
		fmt.Println("Selected plugins:")
		for _, name := range enabled {
			fmt.Printf("  - %s\n", name)
		}
	}
	fmt.Println()
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
