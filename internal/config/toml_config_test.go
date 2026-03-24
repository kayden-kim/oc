package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeOcConfigFixture(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test TOML file: %v", err)
	}
	return configPath
}

func mustLoadOcConfig(t *testing.T, content string) *OcConfig {
	t.Helper()
	config, err := LoadOcConfig(writeOcConfigFixture(t, content))
	if err != nil {
		t.Fatalf("LoadOcConfig failed: %v", err)
	}
	if config == nil {
		t.Fatal("expected config to be non-nil, got nil")
	}
	return config
}

func TestLoadOcConfig_FieldParsing(t *testing.T) {
	tests := []struct {
		name                 string
		content              string
		wantPlugins          []string
		wantEditor           string
		wantAllowMultiPlugin bool
		wantPorts            string
	}{
		{
			name:                 "plugins list",
			content:              `plugins = ["plugin-a", "plugin-b"]`,
			wantPlugins:          []string{"plugin-a", "plugin-b"},
			wantEditor:           "",
			wantAllowMultiPlugin: false,
			wantPorts:            "",
		},
		{
			name:                 "empty plugins",
			content:              `plugins = []`,
			wantPlugins:          []string{},
			wantEditor:           "",
			wantAllowMultiPlugin: false,
			wantPorts:            "",
		},
		{
			name: "editor configured",
			content: `plugins = ["plugin-a"]
editor = "code --wait"`,
			wantPlugins:          []string{"plugin-a"},
			wantEditor:           "code --wait",
			wantAllowMultiPlugin: false,
			wantPorts:            "",
		},
		{
			name: "allow multiple plugins true",
			content: `plugins = ["plugin-a"]
allow_multiple_plugins = true`,
			wantPlugins:          []string{"plugin-a"},
			wantEditor:           "",
			wantAllowMultiPlugin: true,
			wantPorts:            "",
		},
		{
			name:                 "optional fields omitted use defaults",
			content:              `plugins = ["plugin-a"]`,
			wantPlugins:          []string{"plugin-a"},
			wantEditor:           "",
			wantAllowMultiPlugin: false,
			wantPorts:            "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := mustLoadOcConfig(t, tt.content)

			if len(config.Plugins) != len(tt.wantPlugins) {
				t.Fatalf("expected %d plugins, got %d", len(tt.wantPlugins), len(config.Plugins))
			}
			for i := range tt.wantPlugins {
				if config.Plugins[i] != tt.wantPlugins[i] {
					t.Fatalf("plugin[%d]: expected %q, got %q", i, tt.wantPlugins[i], config.Plugins[i])
				}
			}

			if config.Editor != tt.wantEditor {
				t.Fatalf("expected editor %q, got %q", tt.wantEditor, config.Editor)
			}
			if config.AllowMultiplePlugins != tt.wantAllowMultiPlugin {
				t.Fatalf("expected allow_multiple_plugins=%v, got %v", tt.wantAllowMultiPlugin, config.AllowMultiplePlugins)
			}
			if config.Ports != tt.wantPorts {
				t.Fatalf("expected ports %q, got %q", tt.wantPorts, config.Ports)
			}
		})
	}
}

func TestLoadOcConfig_IgnoresTopLevelPorts(t *testing.T) {
	config := mustLoadOcConfig(t, "plugins = [\"plugin-a\"]\nports = \"55000-55500\"")

	if config.Ports != "" {
		t.Fatalf("expected top-level ports to be ignored, got %q", config.Ports)
	}
}

func TestLoadOcConfig_MissingFile(t *testing.T) {
	nonexistentPath := "/nonexistent/path/to/config.toml"

	config, err := LoadOcConfig(nonexistentPath)
	if err != nil {
		t.Fatalf("LoadOcConfig should not return error for missing file, got: %v", err)
	}
	if config != nil {
		t.Fatalf("expected nil config for missing file, got %+v", config)
	}
}

func TestLoadOcConfig_InvalidTOML(t *testing.T) {
	configPath := writeOcConfigFixture(t, `plugins = ["plugin-a" invalid syntax here`)

	config, err := LoadOcConfig(configPath)
	if err == nil {
		t.Fatal("expected LoadOcConfig to return error for invalid TOML, got nil")
	}
	if config != nil {
		t.Fatalf("expected nil config when error occurs, got %+v", config)
	}
}

func TestLoadOcConfigSupportsOcSection(t *testing.T) {
	content := `[oc]
plugins = ["oh-my-opencode", "superpowers"]
allow_multiple_plugins = false
editor = "nvim"
ports = "55000-55500"

[plugin.oh-my-opencode]
ports = "55000-55500"`

	config := mustLoadOcConfig(t, content)

	if len(config.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(config.Plugins))
	}
	if config.Plugins[0] != "oh-my-opencode" || config.Plugins[1] != "superpowers" {
		t.Fatalf("unexpected plugins: %#v", config.Plugins)
	}
	if config.Editor != "nvim" {
		t.Fatalf("expected editor nvim, got %q", config.Editor)
	}
	if config.AllowMultiplePlugins {
		t.Fatal("expected allow_multiple_plugins to be false")
	}
	if config.Ports != "55000-55500" {
		t.Fatalf("expected oc ports 55000-55500, got %q", config.Ports)
	}
	pluginConfig, ok := config.PluginConfigs["oh-my-opencode"]
	if !ok {
		t.Fatal("expected plugin config for oh-my-opencode")
	}
	if pluginConfig.Ports != "55000-55500" {
		t.Fatalf("expected plugin ports 55000-55500, got %q", pluginConfig.Ports)
	}
}

func TestLoadOcConfigOcSectionOverridesFlatKeys(t *testing.T) {
	content := `plugins = ["top-level"]
editor = "vim"
allow_multiple_plugins = true

[oc]
plugins = ["section-plugin"]
editor = "nvim"
allow_multiple_plugins = false
ports = "55000-55500"`

	config := mustLoadOcConfig(t, content)

	if len(config.Plugins) != 1 || config.Plugins[0] != "section-plugin" {
		t.Fatalf("expected [oc] plugins to override flat keys, got %#v", config.Plugins)
	}
	if config.Editor != "nvim" {
		t.Fatalf("expected [oc] editor to override flat key, got %q", config.Editor)
	}
	if config.AllowMultiplePlugins {
		t.Fatal("expected [oc] allow_multiple_plugins to override flat key")
	}
	if config.Ports != "55000-55500" {
		t.Fatalf("expected [oc] ports to be loaded, got %q", config.Ports)
	}
}

func TestHasOcTableRecognizesPortsOnlyConfig(t *testing.T) {
	if !hasOcTable(rawOcConfig{Oc: ocTable{Ports: "55000-55500"}}) {
		t.Fatal("expected [oc].ports to count as an [oc] table")
	}
}
