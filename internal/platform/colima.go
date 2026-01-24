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

// colimaProfile is the dedicated Colima profile name for vibespace
// Using a dedicated profile avoids conflicts with user's default Colima setup
const colimaProfile = "vibespace"

// VMState represents the state of the Colima VM
type VMState int

const (
	VMStateNotExists VMState = iota // VM doesn't exist
	VMStateStopped                  // VM exists but is stopped
	VMStateRunning                  // VM is running
	VMStateBroken                   // VM is in a broken/inconsistent state
)

// ColimaManager manages the Colima-based Kubernetes cluster on macOS
type ColimaManager struct {
	platform      Platform
	vibespaceHome string
	binDir        string
}

// Compile-time interface satisfaction check
var _ ClusterManager = (*ColimaManager)(nil)

// NewColimaManager creates a new Colima cluster manager
func NewColimaManager(p Platform, vibespaceHome string) *ColimaManager {
	return &ColimaManager{
		platform:      p,
		vibespaceHome: vibespaceHome,
		binDir:        filepath.Join(vibespaceHome, "bin"),
	}
}

// colimaProfileDir returns the path to the Colima profile config directory
func (m *ColimaManager) colimaProfileDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".colima", colimaProfile)
}

// Binary paths
func (m *ColimaManager) colimaBin() string {
	return filepath.Join(m.binDir, "colima")
}

func (m *ColimaManager) kubectlBin() string {
	return filepath.Join(m.binDir, "kubectl")
}

// limactlBin returns the path to the limactl binary
func (m *ColimaManager) limactlBin() string {
	return filepath.Join(m.limaBinDir(), "limactl")
}

// limaInstanceDir returns the path to the Lima instance directory for vibespace
func (m *ColimaManager) limaInstanceDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".colima", "_lima", "colima-"+colimaProfile)
}

// limaDisksDir returns the path to the Lima disks directory for vibespace
func (m *ColimaManager) limaDisksDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".colima", "_lima", "_disks", "colima-"+colimaProfile)
}

