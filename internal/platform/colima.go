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

// IsRunning checks if the Colima VM is running
func (m *ColimaManager) IsRunning() (bool, error) {
	// Build PATH with lima/bin and bin directories
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	cmd := exec.Command(m.colimaBin(), "status")
	cmd.Env = append(os.Environ(), "PATH="+newPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// "colima status" returns non-zero if not running
		return false, nil
	}
	// Colima outputs to stderr, check for "colima is running"
	return strings.Contains(string(output), "is running"), nil
}

// Start starts the Colima VM with k3s
func (m *ColimaManager) Start(ctx context.Context, config ClusterConfig) error {
	// Build PATH with lima/bin and bin directories
	// This matches the Tauri app approach
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	// Use bash -c to run colima with PATH set
	// This ensures the environment is properly inherited by Colima's subprocesses (like limactl)
	// Note: We use the default profile (no --profile flag) and Colima updates ~/.kube/config automatically
	commandStr := fmt.Sprintf(
		"PATH='%s' '%s' start --kubernetes --cpu %d --memory %d --disk %d",
		newPath,
		m.colimaBin(),
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

// Stop stops the Colima VM
func (m *ColimaManager) Stop(ctx context.Context) error {
	// Build PATH with lima/bin and bin directories
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	slog.Debug("executing colima stop")

	cmd := exec.CommandContext(ctx, m.colimaBin(), "stop")
	cmd.Env = append(os.Environ(), "PATH="+newPath)
	stdout, stderr := subprocessWriters("colima stop")
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		slog.Error("colima stop failed", "error", err)
		return err
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

// Uninstall removes Colima and all data
func (m *ColimaManager) Uninstall(ctx context.Context) error {
	// Build PATH with lima/bin and bin directories
	currentPath := os.Getenv("PATH")
	newPath := fmt.Sprintf("%s:%s:%s", m.limaBinDir(), m.binDir, currentPath)

	// Stop if running
	_ = m.Stop(ctx)

	// Delete the Colima VM (default profile)
	cmd := exec.CommandContext(ctx, m.colimaBin(), "delete", "--force")
	cmd.Env = append(os.Environ(), "PATH="+newPath)
	_ = cmd.Run()

	// Remove binaries
	_ = os.Remove(m.colimaBin())
	_ = os.Remove(m.kubectlBin())
	// Note: We don't remove ~/.kube/config as it may contain other contexts

	return nil
}

// KubeconfigPath returns the path to the kubeconfig file
// Colima automatically updates ~/.kube/config with the "colima" context
func (m *ColimaManager) KubeconfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
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

