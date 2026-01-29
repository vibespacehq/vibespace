package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// limaInstanceName is the Lima instance name for vibespace
const limaInstanceName = "vibespace"

// LimaManager manages the Lima-based Kubernetes cluster on Linux
type LimaManager struct {
	platform      Platform
	vibespaceHome string
	binDir        string
}

// Compile-time interface satisfaction check
var _ ClusterManager = (*LimaManager)(nil)

// NewLimaManager creates a new Lima cluster manager
func NewLimaManager(p Platform, vibespaceHome string) *LimaManager {
	return &LimaManager{
		platform:      p,
		vibespaceHome: vibespaceHome,
		binDir:        filepath.Join(vibespaceHome, "bin"),
	}
}

// limaInstanceDir returns the path to the Lima instance directory
func (m *LimaManager) limaInstanceDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lima", limaInstanceName)
}

// Binary paths
func (m *LimaManager) limactlBin() string {
	return filepath.Join(m.limaBinDir(), "limactl")
}

func (m *LimaManager) kubectlBin() string {
	return filepath.Join(m.binDir, "kubectl")
}

func (m *LimaManager) limaBinDir() string {
	return filepath.Join(m.vibespaceHome, "lima", "bin")
}

// limaListEntry represents a single entry in limactl list --json output
type limaListEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "Running", "Stopped", "Broken"
	Dir    string `json:"dir"`
}

// GetVMState detects the current state of the Lima VM
func (m *LimaManager) GetVMState(ctx context.Context) VMState {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

	// Use a timeout context for state detection
	detectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Run limactl list --json to check VM status
	cmd := exec.CommandContext(detectCtx, m.limactlBin(), "list", "--json")
	cmd.Env = append(os.Environ(), "PATH="+newPath)
	output, err := cmd.Output()
	if err != nil {
		// If limactl list fails or times out, check for leftover Lima files
		if _, statErr := os.Stat(m.limaInstanceDir()); statErr == nil {
			slog.Debug("limactl list failed but Lima instance exists, likely broken", "error", err)
			return VMStateBroken
		}
		slog.Debug("limactl list failed, VM likely doesn't exist", "error", err)
		return VMStateNotExists
	}

	// Parse the JSON output - limactl list --json returns an array
	// Note: Lima 2.0+ returns empty output (not "[]") when no instances exist
	var entries []limaListEntry
	if len(output) == 0 {
		slog.Debug("limactl list returned empty output, no instances exist")
		return VMStateNotExists
	}
	if err := json.Unmarshal(output, &entries); err != nil {
		slog.Debug("failed to parse limactl list output", "error", err, "output", string(output))
		return VMStateBroken
	}

	// Find our instance in the list
	for _, entry := range entries {
		if entry.Name == limaInstanceName {
			switch entry.Status {
			case "Running":
				// Double-check kubernetes is actually reachable
				kubectlCmd := exec.CommandContext(detectCtx, m.kubectlBin(), "--kubeconfig", m.KubeconfigPath(), "cluster-info")
				if err := kubectlCmd.Run(); err != nil {
					slog.Debug("VM running but kubernetes unreachable, likely broken", "error", err)
					return VMStateBroken
				}
				return VMStateRunning
			case "Stopped":
				return VMStateStopped
			default:
				slog.Debug("unknown VM status", "status", entry.Status)
				return VMStateBroken
			}
		}
	}

	// Instance not in list - check for orphaned Lima files
	if _, err := os.Stat(m.limaInstanceDir()); err == nil {
		slog.Debug("VM not in limactl list but Lima instance exists, broken state")
		return VMStateBroken
	}

	return VMStateNotExists
}

