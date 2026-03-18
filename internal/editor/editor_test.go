package editor

import (
	"runtime"
	"testing"
)

func TestCommandFromEnv_UsesOCEditorFirst(t *testing.T) {
	t.Setenv("OC_EDITOR", "custom-editor --flag")
	t.Setenv("EDITOR", "fallback-editor")

	cmd, err := CommandForPath("/tmp/opencode.json")
	if err != nil {
		t.Fatalf("CommandForPath returned error: %v", err)
	}
	if cmd.Name != "custom-editor" {
		t.Fatalf("expected custom-editor, got %q", cmd.Name)
	}
	if len(cmd.Args) != 2 || cmd.Args[0] != "--flag" || cmd.Args[1] != "/tmp/opencode.json" {
		t.Fatalf("unexpected args: %#v", cmd.Args)
	}
}

func TestCommandFromEnv_UsesEditorWhenOCEditorMissing(t *testing.T) {
	t.Setenv("OC_EDITOR", "")
	t.Setenv("EDITOR", "vim")

	cmd, err := CommandForPath("/tmp/opencode.json")
	if err != nil {
		t.Fatalf("CommandForPath returned error: %v", err)
	}
	if cmd.Name != "vim" {
		t.Fatalf("expected vim, got %q", cmd.Name)
	}
	if len(cmd.Args) != 1 || cmd.Args[0] != "/tmp/opencode.json" {
		t.Fatalf("unexpected args: %#v", cmd.Args)
	}
}

func TestCommandForPath_FallsBackByPlatform(t *testing.T) {
	t.Setenv("OC_EDITOR", "")
	t.Setenv("EDITOR", "")

	cmd, err := CommandForPath("/tmp/opencode.json")
	if err != nil {
		t.Fatalf("CommandForPath returned error: %v", err)
	}

	switch runtime.GOOS {
	case "windows":
		if cmd.Name != "notepad" {
			t.Fatalf("expected notepad, got %q", cmd.Name)
		}
	case "darwin":
		if cmd.Name != "open" {
			t.Fatalf("expected open, got %q", cmd.Name)
		}
		if len(cmd.Args) != 2 || cmd.Args[0] != "-t" || cmd.Args[1] != "/tmp/opencode.json" {
			t.Fatalf("unexpected args: %#v", cmd.Args)
		}
	default:
		if cmd.Name != "xdg-open" {
			t.Fatalf("expected xdg-open, got %q", cmd.Name)
		}
	}
}
