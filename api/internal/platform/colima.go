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

// ColimaManager manages the Colima-based Kubernetes cluster on macOS
type ColimaManager struct {
	platform      Platform
	vibespaceHome string
	binDir        string
}

// NewColimaManager creates a new Colima cluster manager
func NewColimaManager(p Platform, vibespaceHome string) *ColimaManager {
	return &ColimaManager{
		platform:      p,
		vibespaceHome: vibespaceHome,
		binDir:        filepath.Join(vibespaceHome, "bin"),
	}
}

// Binary paths
func (m *ColimaManager) colimaBin() string {
	return filepath.Join(m.binDir, "colima")
}

func (m *ColimaManager) kubectlBin() string {
	return filepath.Join(m.binDir, "kubectl")
}

// IsInstalled checks if Colima and kubectl are installed
func (m *ColimaManager) IsInstalled() (bool, error) {
	// Check for colima binary
	if _, err := os.Stat(m.colimaBin()); os.IsNotExist(err) {
		return false, nil
	}

	// Check for kubectl binary
	if _, err := os.Stat(m.kubectlBin()); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

// Install downloads Colima and kubectl
func (m *ColimaManager) Install(ctx context.Context) error {
	// Ensure bin directory exists
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Download Colima
	if err := m.downloadColima(ctx); err != nil {
		return fmt.Errorf("failed to download Colima: %w", err)
	}

	// Download kubectl
	if err := m.downloadKubectl(ctx); err != nil {
		return fmt.Errorf("failed to download kubectl: %w", err)
	}

	return nil
}

func (m *ColimaManager) downloadColima(ctx context.Context) error {
	// Colima releases: https://github.com/abiosoft/colima/releases
	version := "0.8.1"
	arch := m.platform.Arch
	if arch == "amd64" {
		arch = "x86_64"
	} else if arch == "arm64" {
		arch = "aarch64"
	}

	url := fmt.Sprintf(
		"https://github.com/abiosoft/colima/releases/download/v%s/colima-Darwin-%s",
		version, arch,
	)

	return downloadBinary(ctx, url, m.colimaBin())
}

func (m *ColimaManager) downloadKubectl(ctx context.Context) error {
	// kubectl releases: https://dl.k8s.io
	version := "1.29.0"
	arch := m.platform.Arch

	url := fmt.Sprintf(
		"https://dl.k8s.io/release/v%s/bin/darwin/%s/kubectl",
		version, arch,
	)

	return downloadBinary(ctx, url, m.kubectlBin())
}

// IsRunning checks if the Colima VM is running
func (m *ColimaManager) IsRunning() (bool, error) {
	cmd := exec.Command(m.colimaBin(), "status", "--profile", "vibespace")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// "colima status" returns non-zero if not running
		return false, nil
	}
	return strings.Contains(string(output), "Running"), nil
}

// Start starts the Colima VM with k3s
func (m *ColimaManager) Start(ctx context.Context) error {
	args := []string{
		"start",
		"--profile", "vibespace",
		"--kubernetes",
		"--kubernetes-version", "v1.29.0+k3s1",
		"--cpu", "2",
		"--memory", "4",
		"--disk", "20",
		"--network-address", // Enable network access
	}

	cmd := exec.CommandContext(ctx, m.colimaBin(), args...)
	cmd.Env = append(os.Environ(), "PATH="+m.binDir+":"+os.Getenv("PATH"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Colima: %w", err)
	}

	// Copy kubeconfig to vibespace location
	return m.copyKubeconfig()
}

func (m *ColimaManager) copyKubeconfig() error {
	// Colima stores kubeconfig in ~/.colima/vibespace/kubeconfig.yaml
	home, _ := os.UserHomeDir()
	colimaKubeconfig := filepath.Join(home, ".colima", "vibespace", "kubeconfig.yaml")

	// Wait for kubeconfig to exist
	for i := 0; i < 30; i++ {
		if _, err := os.Stat(colimaKubeconfig); err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	data, err := os.ReadFile(colimaKubeconfig)
	if err != nil {
		return fmt.Errorf("failed to read Colima kubeconfig: %w", err)
	}

	vibespaceKubeconfig := m.KubeconfigPath()
	if err := os.WriteFile(vibespaceKubeconfig, data, 0600); err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

// Stop stops the Colima VM
func (m *ColimaManager) Stop(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, m.colimaBin(), "stop", "--profile", "vibespace")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// WaitReady waits for kubectl to be able to connect
func (m *ColimaManager) WaitReady(ctx context.Context) error {
	kubeconfig := m.KubeconfigPath()

	for i := 0; i < 60; i++ {
		cmd := exec.CommandContext(ctx, m.kubectlBin(),
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

// Uninstall removes Colima and all data
func (m *ColimaManager) Uninstall(ctx context.Context) error {
	// Stop if running
	_ = m.Stop(ctx)

	// Delete the Colima profile
	cmd := exec.CommandContext(ctx, m.colimaBin(), "delete", "--profile", "vibespace", "--force")
	_ = cmd.Run()

	// Remove binaries
	_ = os.Remove(m.colimaBin())
	_ = os.Remove(m.kubectlBin())
	_ = os.Remove(m.KubeconfigPath())

	return nil
}

// KubeconfigPath returns the path to the kubeconfig file
func (m *ColimaManager) KubeconfigPath() string {
	return filepath.Join(m.vibespaceHome, "kubeconfig")
}
