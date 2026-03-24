package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ExitCodeError reports a command exit code without terminating the parent process.
type ExitCodeError struct {
	Code int
}

func (e *ExitCodeError) Error() string {
	return fmt.Sprintf("process exited with code %d", e.Code)
}

func IsExitCode(err error) (*ExitCodeError, bool) {
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr, true
	}
	return nil, false
}

// Runner is a wrapper around os/exec for launching the opencode CLI.
type Runner struct {
	Command string // The executable name to run (default: "opencode")
}

// NewRunner creates a new Runner with the default "opencode" command.
func NewRunner() *Runner {
	return &Runner{
		Command: "opencode",
	}
}

// CheckAvailable verifies that the command exists in PATH.
// Returns an error if the command is not found.
func (r *Runner) CheckAvailable() error {
	_, err := exec.LookPath(r.Command)
	if err != nil {
		return fmt.Errorf("command %q not found in PATH: %w", r.Command, err)
	}
	return nil
}

// Run executes the command with the provided arguments.
// All arguments are passed through unchanged.
// stdin, stdout, and stderr are connected directly to os.Stdin, os.Stdout, os.Stderr.
// If the command exits with a non-zero code, an ExitCodeError is returned.
// If an onStart callback was provided, it is invoked after the process starts successfully.
func (r *Runner) Run(args []string, onStart func()) error {
	cmd := exec.Command(r.Command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if onStart != nil {
		go onStart()
	}

	err := cmd.Wait()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitCodeError{Code: exitErr.ExitCode()}
		}
		return err
	}

	return nil
}
