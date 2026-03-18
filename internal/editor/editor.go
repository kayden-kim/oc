package editor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Command describes an editor invocation.
type Command struct {
	Name string
	Args []string
}

// Open launches an editor for the given path and returns immediately.
func Open(path string) error {
	cmdSpec, err := CommandForPath(path)
	if err != nil {
		return err
	}

	cmd := exec.Command(cmdSpec.Name, cmdSpec.Args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start editor %q: %w", cmdSpec.Name, err)
	}

	return nil
}

// CommandForPath resolves the editor command for a config path.
func CommandForPath(path string) (Command, error) {
	for _, key := range []string{"OC_EDITOR", "EDITOR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			parts, err := splitCommand(value)
			if err != nil {
				return Command{}, fmt.Errorf("parse %s: %w", key, err)
			}
			return Command{Name: parts[0], Args: append(parts[1:], path)}, nil
		}
	}

	switch runtime.GOOS {
	case "windows":
		return Command{Name: "notepad", Args: []string{path}}, nil
	case "darwin":
		return Command{Name: "open", Args: []string{"-t", path}}, nil
	default:
		return Command{Name: "xdg-open", Args: []string{path}}, nil
	}
}

func splitCommand(input string) ([]string, error) {
	var parts []string
	var current strings.Builder
	var quote rune
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		parts = append(parts, current.String())
		current.Reset()
	}

	for _, r := range input {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t':
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escaped {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	flush()
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	return parts, nil
}
