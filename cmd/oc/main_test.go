package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/kayden-kim/oc/internal/runner"
)

type exitPanic struct {
	code int
}

func runMainForTest(t *testing.T, args []string, appErr error) (int, string, string) {
	t.Helper()

	originalArgs := os.Args
	originalStdout := os.Stdout
	originalStderr := os.Stderr
	originalRunApp := runApp
	originalExitFunc := exitFunc

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	os.Args = append([]string{"oc"}, args...)
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	runApp = func([]string, string) error { return appErr }
	exitCode := -1
	exitFunc = func(code int) {
		exitCode = code
		panic(exitPanic{code: code})
	}

	defer func() {
		os.Args = originalArgs
		os.Stdout = originalStdout
		os.Stderr = originalStderr
		runApp = originalRunApp
		exitFunc = originalExitFunc
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitPanic); !ok {
					panic(r)
				}
			}
		}()
		main()
	}()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}

	stdout, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	stderr, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("read stderr: %v", err)
	}

	return exitCode, string(stdout), string(stderr)
}

func TestMain_PrintsVersionAndExitsZero(t *testing.T) {
	originalVersion := version
	version = "vtest"
	defer func() { version = originalVersion }()

	exitCode, stdout, stderr := runMainForTest(t, []string{"--version"}, nil)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if stdout != "vtest\n" {
		t.Fatalf("expected version output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestMain_MapsExitCodeError(t *testing.T) {
	exitCode, stdout, stderr := runMainForTest(t, nil, &runner.ExitCodeError{Code: 17})

	if exitCode != 17 {
		t.Fatalf("expected exit code 17, got %d", exitCode)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestMain_PrintsGenericErrorAndExitsOne(t *testing.T) {
	exitCode, stdout, stderr := runMainForTest(t, []string{"--model", "gpt-5"}, errors.New("boom"))

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !bytes.Contains([]byte(stderr), []byte("Error: boom")) {
		t.Fatalf("expected generic error output, got %q", stderr)
	}
}
