package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tidwall/jsonc"
)

func ApplySelections(content []byte, selections map[string]bool) ([]byte, error) {
	plugins, err := ParsePlugins(content)
	if err != nil {
		return nil, err
	}

	lineEnding := lastDetectedLineEnding
	if lineEnding == "" {
		lineEnding = "\n"
	}

	lines, hadTrailingLineEnding := splitContentLines(content, lineEnding)
	for _, plugin := range plugins {
		wantEnabled, ok := selections[plugin.Name]
		if !ok {
			continue
		}

		if plugin.LineIndex < 0 || plugin.LineIndex >= len(lines) {
			return nil, fmt.Errorf("plugin %q has out-of-range line index %d", plugin.Name, plugin.LineIndex)
		}

		if wantEnabled && !plugin.Enabled {
			lines[plugin.LineIndex] = removeCommentPrefix(lines[plugin.LineIndex])
			continue
		}

		if !wantEnabled && plugin.Enabled {
			lines[plugin.LineIndex] = addCommentPrefix(lines[plugin.LineIndex])
		}
	}

	result := joinContentLines(lines, lineEnding, hadTrailingLineEnding)
	if err := validateJSONC(result); err != nil {
		return nil, err
	}

	return result, nil
}

func WriteConfigFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "opencode.json.tmp.*")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		cleanup()
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		if _, statErr := os.Stat(path); statErr == nil {
			if removeErr := os.Remove(path); removeErr == nil {
				if retryErr := os.Rename(tmpPath, path); retryErr == nil {
					return nil
				}
			}
		}

		cleanup()
		return err
	}

	return nil
}

func splitContentLines(content []byte, lineEnding string) ([]string, bool) {
	asText := string(content)
	hadTrailing := strings.HasSuffix(asText, lineEnding)
	if hadTrailing {
		asText = strings.TrimSuffix(asText, lineEnding)
	}
	if asText == "" {
		return []string{""}, hadTrailing
	}
	return strings.Split(asText, lineEnding), hadTrailing
}

func joinContentLines(lines []string, lineEnding string, addTrailing bool) []byte {
	joined := strings.Join(lines, lineEnding)
	if addTrailing {
		joined += lineEnding
	}
	return []byte(joined)
}

func removeCommentPrefix(line string) string {
	indent, body := splitIndent(line)
	if !strings.HasPrefix(body, "//") {
		return line
	}

	body = strings.TrimPrefix(body, "//")
	body = strings.TrimLeft(body, " \t")
	return indent + body
}

func addCommentPrefix(line string) string {
	indent, body := splitIndent(line)
	body = strings.TrimPrefix(body, "//")
	body = strings.TrimLeft(body, " \t")
	return indent + "// " + body
}

func splitIndent(line string) (string, string) {
	idx := 0
	for idx < len(line) {
		if line[idx] != ' ' && line[idx] != '\t' {
			break
		}
		idx++
	}
	return line[:idx], line[idx:]
}

func validateJSONC(content []byte) error {
	jsonBytes := jsonc.ToJSON(content)
	var parsed any
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		return fmt.Errorf("invalid JSONC after modifications: %w", err)
	}
	return nil
}
