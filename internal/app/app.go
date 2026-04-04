package app

import (
	"fmt"
	"os"

	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/stats"
	"github.com/kayden-kim/oc/internal/tui"
)

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
	if hasContinueArgs(args) {
		return runContinuePath(args, deps, paths, r)
	}
	selectedSession := tui.SessionItem{}

	for {
		state, err := loadIterationState(args, deps, paths, selectedSession)
		if err != nil {
			return err
		}
		selectedSession = state.selectedSession

		selections, cancelled, editTarget, portArgs, nextSession, err := deps.RunTUI(
			state.mergedItems,
			buildEditChoices(paths, state.projectConfigPath, state.projectSource != nil),
			state.sessions,
			state.selectedSession,
			stats.Report{},
			stats.Report{},
			state.statsConfig,
			state.effectivePortsRange,
			state.allowMultiplePlugins,
		)
		if err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
		selectedSession = nextSession
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
