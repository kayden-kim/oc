package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type PluginConfig struct {
	Ports string `toml:"ports"`
}

// OcConfig represents the TOML configuration structure from ~/.oc
type OcConfig struct {
	Plugins              []string `toml:"plugins"`
	Editor               string   `toml:"editor"`
	Ports                string   `toml:"ports"`
	AllowMultiplePlugins bool     `toml:"allow_multiple_plugins"`
	PluginConfigs        map[string]PluginConfig
}

type ocTable struct {
	Plugins              []string `toml:"plugins"`
	Editor               string   `toml:"editor"`
	Ports                string   `toml:"ports"`
	AllowMultiplePlugins bool     `toml:"allow_multiple_plugins"`
}

type rawOcConfig struct {
	Plugins              []string                `toml:"plugins"`
	Editor               string                  `toml:"editor"`
	Ports                string                  `toml:"ports"`
	AllowMultiplePlugins bool                    `toml:"allow_multiple_plugins"`
	Oc                   ocTable                 `toml:"oc"`
	Plugin               map[string]PluginConfig `toml:"plugin"`
}

func hasOcTable(config rawOcConfig) bool {
	return config.Oc.Plugins != nil || config.Oc.Editor != "" || config.Oc.Ports != "" || config.Oc.AllowMultiplePlugins
}

// LoadOcConfig loads the TOML configuration from the specified path.
// If the file does not exist, it returns (nil, nil).
// If the file exists but has invalid TOML syntax, it returns (nil, error).
// If the file is valid, it returns the parsed OcConfig and nil error.
func LoadOcConfig(path string) (*OcConfig, error) {
	// Try to decode the TOML file
	var rawConfig rawOcConfig
	_, err := toml.DecodeFile(path, &rawConfig)

	// If the file doesn't exist, return nil, nil (not an error)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}

	// If there was any other error, return it
	if err != nil {
		return nil, err
	}

	config := &OcConfig{
		Plugins:              rawConfig.Plugins,
		Editor:               rawConfig.Editor,
		Ports:                rawConfig.Ports,
		AllowMultiplePlugins: rawConfig.AllowMultiplePlugins,
		PluginConfigs:        rawConfig.Plugin,
	}

	if hasOcTable(rawConfig) {
		config.Plugins = rawConfig.Oc.Plugins
		config.Editor = rawConfig.Oc.Editor
		config.Ports = rawConfig.Oc.Ports
		config.AllowMultiplePlugins = rawConfig.Oc.AllowMultiplePlugins
	}

	// Return the parsed config
	return config, nil
}