// IsInstalled checks if Lima and kubectl are installed
func (m *LimaManager) IsInstalled() (bool, error) {
	// Check for limactl binary
	if _, err := os.Stat(m.limactlBin()); os.IsNotExist(err) {
		return false, nil
	}

	// Check for kubectl binary
	if _, err := os.Stat(m.kubectlBin()); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

// Install downloads QEMU, Lima, and kubectl
func (m *LimaManager) Install(ctx context.Context) error {
	// Ensure bin directory exists
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Download QEMU (required for Lima VM on Linux)
	if err := m.downloadQEMU(ctx); err != nil {
		return fmt.Errorf("failed to download QEMU: %w", err)
	}

	// Download Lima
	if err := m.downloadLima(ctx); err != nil {
		return fmt.Errorf("failed to download Lima: %w", err)
	}

	// Download kubectl
	if err := m.downloadKubectl(ctx); err != nil {
		return fmt.Errorf("failed to download kubectl: %w", err)
	}

	return nil
}

func (m *LimaManager) downloadLima(ctx context.Context) error {
	// Lima releases: https://github.com/lima-vm/lima/releases
	// Asset naming: lima-X.Y.Z-Linux-x86_64.tar.gz, lima-X.Y.Z-Linux-aarch64.tar.gz
	arch := m.platform.Arch
	if arch == "amd64" {
		arch = "x86_64"
	} else if arch == "arm64" {
		arch = "aarch64"
	}

	// Get the latest release to find the version number (needed for asset name)
	apiURL := "https://api.github.com/repos/lima-vm/lima/releases/latest"
	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch Lima release info: %w", err)
	}
	defer resp.Body.Close()

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return fmt.Errorf("failed to parse Lima release info: %w", err)
	}

	// Version from tag (e.g., "v2.0.3" -> "2.0.3")
	version := strings.TrimPrefix(release.TagName, "v")
	assetName := fmt.Sprintf("lima-%s-Linux-%s.tar.gz", version, arch)

	// Find the asset URL
	var assetURL string
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			assetURL = asset.BrowserDownloadURL
			break
		}
	}
	if assetURL == "" {
		return fmt.Errorf("Lima asset '%s' not found in release %s", assetName, release.TagName)
	}

	// Download and extract to ~/.vibespace/lima/
	limaDir := filepath.Join(m.vibespaceHome, "lima")
	if err := os.MkdirAll(limaDir, 0755); err != nil {
		return fmt.Errorf("failed to create lima directory: %w", err)
	}

	if err := downloadAndExtractTarGz(ctx, assetURL, limaDir); err != nil {
		return fmt.Errorf("failed to download and extract Lima: %w", err)
	}

	return nil
}

func (m *LimaManager) downloadKubectl(ctx context.Context) error {
	// kubectl releases: https://dl.k8s.io
	// Fetch latest stable version dynamically
	version, err := getLatestKubectlVersion(ctx)
	if err != nil {
		// Fallback to known good version
		version = "v1.29.0"
	}
	// version comes with newline, trim it
	version = strings.TrimSpace(version)

	arch := m.platform.Arch

	url := fmt.Sprintf(
		"https://dl.k8s.io/release/%s/bin/linux/%s/kubectl",
		version, arch,
	)

	return downloadBinary(ctx, url, m.kubectlBin())
}

// qemuBinDir returns the path to the QEMU binary directory
func (m *LimaManager) qemuBinDir() string {
	return filepath.Join(m.vibespaceHome, "qemu", "bin")
}

func (m *LimaManager) downloadQEMU(ctx context.Context) error {
	// QEMU releases hosted on vibespace GitHub releases
	// Asset naming: qemu-X.Y.Z-linux-x86_64.tar.gz, qemu-X.Y.Z-linux-aarch64.tar.gz
	const qemuVersion = "10.2.0"
	const repoOwner = "yagizdagabak"
	const repoName = "vibespace"

	arch := m.platform.Arch
	if arch == "amd64" {
		arch = "x86_64"
	} else if arch == "arm64" {
		arch = "aarch64"
	}

	assetName := fmt.Sprintf("qemu-%s-linux-%s.tar.gz", qemuVersion, arch)

	// Get the release asset URL
	assetURL, err := getGitHubReleaseAssetURL(ctx, repoOwner, repoName, "qemu-v"+qemuVersion, assetName)
	if err != nil {
		return fmt.Errorf("failed to get QEMU release URL: %w", err)
	}

	// Download and extract to ~/.vibespace/qemu/
	qemuDir := filepath.Join(m.vibespaceHome, "qemu")
	if err := os.MkdirAll(qemuDir, 0755); err != nil {
		return fmt.Errorf("failed to create qemu directory: %w", err)
	}

	if err := downloadAndExtractTarGz(ctx, assetURL, qemuDir); err != nil {
		return fmt.Errorf("failed to download and extract QEMU: %w", err)
	}

	slog.Debug("QEMU downloaded", "version", qemuVersion, "arch", arch)
	return nil
}

