package config

import (
	"strings"
	"testing"
)

func TestParsePlugins(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    []Plugin
		expectError string
	}{
		{
			name: "mixed active and commented plugins",
			content: `{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "oh-my-opencode",
    // "opencode-antigravity-auth@latest"
  ]
}`,
			expected: []Plugin{
				{Name: "oh-my-opencode", Enabled: true, LineIndex: 3, OriginalLine: `    "oh-my-opencode",`},
				{Name: "opencode-antigravity-auth@latest", Enabled: false, LineIndex: 4, OriginalLine: `    // "opencode-antigravity-auth@latest"`},
			},
		},
		{
			name: "empty plugin array",
			content: `{
  "plugin": []
}`,
			expected: []Plugin{},
		},
		{
			name: "all active plugins",
			content: `{
  "plugin": [
    "alpha",
    "beta"
  ]
}`,
			expected: []Plugin{
				{Name: "alpha", Enabled: true, LineIndex: 2, OriginalLine: `    "alpha",`},
				{Name: "beta", Enabled: true, LineIndex: 3, OriginalLine: `    "beta"`},
			},
		},
		{
			name: "all commented plugins",
			content: `{
  "plugin": [
    // "alpha",
    // "beta"
  ]
}`,
			expected: []Plugin{
				{Name: "alpha", Enabled: false, LineIndex: 2, OriginalLine: `    // "alpha",`},
				{Name: "beta", Enabled: false, LineIndex: 3, OriginalLine: `    // "beta"`},
			},
		},
		{
			name: "special characters in plugin names",
			content: `{
  "plugin": [
    "opencode-antigravity-auth@latest",
    "@scope/plugin"
  ]
}`,
			expected: []Plugin{
				{Name: "opencode-antigravity-auth@latest", Enabled: true, LineIndex: 2, OriginalLine: `    "opencode-antigravity-auth@latest",`},
				{Name: "@scope/plugin", Enabled: true, LineIndex: 3, OriginalLine: `    "@scope/plugin"`},
			},
		},
		{
			name: "comment spacing variants",
			content: `{
  "plugin": [
    //"alpha",
    // "beta",
    //  "gamma"
  ]
}`,
			expected: []Plugin{
				{Name: "alpha", Enabled: false, LineIndex: 2, OriginalLine: `    //"alpha",`},
				{Name: "beta", Enabled: false, LineIndex: 3, OriginalLine: `    // "beta",`},
				{Name: "gamma", Enabled: false, LineIndex: 4, OriginalLine: `    //  "gamma"`},
			},
		},
		{
			name: "trailing comma variations",
			content: `{
  "plugin": [
    "alpha",
    "beta",
    // "gamma"
  ]
}`,
			expected: []Plugin{
				{Name: "alpha", Enabled: true, LineIndex: 2, OriginalLine: `    "alpha",`},
				{Name: "beta", Enabled: true, LineIndex: 3, OriginalLine: `    "beta",`},
				{Name: "gamma", Enabled: false, LineIndex: 4, OriginalLine: `    // "gamma"`},
			},
		},
		{
			name: "windows line endings",
			content: "{\r\n" +
				"  \"plugin\": [\r\n" +
				"    \"alpha\",\r\n" +
				"    // \"beta\"\r\n" +
				"  ]\r\n" +
				"}",
			expected: []Plugin{
				{Name: "alpha", Enabled: true, LineIndex: 2, OriginalLine: `    "alpha",`},
				{Name: "beta", Enabled: false, LineIndex: 3, OriginalLine: `    // "beta"`},
			},
		},
		{
			name: "missing plugin key returns error",
			content: `{
  "mcp": []
}`,
			expectError: "plugin key not found",
		},
		{
			name: "plugin key is not an array returns error",
			content: `{
  "plugin": "alpha"
}`,
			expectError: "plugin key is not an array",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plugins, lineEnding, err := ParsePlugins([]byte(tt.content))
			if tt.expectError != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectError)
				}
				if !strings.Contains(err.Error(), tt.expectError) {
					t.Fatalf("expected error containing %q, got %q", tt.expectError, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("ParsePlugins() unexpected error: %v", err)
			}

			if len(plugins) != len(tt.expected) {
				t.Fatalf("expected %d plugins, got %d (%+v)", len(tt.expected), len(plugins), plugins)
			}

			for i := range tt.expected {
				exp := tt.expected[i]
				got := plugins[i]
				if got.Name != exp.Name || got.Enabled != exp.Enabled || got.LineIndex != exp.LineIndex || got.OriginalLine != exp.OriginalLine {
					t.Fatalf("plugin[%d] mismatch: expected %+v, got %+v", i, exp, got)
				}
			}

			// Verify line ending is detected correctly
			if strings.Contains(tt.content, "\r\n") {
				if lineEnding != "\r\n" {
					t.Fatalf("expected line ending \\r\\n, got %q", lineEnding)
				}
			} else if lineEnding != "\n" {
				t.Fatalf("expected line ending \\n, got %q", lineEnding)
			}
		})
	}
}
