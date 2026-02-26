package config

import (
	"os"
	"strconv"
)

// applyEnvOverrides reads existing VIBESPACE_* environment variables and
// overrides the corresponding config values.
func applyEnvOverrides(cfg *Config) {
	// Resource defaults
	if v := os.Getenv("VIBESPACE_DEFAULT_CPU"); v != "" {
		cfg.Resources.CPU = v
	}
	if v := os.Getenv("VIBESPACE_DEFAULT_CPU_LIMIT"); v != "" {
		cfg.Resources.CPULimit = v
	}
	if v := os.Getenv("VIBESPACE_DEFAULT_MEMORY"); v != "" {
		cfg.Resources.Memory = v
	}
	if v := os.Getenv("VIBESPACE_DEFAULT_MEMORY_LIMIT"); v != "" {
		cfg.Resources.MemoryLimit = v
	}
	if v := os.Getenv("VIBESPACE_DEFAULT_STORAGE"); v != "" {
		cfg.Resources.Storage = v
	}

	// Cluster defaults
	if v := os.Getenv("VIBESPACE_CLUSTER_CPU"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Cluster.CPU = n
		}
	}
	if v := os.Getenv("VIBESPACE_CLUSTER_MEMORY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Cluster.Memory = n
		}
	}
	if v := os.Getenv("VIBESPACE_CLUSTER_DISK"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Cluster.Disk = n
		}
	}
}
