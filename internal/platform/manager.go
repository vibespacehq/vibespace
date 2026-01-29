package platform

import (
	"context"
	"fmt"
)

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

// NewClusterManager creates the appropriate cluster manager for the platform
func NewClusterManager(p Platform, vibespaceHome string) (ClusterManager, error) {
	switch p.OS {
	case "darwin":
		return NewColimaManager(p, vibespaceHome), nil
	case "linux":
		return NewLimaManager(p, vibespaceHome), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", p.OS)
	}
}
