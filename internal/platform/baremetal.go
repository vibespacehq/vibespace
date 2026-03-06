package platform

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// BareMetalManager manages k3s installed directly on a Linux host
type BareMetalManager struct {
	platform      Platform
	vibespaceHome string
	binDir        string
}

// Compile-time interface satisfaction check
var _ ClusterManager = (*BareMetalManager)(nil)

// NewBareMetalManager creates a new bare metal cluster manager
func NewBareMetalManager(p Platform, vibespaceHome string) *BareMetalManager {
	return &BareMetalManager{
		platform:      p,
		vibespaceHome: vibespaceHome,
		binDir:        filepath.Join(vibespaceHome, "bin"),
	}
}

func (m *BareMetalManager) kubectlBin() string {
	return filepath.Join(m.binDir, "kubectl")
}

// IsInstalled checks if k3s and kubectl are installed
func (m *BareMetalManager) IsInstalled() (bool, error) {
	if _, err := os.Stat("/usr/local/bin/k3s"); os.IsNotExist(err) {
		return false, nil
	}
	if _, err := os.Stat(m.kubectlBin()); os.IsNotExist(err) {
		return false, nil
	}
	return true, nil
}

// Install downloads kubectl and installs k3s via the official installer
func (m *BareMetalManager) Install(ctx context.Context) error {
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Download kubectl
	if err := m.downloadKubectl(ctx); err != nil {
		return fmt.Errorf("failed to download kubectl: %w", err)
	}

	// Install k3s via official installer with INSTALL_K3S_SKIP_START=true
	// so we can configure it before first start
	if err := m.installK3s(ctx); err != nil {
		return fmt.Errorf("failed to install k3s: %w", err)
	}

	return nil
}

func (m *BareMetalManager) downloadKubectl(ctx context.Context) error {
	version, err := getLatestKubectlVersion(ctx)
	if err != nil {
		version = "v1.29.0"
	}
	version = strings.TrimSpace(version)

	arch := m.platform.Arch

	url := fmt.Sprintf(
		"https://dl.k8s.io/release/%s/bin/linux/%s/kubectl",
		version, arch,
	)

	var expectedSHA256 string
	checksumContent, err := fetchURL(ctx, url+".sha256")
	if err != nil {
		slog.Warn("could not fetch kubectl checksum, skipping verification", "error", err)
	} else {
		expectedSHA256 = strings.Fields(checksumContent)[0]
	}

	return downloadBinary(ctx, url, m.kubectlBin(), expectedSHA256)
}

func (m *BareMetalManager) installK3s(ctx context.Context) error {
	// Download the k3s install script to a temp file
	installScript, err := fetchURL(ctx, "https://get.k3s.io")
	if err != nil {
		return fmt.Errorf("failed to download k3s installer: %w", err)
	}

	// Fetch the published SHA256 checksum and verify integrity
	checksumContent, err := fetchURL(ctx, "https://get.k3s.io/sha256sum")
	if err != nil {
		return fmt.Errorf("failed to fetch k3s installer checksum: %w", err)
	}
	expectedSHA256 := strings.Fields(checksumContent)[0]

	// Write installer to temp file for checksum verification
	tmpFile, err := os.CreateTemp("", "k3s-install-*.sh")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(installScript); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write installer to temp file: %w", err)
	}
	tmpFile.Close()

	if err := verifySHA256(tmpPath, expectedSHA256); err != nil {
		return fmt.Errorf("k3s installer integrity check failed: %w", err)
	}
	slog.Debug("k3s installer checksum verified", "sha256", expectedSHA256)

	// Run the verified installer via sudo
	slog.Debug("running k3s installer")
	cmd := exec.CommandContext(ctx, "sudo", "sh", tmpPath)
	cmd.Env = append(os.Environ(), "INSTALL_K3S_SKIP_START=true")
	stdout, stderr := bareMetalSubprocessWriters("k3s install")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("k3s installer failed: %w", err)
	}

	slog.Debug("k3s installed (service not started)")
	return nil
}

// GetVMState detects the current state of the k3s service
func (m *BareMetalManager) GetVMState(ctx context.Context) VMState {
	// Check if k3s service file exists
	if _, err := os.Stat("/etc/systemd/system/k3s.service"); os.IsNotExist(err) {
		return VMStateNotExists
	}

	detectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Check if k3s service is active
	cmd := exec.CommandContext(detectCtx, "systemctl", "is-active", "k3s")
	output, err := cmd.Output()
	if err != nil {
		slog.Debug("k3s service not active", "output", strings.TrimSpace(string(output)))
		return VMStateStopped
	}

	status := strings.TrimSpace(string(output))
	if status != "active" {
		slog.Debug("k3s service status", "status", status)
		return VMStateStopped
	}

	// Service is active — verify kubernetes is reachable
	kubectlCmd := exec.CommandContext(detectCtx, m.kubectlBin(), "--kubeconfig", m.KubeconfigPath(), "cluster-info")
	if err := kubectlCmd.Run(); err != nil {
		slog.Debug("k3s running but kubernetes unreachable, likely broken", "error", err)
		return VMStateBroken
	}

	return VMStateRunning
}