// IsRunning checks if the Lima VM is running WITH Kubernetes enabled
func (m *LimaManager) IsRunning() (bool, error) {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

	cmd := exec.Command(m.limactlBin(), "list", "--json")
	cmd.Env = append(os.Environ(), "PATH="+newPath)
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	var entries []limaListEntry
	if err := json.Unmarshal(output, &entries); err != nil {
		return false, nil
	}

	for _, entry := range entries {
		if entry.Name == limaInstanceName && entry.Status == "Running" {
			// Check if Kubernetes is actually reachable via kubectl
			kubeconfig := m.KubeconfigPath()
			kubectlCmd := exec.Command(m.kubectlBin(), "--kubeconfig", kubeconfig, "cluster-info")
			if err := kubectlCmd.Run(); err != nil {
				slog.Debug("lima running but kubernetes not reachable", "error", err)
				return false, nil
			}
			return true, nil
		}
	}

	return false, nil
}

// Start starts the Lima VM with k3s (creates fresh VM)
// Caller should ensure no existing VM exists (use GetVMState and Recover if needed)
func (m *LimaManager) Start(ctx context.Context, config ClusterConfig) error {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

	// Create the Lima VM with k3s template
	// limactl create template://k3s --name=vibespace --cpus=X --memory=XGiB --disk=XGiB
	createArgs := []string{
		"create",
		"template:k3s",
		"--name=" + limaInstanceName,
		fmt.Sprintf("--cpus=%d", config.CPU),
		fmt.Sprintf("--memory=%d", config.Memory),
		fmt.Sprintf("--disk=%d", config.Disk),
		"--tty=false",
	}

	slog.Debug("executing limactl create", "args", createArgs)

	createCmd := exec.CommandContext(ctx, m.limactlBin(), createArgs...)
	createCmd.Env = append(os.Environ(), "PATH="+newPath)
	stdout, stderr := limaSubprocessWriters("limactl create")
	createCmd.Stdout = stdout
	createCmd.Stderr = stderr

	if err := createCmd.Run(); err != nil {
		slog.Error("limactl create failed", "error", err)
		return fmt.Errorf("failed to create Lima VM: %w", err)
	}

	// Start the VM
	// limactl start vibespace
	slog.Debug("executing limactl start")

	startCmd := exec.CommandContext(ctx, m.limactlBin(), "start", limaInstanceName)
	startCmd.Env = append(os.Environ(), "PATH="+newPath)
	startStdout, startStderr := limaSubprocessWriters("limactl start")
	startCmd.Stdout = startStdout
	startCmd.Stderr = startStderr

	if err := startCmd.Run(); err != nil {
		slog.Error("limactl start failed", "error", err)
		return fmt.Errorf("failed to start Lima VM: %w", err)
	}

	// Wait for and copy kubeconfig
	if err := m.waitAndCopyKubeconfig(ctx); err != nil {
		return fmt.Errorf("failed to copy kubeconfig: %w", err)
	}

	slog.Debug("lima start completed")
	return nil
}

