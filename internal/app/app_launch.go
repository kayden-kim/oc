package app

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kayden-kim/oc/internal/launch"
	"github.com/kayden-kim/oc/internal/tui"
)

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

func runContinuePath(args []string, deps RuntimeDeps, paths runtimePaths, r RunnerAPI) error {
	ocConfig, err := deps.LoadOcConfig(paths.ocConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load whitelist: %w", err)
	}

	_, _, effectivePortsRange, _, _ := extractRuntimeConfig(args, ocConfig)
	portArgs := launch.ResolvePortArgs(effectivePortsRange, deps.ParsePortRange, deps.SelectPort, deps.IsPortAvailable, nil)
	return runOpencode(r, args, portArgs, tui.SessionItem{}, nil, deps.SendToast)
}

func logToastFailure(port int, err error) {
	// Toast failures should never block launching opencode.
	// Log them directly where the callback runs.
	//
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

func hasContinueArgs(args []string) bool {
	for _, arg := range args {
		if arg == "-c" || arg == "--continue" {
			return true
		}
		if strings.HasPrefix(arg, "--continue=") || strings.HasPrefix(arg, "-c=") {
			return true
		}
	}
	return false
}
