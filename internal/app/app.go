package app

import (
	"fmt"

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

		outcome, err := resolveTUIOutcome(selections, cancelled, editTarget, portArgs, nextSession, lastExitErr)
		selectedSession = outcome.selectedSession
		if err != nil {
			return err
		}
		if outcome.stop {
			return nil
		}
		if outcome.editTarget != "" {
			if err := deps.OpenEditor(outcome.editTarget, state.configEditor); err != nil {
				return fmt.Errorf("failed to open editor for %s: %w", editTarget, err)
			}
			continue
		}

		if err := persistSelections(deps, state, outcome.selections); err != nil {
			return err
		}

		err = runOpencode(r, args, outcome.portArgs, selectedSession, outcome.selections, deps.SendToast)
		selectedSession = refreshSelectedSession(deps, state.cwd, selectedSession)
		exitErr, shouldContinue, err := resolveLaunchOutcome(err)
		if err != nil {
			return err
		}
		if shouldContinue {
			lastExitErr = exitErr
			continue
		}
		lastExitErr = nil
	}
}
