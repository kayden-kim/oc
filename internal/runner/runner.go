package runner

import (
	"fmt"
	"os"
	"os/exec"
)

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
// If the command exits with a non-zero code, os.Exit is called with that code.
func (r *Runner) Run(args []string) error {
	cmd := exec.Command(r.Command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		// Extract exit code and propagate via os.Exit
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			os.Exit(code)
		}
		// For other errors, return the error
		return err
	}

	return nil
}