// IsRunning checks if k3s is running and kubernetes is reachable
func (m *BareMetalManager) IsRunning() (bool, error) {
	state := m.GetVMState(context.Background())
	return state == VMStateRunning, nil
}

// Start starts k3s with the given configuration
func (m *BareMetalManager) Start(ctx context.Context, config ClusterConfig) error {
	// Write k3s config with tls-san for WireGuard
	k3sConfig := `tls-san:
  - "10.100.0.1"
`
	if err := sudoRun(ctx, "mkdir", "-p", "/etc/rancher/k3s"); err != nil {
		return fmt.Errorf("failed to create k3s config dir: %w", err)
	}

	if err := sudoWrite(ctx, "/etc/rancher/k3s/config.yaml", k3sConfig); err != nil {
		return fmt.Errorf("failed to write k3s config: %w", err)
	}

	// Write systemd drop-in for resource limits
	dropInDir := "/etc/systemd/system/k3s.service.d"
	if err := sudoRun(ctx, "mkdir", "-p", dropInDir); err != nil {
		return fmt.Errorf("failed to create systemd drop-in dir: %w", err)
	}

	cpuQuota := fmt.Sprintf("%d%%", config.CPU*100)
	memoryMax := fmt.Sprintf("%dG", config.Memory)
	dropIn := fmt.Sprintf(`[Service]
CPUQuota=%s
MemoryMax=%s
`, cpuQuota, memoryMax)

	if err := sudoWrite(ctx, filepath.Join(dropInDir, "vibespace-resources.conf"), dropIn); err != nil {
		return fmt.Errorf("failed to write systemd drop-in: %w", err)
	}

	if config.Disk > 0 {
		slog.Warn("disk limit not enforced on bare metal", "disk_gb", config.Disk)
	}

	// Reload systemd and start k3s
	if err := sudoRun(ctx, "systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	slog.Debug("enabling and starting k3s")
	cmd := exec.CommandContext(ctx, "sudo", "systemctl", "enable", "--now", "k3s")
	stdout, stderr := bareMetalSubprocessWriters("systemctl enable --now k3s")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start k3s: %w", err)
	}

	// Wait for kubeconfig to appear, then copy it
	if err := m.waitAndCopyKubeconfig(ctx); err != nil {
		return fmt.Errorf("failed to copy kubeconfig: %w", err)
	}

	slog.Debug("bare metal k3s start completed")
	return nil
}

func (m *BareMetalManager) waitAndCopyKubeconfig(ctx context.Context) error {
	sourceKubeconfig := "/etc/rancher/k3s/k3s.yaml"
	destKubeconfig := m.KubeconfigPath()

	slog.Debug("waiting for kubeconfig", "source", sourceKubeconfig)

	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if _, err := os.Stat(sourceKubeconfig); err == nil {
			// Read via sudo since k3s.yaml is root-owned
			cmd := exec.CommandContext(ctx, "sudo", "cat", sourceKubeconfig)
			data, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("failed to read kubeconfig: %w", err)
			}

			if err := os.WriteFile(destKubeconfig, data, 0600); err != nil {
				return fmt.Errorf("failed to write kubeconfig: %w", err)
			}

			slog.Debug("kubeconfig copied successfully", "dest", destKubeconfig)
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timeout waiting for kubeconfig at %s", sourceKubeconfig)
}

// Resume starts a stopped k3s service
func (m *BareMetalManager) Resume(ctx context.Context) error {
	slog.Debug("starting k3s service")

	if err := sudoRun(ctx, "systemctl", "start", "k3s"); err != nil {
		return fmt.Errorf("failed to start k3s: %w", err)
	}

	// Refresh kubeconfig
	if err := m.waitAndCopyKubeconfig(ctx); err != nil {
		slog.Warn("failed to refresh kubeconfig after resume", "error", err)
	}

	slog.Debug("bare metal resume completed")
	return nil
}

// Recover cleans up a broken k3s state
func (m *BareMetalManager) Recover(ctx context.Context) error {
	slog.Info("recovering from broken bare metal state")

	// Stop k3s if running
	_ = sudoRun(ctx, "systemctl", "stop", "k3s")

	// Remove server state so k3s re-bootstraps
	_ = sudoRun(ctx, "rm", "-rf", "/var/lib/rancher/k3s/server")

	// Remove stale kubeconfig
	kubeconfig := m.KubeconfigPath()
	if _, err := os.Stat(kubeconfig); err == nil {
		slog.Debug("removing stale kubeconfig", "path", kubeconfig)
		_ = os.Remove(kubeconfig)
	}

	slog.Info("recovery cleanup completed")
	return nil
}

