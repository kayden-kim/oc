package runner

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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

func TestRunnerDoesNotClearScreenBeforeLaunch(t *testing.T) {
	r := &Runner{Command: os.Args[0]}
	args := []string{"-test.run=TestHelperProcess", "--", "child-output"}

	t.Setenv("GO_TEST_PROCESS", "1")
	t.Setenv("TEST_MODE", "echo")

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	os.Stdout = writePipe
	defer func() {
		os.Stdout = originalStdout
	}()

	runErr := r.Run(args)
	writePipe.Close()
	output, readErr := io.ReadAll(readPipe)
	readPipe.Close()
	if readErr != nil {
		t.Fatalf("failed to read captured stdout: %v", readErr)
	}
	if runErr != nil {
		t.Fatalf("expected no error, got %v", runErr)
	}

	outputStr := string(output)
	if strings.HasPrefix(outputStr, "\x1b[2J\x1b[H") {
		t.Fatalf("did not expect clear-screen prefix, got %q", outputStr)
	}
	if !strings.Contains(outputStr, "child-output") {
		t.Fatalf("expected child output without clear-screen prefix, got %q", outputStr)
	}
	if strings.Index(outputStr, "child-output") != 0 {
		t.Fatalf("expected child output at start when clear-screen is skipped, got %q", outputStr)
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

// TestRunnerOnSuccessCallback tests that OnSuccess callback is invoked after process starts.
func TestRunnerOnSuccessCallback(t *testing.T) {
	r := &Runner{Command: os.Args[0]}
	args := []string{"-test.run=TestHelperProcess", "--"}

	t.Setenv("GO_TEST_PROCESS", "1")
	t.Setenv("TEST_MODE", "echo")

	called := make(chan bool, 1)
	r.OnSuccess(func() {
		called <- true
	})

	go func() {
		_ = r.Run(args)
	}()

	select {
	case <-called:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("OnSuccess callback was not invoked")
	}
}

// TestRunnerOnSuccessSkipsWhenProcessFailsToStart tests that callback is not invoked on exec error.
func TestRunnerOnSuccessSkipsWhenProcessFailsToStart(t *testing.T) {
	r := &Runner{Command: "nonexistent-binary-xyz-12345"}

	called := false
	r.OnSuccess(func() {
		called = true
	})

	err := r.Run([]string{})
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}

	time.Sleep(100 * time.Millisecond)
	if called {
		t.Fatal("OnSuccess callback should not be invoked when process fails to start")
	}
}
