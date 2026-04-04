package app

import (
	"errors"
	"testing"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/port"
	"github.com/kayden-kim/oc/internal/runner"
)

func TestRunContinuePath_LoadsOcConfigAndUsesResolvedPortArgs(t *testing.T) {
	r := &fakeRunner{}
	loadCalls := 0
	parsedRange := ""

	err := runContinuePath([]string{"--continue=ses_manual"}, RuntimeDeps{
		LoadOcConfig: func(path string) (*config.OcConfig, error) {
			loadCalls++
			return &config.OcConfig{Ports: "60000-60010"}, nil
		},
		ParsePortRange: func(raw string) (int, int, error) {
			parsedRange = raw
			return 60000, 60010, nil
		},
		SelectPort: func(minPort, maxPort int, checkAvailable func(int) bool, logFn func(int, int, bool)) port.SelectResult {
			return port.SelectResult{Port: 60005, Attempts: 1, Found: true}
		},
		IsPortAvailable: func(int) bool { return true },
	}, resolveRuntimePaths(t.TempDir()), r)
	if err != nil {
		t.Fatalf("runContinuePath returned error: %v", err)
	}
	if loadCalls != 1 {
		t.Fatalf("expected LoadOcConfig to be called once, got %d", loadCalls)
	}
	if parsedRange != "60000-60010" {
		t.Fatalf("expected continue path to parse ports from oc config, got %q", parsedRange)
	}
	if !r.ran {
		t.Fatal("expected runner to execute in continue path")
	}
	if len(r.args) != 3 || r.args[0] != "--continue=ses_manual" || r.args[1] != "--port" || r.args[2] != "60005" {
		t.Fatalf("unexpected continue args: %#v", r.args)
	}
	if r.runCalls != 1 {
		t.Fatalf("expected runner to execute once, got %d", r.runCalls)
	}
}

func TestRunContinuePath_LoadOcConfigErrorMatchesExistingWrap(t *testing.T) {
	want := errors.New("bad config")
	err := runContinuePath(nil, RuntimeDeps{
		LoadOcConfig: func(string) (*config.OcConfig, error) {
			return nil, want
		},
	}, resolveRuntimePaths(t.TempDir()), &fakeRunner{})
	if !errors.Is(err, want) {
		t.Fatalf("expected wrapped load error, got %v", err)
	}
	if err == nil || err.Error() != "failed to load whitelist: bad config" {
		t.Fatalf("expected preserved error text, got %v", err)
	}
}

func TestHandleLaunchOutcome_ExitCodeKeepsLooping(t *testing.T) {
	lastExitErr, shouldContinue, err := handleLaunchOutcome(&runner.ExitCodeError{Code: 7})
	if err != nil {
		t.Fatalf("expected no terminal error, got %v", err)
	}
	if !shouldContinue {
		t.Fatal("expected exit code result to continue loop")
	}
	if lastExitErr == nil || lastExitErr.Code != 7 {
		t.Fatalf("expected last exit error to be preserved, got %+v", lastExitErr)
	}
}
