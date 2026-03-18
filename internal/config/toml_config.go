package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

// OcConfig represents the TOML configuration structure from ~/.oc
type OcConfig struct {
	Plugins []string `toml:"plugins"`
}

// LoadOcConfig loads the TOML configuration from the specified path.
// If the file does not exist, it returns (nil, nil).
// If the file exists but has invalid TOML syntax, it returns (nil, error).
// If the file is valid, it returns the parsed OcConfig and nil error.
func LoadOcConfig(path string) (*OcConfig, error) {
	// Try to decode the TOML file
	var config OcConfig
	err := toml.DecodeFile(path, &config)

	// If the file doesn't exist, return nil, nil (not an error)
	if err != nil && os.IsNotExist(err) {
		return nil, nil
	}

	// If there was any other error, return it
	if err != nil {
		return nil, err
	}

	// Return the parsed config
	return &config, nil
}
