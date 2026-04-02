package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kayden-kim/oc/internal/config"
	"github.com/kayden-kim/oc/internal/launch"
)

type runtimePaths struct {
	ocConfigPath      string
	configDir         string
	configPath        string
	projectConfigPath string
}

func resolveRuntimePaths(homeDir string) runtimePaths {
	return runtimePaths{
		ocConfigPath:      filepath.Join(homeDir, ".oc"),
		configDir:         filepath.Join(homeDir, ".config", "opencode"),
		configPath:        filepath.Join(homeDir, ".config", "opencode", "opencode.json"),
		projectConfigPath: "",
	}
}

func withProjectConfigPath(paths runtimePaths, cwd string) runtimePaths {
	paths.projectConfigPath = filepath.Join(cwd, ".opencode", "opencode.json")
	return paths
}

func extractRuntimeConfig(args []string, ocConfig *config.OcConfig) ([]string, string, string, bool, config.StatsConfig) {
	var whitelist []string
	var configEditor string
	var portsRange string
	var statsConfig config.StatsConfig
	allowMultiplePlugins := false
	if ocConfig != nil {
		whitelist = ocConfig.Plugins
		configEditor = ocConfig.Editor
		portsRange = ocConfig.Ports
		allowMultiplePlugins = ocConfig.AllowMultiplePlugins
		statsConfig = ocConfig.Stats
	}

	if launch.HasPortFlag(args) {
		portsRange = ""
	} else if portsRange == "" {
		portsRange = DefaultPortsRange
	}

	return whitelist, configEditor, portsRange, allowMultiplePlugins, statsConfig
}

func readConfigContent(deps RuntimeDeps, configPath string) ([]byte, error) {
	content, err := deps.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("opencode.json not found at %s", configPath)
		}
		return nil, fmt.Errorf("failed to read opencode.json: %w", err)
	}
	return content, nil
}

func readOptionalConfigContent(deps RuntimeDeps, path string) ([]byte, error) {
	content, err := deps.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return content, nil
}
