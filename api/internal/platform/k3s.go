package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// K3sManager manages the k3s Kubernetes cluster on Linux
type K3sManager struct {
	platform      Platform
	vibespaceHome string
	binDir        string
}

// NewK3sManager creates a new k3s cluster manager
func NewK3sManager(p Platform, vibespaceHome string) *K3sManager {
	return &K3sManager{
		platform:      p,
		vibespaceHome: vibespaceHome,
		binDir:        filepath.Join(vibespaceHome, "bin"),
	}
}

// Binary paths
func (m *K3sManager) k3sBin() string {
	return filepath.Join(m.binDir, "k3s")
}

func (m *K3sManager) kubectlBin() string {
	// k3s includes kubectl as "k3s kubectl"
	return m.k3sBin()
}

// IsInstalled checks if k3s is installed
func (m *K3sManager) IsInstalled() (bool, error) {
	if _, err := os.Stat(m.k3sBin()); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

// Install downloads k3s binary
func (m *K3sManager) Install(ctx context.Context) error {
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	return m.downloadK3s(ctx)
}

func (m *K3sManager) downloadK3s(ctx context.Context) error {
	// k3s releases: https://github.com/k3s-io/k3s/releases
	// Asset naming: k3s (amd64), k3s-arm64
	assetName := "k3s"
	if m.platform.Arch == "arm64" {
		assetName = "k3s-arm64"
	}

	url, err := getGitHubReleaseAssetURL(ctx, "k3s-io", "k3s", assetName)
	if err != nil {
		return fmt.Errorf("failed to get k3s download URL: %w", err)
	}

	return downloadBinary(ctx, url, m.k3sBin())
}

// IsRunning checks if k3s is running
func (m *K3sManager) IsRunning() (bool, error) {
	// Check if k3s process is running via PID file
	pidFile := filepath.Join(m.vibespaceHome, "k3s.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return false, nil
	}

	pid := strings.TrimSpace(string(data))
	// Check if process exists
	if _, err := os.Stat(filepath.Join("/proc", pid)); err == nil {
		return true, nil
	}

	return false, nil
}

// Start starts k3s server
func (m *K3sManager) Start(ctx context.Context) error {
	// Create data directory
	dataDir := filepath.Join(m.vibespaceHome, "k3s-data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	kubeconfig := m.KubeconfigPath()
	pidFile := filepath.Join(m.vibespaceHome, "k3s.pid")
	logFile := filepath.Join(m.vibespaceHome, "k3s.log")

	// Start k3s server in background
	args := []string{
		"server",
		"--data-dir", dataDir,
		"--write-kubeconfig", kubeconfig,
		"--write-kubeconfig-mode", "644",
		"--disable", "traefik", // We'll install our own Traefik
	}

	cmd := exec.Command(m.k3sBin(), args...)

	// Redirect output to log file
	log, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start k3s: %w", err)
	}

	// Save PID
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
}

// Stop stops k3s server
func (m *K3sManager) Stop(ctx context.Context) error {
	pidFile := filepath.Join(m.vibespaceHome, "k3s.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return nil // Not running
	}

	pid := strings.TrimSpace(string(data))

	// Kill the process
	cmd := exec.Command("kill", pid)
	_ = cmd.Run()

	// Clean up PID file
	_ = os.Remove(pidFile)

	return nil
}

// WaitReady waits for kubectl to be able to connect
func (m *K3sManager) WaitReady(ctx context.Context) error {
	kubeconfig := m.KubeconfigPath()

	for i := 0; i < 60; i++ {
		// k3s includes kubectl functionality
		cmd := exec.CommandContext(ctx, m.k3sBin(),
			"kubectl",
			"--kubeconfig", kubeconfig,
			"cluster-info",
		)
		if err := cmd.Run(); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return fmt.Errorf("timeout waiting for cluster")
}

// Uninstall removes k3s and all data
func (m *K3sManager) Uninstall(ctx context.Context) error {
	// Stop if running
	_ = m.Stop(ctx)

	// Remove data directory
	dataDir := filepath.Join(m.vibespaceHome, "k3s-data")
	_ = os.RemoveAll(dataDir)

	// Remove binaries and files
	_ = os.Remove(m.k3sBin())
	_ = os.Remove(m.KubeconfigPath())
	_ = os.Remove(filepath.Join(m.vibespaceHome, "k3s.pid"))
	_ = os.Remove(filepath.Join(m.vibespaceHome, "k3s.log"))

	return nil
}

// KubeconfigPath returns the path to the kubeconfig file
func (m *K3sManager) KubeconfigPath() string {
	return filepath.Join(m.vibespaceHome, "kubeconfig")
}