// waitAndCopyKubeconfig waits for k3s to bootstrap and copies the kubeconfig
func (m *LimaManager) waitAndCopyKubeconfig(ctx context.Context) error {
	// Lima's k3s template copies kubeconfig to ~/.lima/vibespace/copied-from-guest/kubeconfig.yaml
	home, _ := os.UserHomeDir()
	sourceKubeconfig := filepath.Join(home, ".lima", limaInstanceName, "copied-from-guest", "kubeconfig.yaml")
	destKubeconfig := m.KubeconfigPath()

	slog.Debug("waiting for kubeconfig", "source", sourceKubeconfig)

	// Wait up to 120 seconds for kubeconfig to appear (k3s bootstrap takes time)
	for i := 0; i < 60; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if _, err := os.Stat(sourceKubeconfig); err == nil {
			// Kubeconfig exists, copy it
			data, err := os.ReadFile(sourceKubeconfig)
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

// Resume starts a stopped Lima VM without recreating it
func (m *LimaManager) Resume(ctx context.Context) error {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

	slog.Debug("executing limactl start (resume)")

	cmd := exec.CommandContext(ctx, m.limactlBin(), "start", limaInstanceName)
	cmd.Env = append(os.Environ(), "PATH="+newPath)
	stdout, stderr := limaSubprocessWriters("limactl start")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		slog.Error("limactl start (resume) failed", "error", err)
		return fmt.Errorf("failed to resume Lima VM: %w", err)
	}

	// Refresh kubeconfig in case it changed
	if err := m.waitAndCopyKubeconfig(ctx); err != nil {
		slog.Warn("failed to refresh kubeconfig after resume", "error", err)
		// Don't fail - kubeconfig might already be valid
	}

	slog.Debug("lima resume completed")
	return nil
}

// Recover cleans up a broken VM state so a fresh Start can succeed
func (m *LimaManager) Recover(ctx context.Context) error {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

	slog.Info("recovering from broken VM state")

	// Try to unlock Lima disk first (in case it's locked from a crash)
	limactl := m.limactlBin()
	if _, err := os.Stat(limactl); err == nil {
		slog.Debug("attempting disk unlock")
		unlockCmd := exec.CommandContext(ctx, limactl, "disk", "unlock", limaInstanceName)
		unlockCmd.Env = append(os.Environ(), "PATH="+newPath)
		_ = unlockCmd.Run() // Ignore errors - disk may not exist or be locked
	}

	// Force delete the Lima VM
	slog.Debug("executing limactl delete for recovery")
	deleteCmd := exec.CommandContext(ctx, m.limactlBin(), "delete", "--force", limaInstanceName)
	deleteCmd.Env = append(os.Environ(), "PATH="+newPath)
	output, err := deleteCmd.CombinedOutput()
	if err != nil {
		slog.Debug("limactl delete failed during recovery", "error", err, "output", string(output))
	}

	// Clean up Lima instance directory
	limaDir := m.limaInstanceDir()
	if _, err := os.Stat(limaDir); err == nil {
		slog.Debug("removing Lima instance directory", "path", limaDir)
		if err := os.RemoveAll(limaDir); err != nil {
			slog.Warn("failed to remove Lima instance directory", "path", limaDir, "error", err)
		}
	}

	// Remove stale kubeconfig (will be regenerated on start)
	kubeconfig := m.KubeconfigPath()
	if _, err := os.Stat(kubeconfig); err == nil {
		slog.Debug("removing stale kubeconfig", "path", kubeconfig)
		_ = os.Remove(kubeconfig)
	}

	slog.Info("recovery cleanup completed")
	return nil
}

// Stop stops the Lima VM gracefully, with force fallback on timeout
func (m *LimaManager) Stop(ctx context.Context) error {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

	slog.Debug("executing limactl stop")

	// Try graceful stop with 30s timeout
	stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(stopCtx, m.limactlBin(), "stop", limaInstanceName)
	cmd.Env = append(os.Environ(), "PATH="+newPath)
	stdout, stderr := limaSubprocessWriters("limactl stop")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		// Check if it was a timeout
		if stopCtx.Err() == context.DeadlineExceeded {
			slog.Warn("limactl stop timed out, trying force stop")
		} else {
			slog.Warn("limactl stop failed, trying force stop", "error", err)
		}

		// Force stop
		forceCmd := exec.CommandContext(ctx, m.limactlBin(), "stop", "--force", limaInstanceName)
		forceCmd.Env = append(os.Environ(), "PATH="+newPath)
		forceStdout, forceStderr := limaSubprocessWriters("limactl stop --force")
		forceCmd.Stdout = forceStdout
		forceCmd.Stderr = forceStderr

		if forceErr := forceCmd.Run(); forceErr != nil {
			slog.Error("limactl force stop also failed", "error", forceErr)
			return fmt.Errorf("failed to stop Lima VM: %w (force stop also failed: %v)", err, forceErr)
		}
		slog.Info("limactl force stop succeeded")
	}

	slog.Debug("limactl stop completed")
	return nil
}

