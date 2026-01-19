package platform

import (
	"context"
	"fmt"
)

// ClusterManager defines the interface for managing the local Kubernetes cluster
type ClusterManager interface {
	// IsInstalled checks if the required binaries are installed
	IsInstalled() (bool, error)

	// Install downloads and installs the required binaries
	Install(ctx context.Context) error

	// IsRunning checks if the cluster is currently running
	IsRunning() (bool, error)

	// Start starts the cluster
	Start(ctx context.Context) error

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
		return NewK3sManager(p, vibespaceHome), nil
	default:
		return nil, fmt.Errorf("unsupported platform: %s", p.OS)
	}
}