// colimaListEntry represents a single entry in colima list --json output
type colimaListEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// GetVMState detects the current state of the Colima VM
func (m *ColimaManager) GetVMState(ctx context.Context) VMState {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	// Use a timeout context for state detection
	detectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Run colima list --json to check VM status
	cmd := exec.CommandContext(detectCtx, m.colimaBin(), "--profile", colimaProfile, "list", "--json")
	cmd.Env = append(os.Environ(),
		"PATH="+newPath,
		"KUBECONFIG="+m.KubeconfigPath(),
	)
	output, err := cmd.Output()
	if err != nil {
		// If colima list fails or times out, check for leftover Lima files
		if _, statErr := os.Stat(m.limaInstanceDir()); statErr == nil {
			slog.Debug("colima list failed but Lima instance exists, likely broken", "error", err)
			return VMStateBroken
		}
		slog.Debug("colima list failed, VM likely doesn't exist", "error", err)
		return VMStateNotExists
	}

	// Parse the JSON output - can be a single object or an array depending on colima version
	var entries []colimaListEntry
	if err := json.Unmarshal(output, &entries); err != nil {
		// Try parsing as single object (colima returns object when only one VM exists)
		var single colimaListEntry
		if err2 := json.Unmarshal(output, &single); err2 != nil {
			slog.Debug("failed to parse colima list output", "error", err, "output", string(output))
			return VMStateBroken
		}
		entries = []colimaListEntry{single}
	}

	// Find our profile in the list
	for _, entry := range entries {
		if entry.Name == colimaProfile {
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

	// Profile not in list - check for orphaned Lima files
	if _, err := os.Stat(m.limaInstanceDir()); err == nil {
		slog.Debug("VM not in colima list but Lima instance exists, broken state")
		return VMStateBroken
	}

	return VMStateNotExists
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

// Install downloads Colima, Lima, and kubectl
func (m *ColimaManager) Install(ctx context.Context) error {
	// Ensure bin directory exists
	if err := os.MkdirAll(m.binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	// Download Lima first (Colima depends on it)
	if err := m.downloadLima(ctx); err != nil {
		return fmt.Errorf("failed to download Lima: %w", err)
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

func (m *ColimaManager) downloadLima(ctx context.Context) error {
	// Lima releases: https://github.com/lima-vm/lima/releases
	// Asset naming: lima-X.Y.Z-Darwin-arm64.tar.gz, lima-X.Y.Z-Darwin-x86_64.tar.gz
	arch := m.platform.Arch
	if arch == "amd64" {
		arch = "x86_64"
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
	assetName := fmt.Sprintf("lima-%s-Darwin-%s.tar.gz", version, arch)

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
	// This matches the Tauri app structure where Lima lives in its own subdirectory
	limaDir := filepath.Join(m.vibespaceHome, "lima")
	if err := os.MkdirAll(limaDir, 0755); err != nil {
		return fmt.Errorf("failed to create lima directory: %w", err)
	}

	if err := downloadAndExtractTarGz(ctx, assetURL, limaDir); err != nil {
		return fmt.Errorf("failed to download and extract Lima: %w", err)
	}

	return nil
}

// limaBinDir returns the path to Lima's bin directory
func (m *ColimaManager) limaBinDir() string {
	return filepath.Join(m.vibespaceHome, "lima", "bin")
}

func (m *ColimaManager) downloadColima(ctx context.Context) error {
	// Colima releases: https://github.com/abiosoft/colima/releases
	arch := m.platform.Arch
	if arch == "amd64" {
		arch = "x86_64"
	}
	// arm64 stays as arm64 for macOS

	assetName := fmt.Sprintf("colima-Darwin-%s", arch)
	url, err := getGitHubReleaseAssetURL(ctx, "abiosoft", "colima", assetName)
	if err != nil {
		return fmt.Errorf("failed to get Colima download URL: %w", err)
	}

	return downloadBinary(ctx, url, m.colimaBin())
}

func (m *ColimaManager) downloadKubectl(ctx context.Context) error {
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
		"https://dl.k8s.io/release/%s/bin/darwin/%s/kubectl",
		version, arch,
	)

	return downloadBinary(ctx, url, m.kubectlBin())
}

// IsRunning checks if the Colima VM is running WITH Kubernetes enabled
func (m *ColimaManager) IsRunning() (bool, error) {
	// Build PATH with lima/bin and bin directories
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	cmd := exec.Command(m.colimaBin(), "--profile", colimaProfile, "status")
	cmd.Env = append(os.Environ(),
		"PATH="+newPath,
		"KUBECONFIG="+m.KubeconfigPath(),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// "colima status" returns non-zero if not running
		return false, nil
	}

	outputStr := string(output)
	// Check if Colima is running
	if !strings.Contains(outputStr, "is running") {
		return false, nil
	}

	// Check if Kubernetes is actually reachable via kubectl
	// Note: colima status doesn't always report "docker+k3s" reliably
	kubeconfig := m.KubeconfigPath()
	kubectlCmd := exec.Command(m.kubectlBin(), "--kubeconfig", kubeconfig, "cluster-info")
	if err := kubectlCmd.Run(); err != nil {
		slog.Debug("colima running but kubernetes not reachable", "error", err)
		return false, nil
	}

	return true, nil
}

// Start starts the Colima VM with k3s (creates fresh VM)
// Caller should ensure no existing VM exists (use GetVMState and Recover if needed)
func (m *ColimaManager) Start(ctx context.Context, config ClusterConfig) error {
	// Build PATH with lima/bin and bin directories
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	// Use bash -c to run colima with PATH and KUBECONFIG set
	// This ensures the environment is properly inherited by Colima's subprocesses (like limactl)
	// Using dedicated "vibespace" profile to avoid conflicts with user's default Colima setup
	commandStr := fmt.Sprintf(
		"PATH='%s' KUBECONFIG='%s' '%s' --profile %s start --kubernetes --cpu %d --memory %d --disk %d",
		newPath,
		m.KubeconfigPath(),
		m.colimaBin(),
		colimaProfile,
		config.CPU,
		config.Memory,
		config.Disk,
	)

	slog.Debug("executing colima start", "command", commandStr)

	cmd := exec.CommandContext(ctx, "bash", "-c", commandStr)
	stdout, stderr := subprocessWriters("colima start")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		slog.Error("colima start failed", "error", err)
		return fmt.Errorf("failed to start Colima: %w", err)
	}

	slog.Debug("colima start completed")
	return nil
}

// Resume starts a stopped Colima VM without recreating it
func (m *ColimaManager) Resume(ctx context.Context) error {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	// Use bash -c to run colima with PATH and KUBECONFIG set
	commandStr := fmt.Sprintf(
		"PATH='%s' KUBECONFIG='%s' '%s' --profile %s start",
		newPath,
		m.KubeconfigPath(),
		m.colimaBin(),
		colimaProfile,
	)

	slog.Debug("executing colima resume (start stopped VM)", "command", commandStr)

	cmd := exec.CommandContext(ctx, "bash", "-c", commandStr)
	stdout, stderr := subprocessWriters("colima resume")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		slog.Error("colima resume failed", "error", err)
		return fmt.Errorf("failed to resume Colima: %w", err)
	}

	slog.Debug("colima resume completed")
	return nil
}

// Recover cleans up a broken VM state so a fresh Start can succeed
func (m *ColimaManager) Recover(ctx context.Context) error {
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	slog.Info("recovering from broken VM state")

	// Try to unlock Lima disk first (in case it's locked from a crash)
	limactl := m.limactlBin()
	if _, err := os.Stat(limactl); err == nil {
		slog.Debug("attempting disk unlock")
		unlockCmd := exec.CommandContext(ctx, limactl, "disk", "unlock", "colima-"+colimaProfile)
		unlockCmd.Env = append(os.Environ(), "PATH="+newPath)
		_ = unlockCmd.Run() // Ignore errors - disk may not exist or be locked
	}

	// Force delete the Colima VM using bash -c for proper PATH inheritance
	commandStr := fmt.Sprintf(
		"PATH='%s' '%s' --profile %s delete --force",
		newPath,
		m.colimaBin(),
		colimaProfile,
	)
	slog.Debug("executing colima delete for recovery", "command", commandStr)

	cmd := exec.CommandContext(ctx, "bash", "-c", commandStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Debug("colima delete failed during recovery", "error", err, "output", string(output))
	}

	// Clean up Lima instance directory
	limaDir := m.limaInstanceDir()
	if _, err := os.Stat(limaDir); err == nil {
		slog.Debug("removing Lima instance directory", "path", limaDir)
		if err := os.RemoveAll(limaDir); err != nil {
			slog.Warn("failed to remove Lima instance directory", "path", limaDir, "error", err)
		}
	}

	// Clean up Lima disks directory (persistent disk)
	disksDir := m.limaDisksDir()
	if _, err := os.Stat(disksDir); err == nil {
		slog.Debug("removing Lima disks directory", "path", disksDir)
		if err := os.RemoveAll(disksDir); err != nil {
			slog.Warn("failed to remove Lima disks directory", "path", disksDir, "error", err)
		}
	}

	// Clean up profile config directory
	profileDir := m.colimaProfileDir()
	if _, err := os.Stat(profileDir); err == nil {
		slog.Debug("removing Colima profile directory", "path", profileDir)
		if err := os.RemoveAll(profileDir); err != nil {
			slog.Warn("failed to remove profile directory", "path", profileDir, "error", err)
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

// Stop stops the Colima VM gracefully, with force fallback on timeout
func (m *ColimaManager) Stop(ctx context.Context) error {
	// Build PATH with lima/bin and bin directories
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	slog.Debug("executing colima stop")

	// Try graceful stop with 30s timeout
	stopCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(stopCtx, m.colimaBin(), "--profile", colimaProfile, "stop")
	cmd.Env = append(os.Environ(),
		"PATH="+newPath,
		"KUBECONFIG="+m.KubeconfigPath(),
	)
	stdout, stderr := subprocessWriters("colima stop")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		// Check if it was a timeout
		if stopCtx.Err() == context.DeadlineExceeded {
			slog.Warn("colima stop timed out, trying force stop")
		} else {
			slog.Warn("colima stop failed, trying force stop", "error", err)
		}

		// Force stop
		forceCmd := exec.CommandContext(ctx, m.colimaBin(), "--profile", colimaProfile, "stop", "--force")
		forceCmd.Env = append(os.Environ(),
			"PATH="+newPath,
			"KUBECONFIG="+m.KubeconfigPath(),
		)
		forceStdout, forceStderr := subprocessWriters("colima stop --force")
		forceCmd.Stdout = forceStdout
		forceCmd.Stderr = forceStderr

		if forceErr := forceCmd.Run(); forceErr != nil {
			slog.Error("colima force stop also failed", "error", forceErr)
			return fmt.Errorf("failed to stop Colima: %w (force stop also failed: %v)", err, forceErr)
		}
		slog.Info("colima force stop succeeded")
	}

	slog.Debug("colima stop completed")
	return nil
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

// Uninstall removes the vibespace Colima VM and all related data
// Note: Does NOT remove ~/.vibespace/ directory (caller handles that)
// Note: Does NOT touch ~/.kube/config (we use isolated kubeconfig)
func (m *ColimaManager) Uninstall(ctx context.Context) error {
	// Build PATH with lima/bin and bin directories
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	slog.Info("uninstalling vibespace cluster")

	// Stop if running (ignore errors - VM might not be running)
	_ = m.Stop(ctx)

	// Try to unlock Lima disk first (in case it's locked from a crash)
	limactl := m.limactlBin()
	if _, err := os.Stat(limactl); err == nil {
		slog.Debug("attempting disk unlock")
		unlockCmd := exec.CommandContext(ctx, limactl, "disk", "unlock", "colima-"+colimaProfile)
		unlockCmd.Env = append(os.Environ(), "PATH="+newPath)
		_ = unlockCmd.Run() // Ignore errors - disk may not exist or be locked
	}

	// Delete the Colima VM using bash -c for proper PATH inheritance
	// This is critical - without proper PATH, colima can't find limactl
	commandStr := fmt.Sprintf(
		"PATH='%s' '%s' --profile %s delete --force",
		newPath,
		m.colimaBin(),
		colimaProfile,
	)
	slog.Debug("executing colima delete", "command", commandStr)

	cmd := exec.CommandContext(ctx, "bash", "-c", commandStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("colima delete failed", "error", err, "output", string(output))
		// Continue with manual cleanup even if delete failed
	} else {
		slog.Debug("colima delete succeeded", "output", string(output))
	}

	// Always do manual cleanup to ensure no leftovers
	// Clean up Lima instance directory (~/.colima/_lima/colima-vibespace/)
	limaDir := m.limaInstanceDir()
	if _, err := os.Stat(limaDir); err == nil {
		slog.Debug("removing Lima instance directory", "path", limaDir)
		if err := os.RemoveAll(limaDir); err != nil {
			slog.Warn("failed to remove Lima instance directory", "path", limaDir, "error", err)
		}
	}

	// Clean up Lima disks directory (~/.colima/_lima/_disks/colima-vibespace/)
	// This is critical - colima delete doesn't remove the persistent disk!
	disksDir := m.limaDisksDir()
	if _, err := os.Stat(disksDir); err == nil {
		slog.Debug("removing Lima disks directory", "path", disksDir)
		if err := os.RemoveAll(disksDir); err != nil {
			slog.Warn("failed to remove Lima disks directory", "path", disksDir, "error", err)
		}
	}

	// Clean up profile config directory (~/.colima/vibespace/)
	profileDir := m.colimaProfileDir()
	if _, err := os.Stat(profileDir); err == nil {
		slog.Debug("removing Colima profile directory", "path", profileDir)
		if err := os.RemoveAll(profileDir); err != nil {
			slog.Warn("failed to remove profile directory", "path", profileDir, "error", err)
		}
	}

	// Verify deletion - check if VM still appears in colima list
	checkCmd := fmt.Sprintf("PATH='%s' '%s' --profile %s list --json 2>/dev/null", newPath, m.colimaBin(), colimaProfile)
	checkOutput, _ := exec.CommandContext(ctx, "bash", "-c", checkCmd).Output()
	if len(checkOutput) > 0 && strings.Contains(string(checkOutput), colimaProfile) {
		slog.Warn("VM still exists after delete attempt", "output", string(checkOutput))
	}

	slog.Info("vibespace cluster uninstalled")
	return nil
}

// KubeconfigPath returns the path to the isolated kubeconfig file
// Using ~/.vibespace/kubeconfig to avoid touching user's ~/.kube/config
func (m *ColimaManager) KubeconfigPath() string {
	return filepath.Join(m.vibespaceHome, "kubeconfig")
}

// subprocessWriters returns writers for subprocess stdout/stderr
// By default, output is discarded to keep CLI output clean
// In debug mode (VIBESPACE_DEBUG=1), output is logged to debug log file
func subprocessWriters(prefix string) (io.Writer, io.Writer) {
	if os.Getenv("VIBESPACE_DEBUG") == "" {
		// Discard subprocess output to keep CLI clean
		return io.Discard, io.Discard
	}

	// In debug mode, log output to slog (goes to debug log file, not terminal)
	stdoutWriter := &logTeeWriter{w: io.Discard, prefix: prefix, stream: "stdout"}
	stderrWriter := &logTeeWriter{w: io.Discard, prefix: prefix, stream: "stderr"}
	return stdoutWriter, stderrWriter
}

// logTeeWriter writes to both an underlying writer and slog
type logTeeWriter struct {
	w      io.Writer
	prefix string
	stream string
	buf    []byte
}

func (w *logTeeWriter) Write(p []byte) (n int, err error) {
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

