package app

import (
	"fmt"
	"os"

	"github.com/kayden-kim/oc/internal/runner"
)

func handleLaunchOutcome(err error) (*runner.ExitCodeError, bool, error) {
	if exitErr, ok := runner.IsExitCode(err); ok {
		fmt.Fprintf(os.Stderr, "opencode exited with code %d\n\n", exitErr.Code)
		return exitErr, true, nil
	}
	if err != nil {
		return nil, false, err
	}
	return nil, false, nil
}