// Stop stops the k3s service gracefully, with kill fallback
func (m *BareMetalManager) Stop(ctx context.Context) error {
	slog.Debug("stopping k3s service")

	// Try graceful stop with 30s timeout
	stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(stopCtx, "sudo", "systemctl", "stop", "k3s")
	stdout, stderr := bareMetalSubprocessWriters("systemctl stop k3s")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if stopCtx.Err() == context.DeadlineExceeded {
			slog.Warn("k3s stop timed out, trying kill")
		} else {
			slog.Warn("k3s stop failed, trying kill", "error", err)
		}

		// Force kill
		if killErr := sudoRun(ctx, "systemctl", "kill", "k3s"); killErr != nil {
			return fmt.Errorf("failed to stop k3s: %w (kill also failed: %v)", err, killErr)
		}
		slog.Info("k3s force kill succeeded")
	}

	slog.Debug("k3s stop completed")
	return nil
}

// WaitReady waits for kubectl to be able to connect
func (m *BareMetalManager) WaitReady(ctx context.Context) error {
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

// Uninstall removes k3s using its built-in uninstall script
func (m *BareMetalManager) Uninstall(ctx context.Context) error {
	slog.Info("uninstalling bare metal k3s")

	// Stop if running
	_ = m.Stop(ctx)

	// Use the k3s-provided uninstall script
	uninstallScript := "/usr/local/bin/k3s-uninstall.sh"
	if _, err := os.Stat(uninstallScript); err == nil {
		slog.Debug("running k3s uninstall script")
		if err := sudoRun(ctx, uninstallScript); err != nil {
			slog.Warn("k3s uninstall script failed, doing manual cleanup", "error", err)
			m.manualCleanup(ctx)
		}
	} else {
		slog.Debug("k3s uninstall script not found, doing manual cleanup")
		m.manualCleanup(ctx)
	}

	// Remove stale kubeconfig
	kubeconfig := m.KubeconfigPath()
	if _, err := os.Stat(kubeconfig); err == nil {
		_ = os.Remove(kubeconfig)
	}

	slog.Info("bare metal k3s uninstalled")
	return nil
}

func (m *BareMetalManager) manualCleanup(ctx context.Context) {
	_ = sudoRun(ctx, "systemctl", "stop", "k3s")
	_ = sudoRun(ctx, "systemctl", "disable", "k3s")
	_ = sudoRun(ctx, "rm", "-f", "/etc/systemd/system/k3s.service")
	_ = sudoRun(ctx, "rm", "-rf", "/etc/systemd/system/k3s.service.d")
	_ = sudoRun(ctx, "rm", "-rf", "/var/lib/rancher/k3s")
	_ = sudoRun(ctx, "rm", "-rf", "/etc/rancher/k3s")
	_ = sudoRun(ctx, "rm", "-f", "/usr/local/bin/k3s")
	_ = sudoRun(ctx, "rm", "-f", "/usr/local/bin/k3s-uninstall.sh")
	_ = sudoRun(ctx, "systemctl", "daemon-reload")
}

// KubeconfigPath returns the path to the isolated kubeconfig file
func (m *BareMetalManager) KubeconfigPath() string {
	return filepath.Join(m.vibespaceHome, "kubeconfig")
}

// sudoRun runs a command with sudo
func sudoRun(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "sudo", args...)
	stdout, stderr := bareMetalSubprocessWriters("sudo " + args[0])
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// sudoWrite writes content to a file via sudo tee
func sudoWrite(ctx context.Context, path, content string) error {
	cmd := exec.CommandContext(ctx, "sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	cmd.Stdout = io.Discard
	stderr := &bareMetalLogWriter{prefix: "sudo tee", stream: "stderr"}
	cmd.Stderr = stderr
	return cmd.Run()
}

// bareMetalSubprocessWriters returns writers for subprocess stdout/stderr
func bareMetalSubprocessWriters(prefix string) (io.Writer, io.Writer) {
	if os.Getenv("VIBESPACE_DEBUG") == "" {
		return io.Discard, io.Discard
	}

	stdoutWriter := &bareMetalLogWriter{w: io.Discard, prefix: prefix, stream: "stdout"}
	stderrWriter := &bareMetalLogWriter{w: io.Discard, prefix: prefix, stream: "stderr"}
	return stdoutWriter, stderrWriter
}

// bareMetalLogWriter writes to both an underlying writer and slog
type bareMetalLogWriter struct {
	w      io.Writer
	prefix string
	stream string
	buf    []byte
}

func (w *bareMetalLogWriter) Write(p []byte) (n int, err error) {
	if w.w != nil {
		n, err = w.w.Write(p)
		if err != nil {
			return n, err
		}
	} else {
		n = len(p)
	}

	w.buf = append(w.buf, p...)
	for {
		idx := -1
		for i, b := range w.buf {
			if b == '\n' {
				idx = i
				break
			}
		}
		if idx < 0 {
			break
		}

		line := strings.TrimSpace(string(w.buf[:idx]))
		w.buf = w.buf[idx+1:]

		if line != "" {
			slog.Debug(w.prefix, "stream", w.stream, "line", line)
		}
	}

	return n, nil
}
