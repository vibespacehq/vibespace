package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load loads configuration from the given path (or default location).
// Priority: configPath arg > VIBESPACE_CONFIG env > ~/.vibespace/config.yaml
// Starts from Default(), YAML merges on top, then env var overrides are applied.
func Load(configPath string) (*Config, error) {
	cfg := Default()

	path := resolveConfigPath(configPath)
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				// No config file — use defaults + env overrides
				applyEnvOverrides(cfg)
				if err := Validate(cfg); err != nil {
					return nil, fmt.Errorf("config validation: %w", err)
				}
				return cfg, nil
			}
			return nil, fmt.Errorf("reading config file %s: %w", path, err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", path, err)
		}
	}

	applyEnvOverrides(cfg)

	if err := Validate(cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return cfg, nil
}

// resolveConfigPath determines the config file path.
func resolveConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if envPath := os.Getenv("VIBESPACE_CONFIG"); envPath != "" {
		return envPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".vibespace", "config.yaml")
}
