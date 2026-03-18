package config

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

var (
	pluginKeyPattern        = regexp.MustCompile(`"plugin"\s*:`)
	pluginArrayStartPattern = regexp.MustCompile(`"plugin"\s*:\s*\[`)
	pluginArrayEndPattern   = regexp.MustCompile(`^\s*\]`)
	activePluginPattern     = regexp.MustCompile(`^\s*"([^"]+)"`)
	commentedPluginPattern  = regexp.MustCompile(`^\s*//\s*"([^"]+)"`)

	// lastDetectedLineEnding is kept for future writer logic.
	lastDetectedLineEnding = "\n"
)

func ParsePlugins(content []byte) ([]Plugin, error) {
	lastDetectedLineEnding = detectLineEnding(content)

	plugins := make([]Plugin, 0)
	scanner := bufio.NewScanner(bytes.NewReader(content))

	inPluginArray := false
	foundPluginKey := false
	lineIndex := 0

	for scanner.Scan() {
		line := scanner.Text()

		if !inPluginArray {
			if pluginKeyPattern.MatchString(line) {
				if !pluginArrayStartPattern.MatchString(line) {
					return nil, fmt.Errorf("plugin key is not an array")
				}

				foundPluginKey = true
				inPluginArray = true

				arrayStart := strings.Index(line, "[")
				if arrayStart >= 0 && strings.Contains(line[arrayStart+1:], "]") {
					inPluginArray = false
				}
			}

			lineIndex++
			continue
		}

		if pluginArrayEndPattern.MatchString(line) {
			inPluginArray = false
			lineIndex++
			continue
		}

		if m := commentedPluginPattern.FindStringSubmatch(line); len(m) == 2 {
			plugins = append(plugins, Plugin{
				Name:         m[1],
				Enabled:      false,
				LineIndex:    lineIndex,
				OriginalLine: line,
			})
			lineIndex++
			continue
		}

		if m := activePluginPattern.FindStringSubmatch(line); len(m) == 2 {
			plugins = append(plugins, Plugin{
				Name:         m[1],
				Enabled:      true,
				LineIndex:    lineIndex,
				OriginalLine: line,
			})
		}

		lineIndex++
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if !foundPluginKey {
		return nil, fmt.Errorf("plugin key not found")
	}

	if inPluginArray {
		return nil, fmt.Errorf("plugin array not closed")
	}

	return plugins, nil
}

func detectLineEnding(content []byte) string {
	if bytes.Contains(content, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}