// WaitReady waits for kubectl to be able to connect
func (m *LimaManager) WaitReady(ctx context.Context) error {
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

// Uninstall removes the vibespace Lima VM and all related data
func (m *LimaManager) Uninstall(ctx context.Context) error {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s:%s", m.qemuBinDir(), m.limaBinDir(), m.binDir, currentPath)

	slog.Info("uninstalling vibespace cluster")

	// Stop if running (ignore errors - VM might not be running)
	_ = m.Stop(ctx)

	// Try to unlock Lima disk first (in case it's locked from a crash)
	limactl := m.limactlBin()
	if _, err := os.Stat(limactl); err == nil {
		slog.Debug("attempting disk unlock")
		unlockCmd := exec.CommandContext(ctx, limactl, "disk", "unlock", limaInstanceName)
		unlockCmd.Env = append(os.Environ(), "PATH="+newPath)
		_ = unlockCmd.Run() // Ignore errors - disk may not exist or be locked
	}

	// Delete the Lima VM
	slog.Debug("executing limactl delete")
	deleteCmd := exec.CommandContext(ctx, m.limactlBin(), "delete", "--force", limaInstanceName)
	deleteCmd.Env = append(os.Environ(), "PATH="+newPath)
	output, err := deleteCmd.CombinedOutput()
	if err != nil {
		slog.Warn("limactl delete failed", "error", err, "output", string(output))
		// Continue with manual cleanup even if delete failed
	} else {
		slog.Debug("limactl delete succeeded", "output", string(output))
	}

	// Always do manual cleanup to ensure no leftovers
	// Clean up Lima instance directory (~/.lima/vibespace/)
	limaDir := m.limaInstanceDir()
	if _, err := os.Stat(limaDir); err == nil {
		slog.Debug("removing Lima instance directory", "path", limaDir)
		if err := os.RemoveAll(limaDir); err != nil {
			slog.Warn("failed to remove Lima instance directory", "path", limaDir, "error", err)
		}
	}

	// Verify deletion - check if VM still appears in limactl list
	checkCmd := exec.CommandContext(ctx, m.limactlBin(), "list", "--json")
	checkCmd.Env = append(os.Environ(), "PATH="+newPath)
	checkOutput, _ := checkCmd.Output()
	if len(checkOutput) > 0 && strings.Contains(string(checkOutput), limaInstanceName) {
		slog.Warn("VM still exists after delete attempt", "output", string(checkOutput))
	}

	slog.Info("vibespace cluster uninstalled")
	return nil
}

// KubeconfigPath returns the path to the isolated kubeconfig file
func (m *LimaManager) KubeconfigPath() string {
	return filepath.Join(m.vibespaceHome, "kubeconfig")
}

// limaSubprocessWriters returns writers for subprocess stdout/stderr
// By default, output is discarded to keep CLI output clean
// In debug mode (VIBESPACE_DEBUG=1), output is logged to debug log file
func limaSubprocessWriters(prefix string) (io.Writer, io.Writer) {
	if os.Getenv("VIBESPACE_DEBUG") == "" {
		// Discard subprocess output to keep CLI clean
		return io.Discard, io.Discard
	}

	// In debug mode, log output to slog (goes to debug log file, not terminal)
	stdoutWriter := &limaLogTeeWriter{w: io.Discard, prefix: prefix, stream: "stdout"}
	stderrWriter := &limaLogTeeWriter{w: io.Discard, prefix: prefix, stream: "stderr"}
	return stdoutWriter, stderrWriter
}

// limaLogTeeWriter writes to both an underlying writer and slog
type limaLogTeeWriter struct {
	w      io.Writer
	prefix string
	stream string
	buf    []byte
}

func (w *limaLogTeeWriter) Write(p []byte) (n int, err error) {
	// Write to underlying writer first
	n, err = w.w.Write(p)
	if err != nil {
		return n, err
	}

	// Buffer and log complete lines
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
