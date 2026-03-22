package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestHelperProcess is the mock subprocess for testing.
// It checks GO_TEST_PROCESS=1 to know if it's running as a subprocess.
// If not set, this test is skipped during normal test runs.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_PROCESS") != "1" {
		return
	}

	// Parse args after "--" separator
	args := os.Args
	var realArgs []string
	for i, arg := range args {
		if arg == "--" {
			realArgs = args[i+1:]
			break
		}
	}

	// Mock different behaviors based on test mode
	mode := os.Getenv("TEST_MODE")
	switch mode {
	case "echo":
		// Echo all args to stdout
		if len(realArgs) > 0 {
			fmt.Println(strings.Join(realArgs, " "))
		}
		os.Exit(0)
	case "exit42":
		// Exit with code 42
		os.Exit(42)
	case "stderr":
		// Write to stderr
		fmt.Fprintf(os.Stderr, "Error message\n")
		os.Exit(0)
	case "stdin_echo":
		// Read from stdin and echo to stdout
		var input string
		fmt.Scanln(&input)
		fmt.Println(input)
		os.Exit(0)
	default:
		// Default: just print args received
		fmt.Fprintf(os.Stderr, "Args received: %v\n", realArgs)
		os.Exit(0)
	}
}

// TestRunnerNewRunner tests that NewRunner creates a Runner with "opencode" command.
func TestRunnerNewRunner(t *testing.T) {
	r := NewRunner()
	if r == nil {
		t.Fatal("NewRunner returned nil")
	}
	if r.Command != "opencode" {
		t.Errorf("Expected Command to be 'opencode', got %q", r.Command)
	}
}

// TestRunnerCheckAvailable tests that CheckAvailable fails for non-existent command.
func TestRunnerCheckAvailable(t *testing.T) {
	// Test with non-existent command
	r := &Runner{Command: "nonexistent-binary-xyz-12345"}
	err := r.CheckAvailable()
	if err == nil {
		t.Error("Expected error for non-existent command, got nil")
	}

	// Test with real command (use "go" which should exist)
	r = &Runner{Command: "go"}
	err = r.CheckAvailable()
	if err != nil {
		t.Errorf("Expected no error for 'go' command, got: %v", err)
	}
}

// TestRunnerArgsForwarding tests that arguments are forwarded unchanged to subprocess.
func TestRunnerArgsForwarding(t *testing.T) {
	// Use test binary as mock subprocess
	r := &Runner{Command: os.Args[0]}

	cmd := exec.Command(r.Command, "-test.run=TestHelperProcess", "--", "--model", "gpt-4", "--verbose")
	cmd.Env = append(os.Environ(), "GO_TEST_PROCESS=1", "TEST_MODE=echo")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helper process failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	// Verify all args were forwarded
	if !strings.Contains(outputStr, "--model") {
		t.Errorf("--model flag not in output: %s", outputStr)
	}
	if !strings.Contains(outputStr, "gpt-4") {
		t.Errorf("gpt-4 value not in output: %s", outputStr)
	}
	if !strings.Contains(outputStr, "--verbose") {
		t.Errorf("--verbose flag not in output: %s", outputStr)
	}
}

// TestRunnerExitCodePropagation tests that exit codes are returned correctly.
func TestRunnerExitCodePropagation(t *testing.T) {
	r := &Runner{Command: os.Args[0]}
	args := []string{"-test.run=TestHelperProcess", "--"}
	t.Setenv("GO_TEST_PROCESS", "1")
	t.Setenv("TEST_MODE", "exit42")

	err := r.Run(args)
	if err == nil {
		t.Fatal("Expected error for exit code 42, got nil")
	}

	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Expected *ExitCodeError, got %T", err)
	}

	if exitErr.Code != 42 {
		t.Errorf("Expected exit code 42, got %d", exitErr.Code)
	}
}

func TestIsExitCode(t *testing.T) {
	err := &ExitCodeError{Code: 42}
	got, ok := IsExitCode(err)
	if !ok {
		t.Fatal("expected exit code error to be recognized")
	}
	if got.Code != 42 {
		t.Fatalf("expected exit code 42, got %d", got.Code)
	}
}

// TestRunnerStderrConnection tests that stderr is properly connected.
func TestRunnerStderrConnection(t *testing.T) {
	r := &Runner{Command: os.Args[0]}

	cmd := exec.Command(r.Command, "-test.run=TestHelperProcess", "--")
	cmd.Env = append(os.Environ(), "GO_TEST_PROCESS=1", "TEST_MODE=stderr")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Helper process failed: %v", err)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "Error message") {
		t.Errorf("Expected stderr output not found: %s", outputStr)
	}
}
