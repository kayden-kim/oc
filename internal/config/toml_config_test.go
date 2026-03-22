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

// TestLoadOcConfigWithEditor tests loading TOML with editor field
func TestLoadOcConfigWithEditor(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `plugins = ["plugin-a"]
editor = "code --wait"`
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

	if config.Editor != "code --wait" {
		t.Errorf("expected editor to be 'code --wait', got '%s'", config.Editor)
	}
}

// TestLoadOcConfigWithoutEditor tests loading TOML without editor field
func TestLoadOcConfigWithoutEditor(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `plugins = ["plugin-a"]`
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

	if config.Editor != "" {
		t.Errorf("expected editor to be empty, got '%s'", config.Editor)
	}
}

// TestLoadOcConfigWithPorts tests loading TOML with ports field
func TestLoadOcConfigWithPorts(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `plugins = ["plugin-a"]
ports = "50000-55000"`
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

	if config.Ports != "50000-55000" {
		t.Errorf("expected ports to be '50000-55000', got '%s'", config.Ports)
	}
}

// TestLoadOcConfigWithoutPorts tests loading TOML without ports field
func TestLoadOcConfigWithoutPorts(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `plugins = ["plugin-a"]`
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

	if config.Ports != "" {
		t.Errorf("expected ports to be empty, got '%s'", config.Ports)
	}
}
