package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// VMState represents the state of the cluster VM/service
type VMState int

const (
	VMStateNotExists VMState = iota // VM/service doesn't exist
	VMStateStopped                  // VM/service exists but is stopped
	VMStateRunning                  // VM/service is running
	VMStateBroken                   // VM/service is in a broken/inconsistent state
)

// ClusterMode represents how the cluster is managed
type ClusterMode string

const (
	ClusterModeColima    ClusterMode = "colima"
	ClusterModeLima      ClusterMode = "lima"
	ClusterModeBareMetal ClusterMode = "baremetal"
)

// ClusterState persists the cluster mode so subsequent commands
// automatically use the correct manager without flags.
type ClusterState struct {
	Mode ClusterMode `json:"mode"`
}

const clusterStateFile = "cluster.json"

// SaveClusterState persists the cluster mode to ~/.vibespace/cluster.json
func SaveClusterState(vibespaceHome string, mode ClusterMode) error {
	state := ClusterState{Mode: mode}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cluster state: %w", err)
	}
	path := filepath.Join(vibespaceHome, clusterStateFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write cluster state: %w", err)
	}
	return nil
}

// LoadClusterState reads the persisted cluster mode from ~/.vibespace/cluster.json
func LoadClusterState(vibespaceHome string) (*ClusterState, error) {
	path := filepath.Join(vibespaceHome, clusterStateFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state ClusterState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse cluster state: %w", err)
	}
	return &state, nil
}

// ClusterConfig contains configuration for cluster resources
type ClusterConfig struct {
	CPU    int // Number of CPU cores
	Memory int // Memory in GB
	Disk   int // Disk size in GB
}

// ClusterManager defines the interface for managing the local Kubernetes cluster
type ClusterManager interface {
	// IsInstalled checks if the required binaries are installed
	IsInstalled() (bool, error)

	// Install downloads and installs the required binaries
	Install(ctx context.Context) error

	// IsRunning checks if the cluster is currently running
	IsRunning() (bool, error)

	// GetVMState returns the current state of the cluster VM
	// Used for smart init logic (resume stopped, recover broken, create fresh)
	GetVMState(ctx context.Context) VMState

	// Start starts the cluster with the given configuration (creates fresh)
	Start(ctx context.Context, config ClusterConfig) error

	// Resume starts a stopped cluster without recreating it
	Resume(ctx context.Context) error

	// Recover cleans up a broken cluster state so Start can succeed
	Recover(ctx context.Context) error

	// Stop stops the cluster (preserves data)
	Stop(ctx context.Context) error

	// WaitReady waits for the cluster to be ready
	WaitReady(ctx context.Context) error

	// Uninstall removes the cluster and all data
	Uninstall(ctx context.Context) error

	// KubeconfigPath returns the path to the kubeconfig file
	KubeconfigPath() string
}

// ClusterManagerOptions configures which cluster manager to create
type ClusterManagerOptions struct {
	BareMetal bool // Use bare metal k3s (Linux only)
}

// NewClusterManager creates the appropriate cluster manager for the platform.
// It checks persisted cluster.json first, then falls back to platform
// detection combined with the provided options.
func NewClusterManager(p Platform, vibespaceHome string, opts ClusterManagerOptions) (ClusterManager, error) {
	// Check persisted cluster state first
	if state, err := LoadClusterState(vibespaceHome); err == nil {
		switch state.Mode {
		case ClusterModeBareMetal:
			return NewBareMetalManager(p, vibespaceHome), nil
		case ClusterModeColima:
			return NewColimaManager(p, vibespaceHome), nil
		case ClusterModeLima:
			return NewLimaManager(p, vibespaceHome), nil
		}
	}

	// Fall back to platform detection + options
	switch p.OS {
	case "darwin":
		return NewColimaManager(p, vibespaceHome), nil
	case "linux":
		if opts.BareMetal {
			return NewBareMetalManager(p, vibespaceHome), nil
		}
		return NewLimaManager(p, vibespaceHome), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", p.OS)
	}
}
