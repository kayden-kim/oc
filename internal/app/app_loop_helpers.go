package app

import (
	"fmt"
	"os"

	"github.com/kayden-kim/oc/internal/runner"
	"github.com/kayden-kim/oc/internal/tui"
)

type tuiOutcome struct {
	stop            bool
	editTarget      string
	selections      map[string]bool
	portArgs        []string
	selectedSession tui.SessionItem
}

func resolveTUIOutcome(selections map[string]bool, cancelled bool, editTarget string, portArgs []string, nextSession tui.SessionItem, lastExitErr *runner.ExitCodeError) (tuiOutcome, error) {
	outcome := tuiOutcome{
		editTarget:      editTarget,
		selections:      selections,
		portArgs:        portArgs,
		selectedSession: nextSession,
	}

	if cancelled {
		outcome.stop = true
		if lastExitErr != nil {
			return outcome, lastExitErr
		}
		return outcome, nil
	}

	return outcome, nil
}

func resolveLaunchOutcome(err error) (*runner.ExitCodeError, bool, error) {
	if exitErr, ok := runner.IsExitCode(err); ok {
		fmt.Fprintf(os.Stderr, "opencode exited with code %d\n\n", exitErr.Code)
		return exitErr, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return nil, false, nil
}
