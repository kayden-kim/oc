package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadOcConfigValid tests loading valid TOML with plugins array
func TestLoadOcConfigValid(t *testing.T) {
	// Create a temporary TOML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `plugins = ["plugin-a", "plugin-b"]`
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test TOML file: %v", err)
	}

	config, err := LoadOcConfig(configPath)
	if err != nil {
		t.Fatalf("LoadOcConfig failed: %v", err)
	}

	if config == nil {
		t.Fatal("expected config to be non-nil, got nil")
	}

	if len(config.Plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(config.Plugins))
	}

	if config.Plugins[0] != "plugin-a" {
		t.Errorf("expected first plugin to be 'plugin-a', got '%s'", config.Plugins[0])
	}

	if config.Plugins[1] != "plugin-b" {
		t.Errorf("expected second plugin to be 'plugin-b', got '%s'", config.Plugins[1])
	}
}

// TestLoadOcConfigMissingFile tests that missing file returns nil, nil (not an error)
func TestLoadOcConfigMissingFile(t *testing.T) {
	nonexistentPath := "/nonexistent/path/to/config.toml"

	config, err := LoadOcConfig(nonexistentPath)

	if err != nil {
		t.Fatalf("LoadOcConfig should not return error for missing file, got: %v", err)
	}

	if config != nil {
		t.Errorf("expected nil config for missing file, got %+v", config)
	}
}

// TestLoadOcConfigEmptyPlugins tests empty plugins array
func TestLoadOcConfigEmptyPlugins(t *testing.T) {
	// Create a temporary TOML file with empty plugins array
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `plugins = []`
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test TOML file: %v", err)
	}

	config, err := LoadOcConfig(configPath)
	if err != nil {
		t.Fatalf("LoadOcConfig failed: %v", err)
	}

	if config == nil {
		t.Fatal("expected config to be non-nil, got nil")
	}

	if len(config.Plugins) != 0 {
		t.Errorf("expected 0 plugins, got %d", len(config.Plugins))
	}
}

// TestLoadOcConfigInvalidTOML tests invalid TOML syntax returns error
func TestLoadOcConfigInvalidTOML(t *testing.T) {
	// Create a temporary file with invalid TOML
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `plugins = ["plugin-a" invalid syntax here`
	err := os.WriteFile(configPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("failed to write test TOML file: %v", err)
	}

	config, err := LoadOcConfig(configPath)

	if err == nil {
		t.Fatal("expected LoadOcConfig to return error for invalid TOML, got nil")
	}

	if config != nil {
		t.Errorf("expected nil config when error occurs, got %+v", config)
	}
}
