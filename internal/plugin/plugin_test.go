package plugin

import (
	"testing"

	"github.com/kayden-kim/oc/internal/config"
)

func TestFilterByWhitelist(t *testing.T) {
	tests := []struct {
		name        string
		plugins     []config.Plugin
		whitelist   []string
		wantVisible int
		wantHidden  int
		checkState  func(t *testing.T, visible, hidden []config.Plugin)
	}{
		{
			name: "nil whitelist shows all plugins as visible",
			plugins: []config.Plugin{
				{Name: "plugin-a", Enabled: true, LineIndex: 1, OriginalLine: `    "plugin-a",`},
				{Name: "plugin-b", Enabled: false, LineIndex: 2, OriginalLine: `    // "plugin-b",`},
				{Name: "plugin-c", Enabled: true, LineIndex: 3, OriginalLine: `    "plugin-c",`},
			},
			whitelist:   nil,
			wantVisible: 3,
			wantHidden:  0,
			checkState: func(t *testing.T, visible, hidden []config.Plugin) {
				// Verify state preservation
				if visible[0].LineIndex != 1 || visible[0].Enabled != true {
					t.Errorf("plugin-a state not preserved")
				}
				if visible[1].LineIndex != 2 || visible[1].Enabled != false {
					t.Errorf("plugin-b state not preserved")
				}
			},
		},
		{
			name: "empty whitelist hides all plugins",
			plugins: []config.Plugin{
				{Name: "plugin-a", Enabled: true, LineIndex: 1, OriginalLine: `    "plugin-a",`},
				{Name: "plugin-b", Enabled: false, LineIndex: 2, OriginalLine: `    // "plugin-b",`},
			},
			whitelist:   []string{},
			wantVisible: 0,
			wantHidden:  2,
			checkState: func(t *testing.T, visible, hidden []config.Plugin) {
				// Verify state preservation in hidden
				if hidden[0].Name != "plugin-a" || hidden[0].LineIndex != 1 {
					t.Errorf("hidden plugin-a state not preserved")
				}
			},
		},
		{
			name: "whitelist with matching plugins splits correctly",
			plugins: []config.Plugin{
				{Name: "plugin-a", Enabled: true, LineIndex: 1, OriginalLine: `    "plugin-a",`},
				{Name: "plugin-b", Enabled: true, LineIndex: 2, OriginalLine: `    "plugin-b",`},
				{Name: "plugin-c", Enabled: true, LineIndex: 3, OriginalLine: `    "plugin-c",`},
			},
			whitelist:   []string{"plugin-a", "plugin-c"},
			wantVisible: 2,
			wantHidden:  1,
			checkState: func(t *testing.T, visible, hidden []config.Plugin) {
				// Verify visible contains correct names
				visibleNames := map[string]bool{visible[0].Name: true, visible[1].Name: true}
				if !visibleNames["plugin-a"] || !visibleNames["plugin-c"] {
					t.Errorf("visible plugins are incorrect")
				}
				// Verify hidden
				if hidden[0].Name != "plugin-b" {
					t.Errorf("expected plugin-b in hidden, got %s", hidden[0].Name)
				}
			},
		},
		{
			name: "whitelist with non-existent names results in no visible",
			plugins: []config.Plugin{
				{Name: "plugin-a", Enabled: true, LineIndex: 1, OriginalLine: `    "plugin-a",`},
				{Name: "plugin-b", Enabled: true, LineIndex: 2, OriginalLine: `    "plugin-b",`},
			},
			whitelist:   []string{"plugin-x", "plugin-y"},
			wantVisible: 0,
			wantHidden:  2,
			checkState: func(t *testing.T, visible, hidden []config.Plugin) {
				if len(hidden) != 2 {
					t.Errorf("all plugins should be hidden, got %d", len(hidden))
				}
			},
		},
		{
			name: "case-sensitive matching (uppercase vs lowercase)",
			plugins: []config.Plugin{
				{Name: "Plugin", Enabled: true, LineIndex: 1, OriginalLine: `    "Plugin",`},
				{Name: "plugin", Enabled: true, LineIndex: 2, OriginalLine: `    "plugin",`},
				{Name: "PLUGIN", Enabled: true, LineIndex: 3, OriginalLine: `    "PLUGIN",`},
			},
			whitelist:   []string{"plugin"},
			wantVisible: 1,
			wantHidden:  2,
			checkState: func(t *testing.T, visible, hidden []config.Plugin) {
				if visible[0].Name != "plugin" {
					t.Errorf("expected 'plugin', got '%s'", visible[0].Name)
				}
				hiddenNames := map[string]bool{hidden[0].Name: true, hidden[1].Name: true}
				if !hiddenNames["Plugin"] || !hiddenNames["PLUGIN"] {
					t.Errorf("case sensitivity not working correctly")
				}
			},
		},
		{
			name: "state preservation across visible and hidden",
			plugins: []config.Plugin{
				{Name: "enabled-plugin", Enabled: true, LineIndex: 10, OriginalLine: `    "enabled-plugin",`},
				{Name: "disabled-plugin", Enabled: false, LineIndex: 11, OriginalLine: `    // "disabled-plugin",`},
			},
			whitelist:   []string{"enabled-plugin"},
			wantVisible: 1,
			wantHidden:  1,
			checkState: func(t *testing.T, visible, hidden []config.Plugin) {
				// Check visible
				if visible[0].Enabled != true || visible[0].LineIndex != 10 {
					t.Errorf("visible plugin state not preserved")
				}
				// Check hidden
				if hidden[0].Enabled != false || hidden[0].LineIndex != 11 {
					t.Errorf("hidden plugin state not preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visible, hidden := FilterByWhitelist(tt.plugins, tt.whitelist)

			if len(visible) != tt.wantVisible {
				t.Errorf("visible plugins count = %d, want %d", len(visible), tt.wantVisible)
			}
			if len(hidden) != tt.wantHidden {
				t.Errorf("hidden plugins count = %d, want %d", len(hidden), tt.wantHidden)
			}

			if tt.checkState != nil {
				tt.checkState(t, visible, hidden)
			}
		})
	}
}
