package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultReturnsValidConfig(t *testing.T) {
	cfg := Default()
	if err := Validate(cfg); err != nil {
		t.Fatalf("Default() config should be valid: %v", err)
	}
}

func TestGlobalReturnsDefaultWhenNotLoaded(t *testing.T) {
	// Reset global
	SetGlobal(nil)
	cfg := Global()
	if cfg == nil {
		t.Fatal("Global() should return non-nil config")
	}
	if cfg.Resources.CPU != "250m" {
		t.Errorf("expected default CPU 250m, got %s", cfg.Resources.CPU)
	}
}

func TestPartialYAMLMerge(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Only override a few fields
	yaml := `
resources:
  cpu: "500m"
  memory: "1Gi"
theme:
  brand:
    teal: "#FF0000"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Overridden values
	if cfg.Resources.CPU != "500m" {
		t.Errorf("expected CPU 500m, got %s", cfg.Resources.CPU)
	}
	if cfg.Resources.Memory != "1Gi" {
		t.Errorf("expected Memory 1Gi, got %s", cfg.Resources.Memory)
	}
	if cfg.Theme.Brand.Teal != "#FF0000" {
		t.Errorf("expected teal #FF0000, got %s", cfg.Theme.Brand.Teal)
	}

	// Non-overridden values should keep defaults
	if cfg.Resources.CPULimit != "1000m" {
		t.Errorf("expected default CPULimit 1000m, got %s", cfg.Resources.CPULimit)
	}
	if cfg.Resources.Storage != "10Gi" {
		t.Errorf("expected default Storage 10Gi, got %s", cfg.Resources.Storage)
	}
	if cfg.Ports.SSH != 22 {
		t.Errorf("expected default SSH port 22, got %d", cfg.Ports.SSH)
	}
	if cfg.Theme.Brand.Pink != "#F102F3" {
		t.Errorf("expected default pink #F102F3, got %s", cfg.Theme.Brand.Pink)
	}
}

func TestEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	// Write a config that sets CPU to 500m
	if err := os.WriteFile(cfgPath, []byte("resources:\n  cpu: \"500m\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Env var should override YAML value
	t.Setenv("VIBESPACE_DEFAULT_CPU", "750m")
	t.Setenv("VIBESPACE_CLUSTER_CPU", "16")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Resources.CPU != "750m" {
		t.Errorf("expected env-overridden CPU 750m, got %s", cfg.Resources.CPU)
	}
	if cfg.Cluster.CPU != 16 {
		t.Errorf("expected env-overridden cluster CPU 16, got %d", cfg.Cluster.CPU)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load() with missing file should not error: %v", err)
	}
	// Should return defaults
	if cfg.Resources.CPU != "250m" {
		t.Errorf("expected default CPU 250m, got %s", cfg.Resources.CPU)
	}
}

func TestDurationYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")

	yaml := `
timeouts:
  daemon_socket: "30s"
  cluster_startup: "15m"
network:
  invite_token_ttl: "1h"
`
	if err := os.WriteFile(cfgPath, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Timeouts.DaemonSocket.Duration != 30*time.Second {
		t.Errorf("expected 30s, got %v", cfg.Timeouts.DaemonSocket.Duration)
	}
	if cfg.Timeouts.ClusterStartup.Duration != 15*time.Minute {
		t.Errorf("expected 15m, got %v", cfg.Timeouts.ClusterStartup.Duration)
	}
	if cfg.Network.InviteTokenTTL.Duration != time.Hour {
		t.Errorf("expected 1h, got %v", cfg.Network.InviteTokenTTL.Duration)
	}
}

func TestValidationCatchesBadValues(t *testing.T) {
	cfg := Default()

	// Bad port
	cfg.Ports.SSH = 0
	if err := Validate(cfg); err == nil {
		t.Error("expected validation error for port 0")
	}

	// Reset and test bad color
	cfg = Default()
	cfg.Theme.Brand.Teal = "not-a-color"
	if err := Validate(cfg); err == nil {
		t.Error("expected validation error for invalid hex color")
	}

	// Reset and test bad deployment strategy
	cfg = Default()
	cfg.Kubernetes.DeploymentStrategy = "Invalid"
	if err := Validate(cfg); err == nil {
		t.Error("expected validation error for invalid deployment strategy")
	}

	// Reset and test empty namespace
	cfg = Default()
	cfg.Kubernetes.Namespace = ""
	if err := Validate(cfg); err == nil {
		t.Error("expected validation error for empty namespace")
	}
}
