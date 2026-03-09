package daemon

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
)

// DaemonPaths contains paths for daemon state files
type DaemonPaths struct {
	PidFile  string // ~/.vibespace/daemon.pid
	SockFile string // ~/.vibespace/daemon.sock
	LogFile  string // ~/.vibespace/daemon.log
}

// GetDaemonPaths returns paths for the daemon
func GetDaemonPaths() (*DaemonPaths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	baseDir := filepath.Join(home, ".vibespace")
	return &DaemonPaths{
		PidFile:  filepath.Join(baseDir, "daemon.pid"),
		SockFile: filepath.Join(baseDir, "daemon.sock"),
		LogFile:  filepath.Join(baseDir, "daemon.log"),
	}, nil
}

// SpawnDaemon spawns the daemon process
func SpawnDaemon() error {
	// Ensure directories exist
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	baseDir := filepath.Join(home, ".vibespace")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create daemon directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "forwards"), 0755); err != nil {
		return fmt.Errorf("failed to create forwards directory: %w", err)
	}

	// Check if already running via socket
	if IsDaemonRunning() {
		return fmt.Errorf("%w", vserrors.ErrDaemonAlreadyRunning)
	}

	// Kill any orphaned daemon processes
	killDaemonProcesses()

	// Clean up stale files
	paths, _ := GetDaemonPaths()
	os.Remove(paths.PidFile)
	os.Remove(paths.SockFile)

	// Get the path to the current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build command: vibespace daemon
	cmd := exec.Command(executable, "daemon")

	// Detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Redirect stdout/stderr to /dev/null for daemon
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open /dev/null: %w", err)
	}
	defer devNull.Close()

	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.Stdin = nil

	// Start the daemon process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to be ready
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if IsDaemonRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return vserrors.ErrDaemonStartTimeout
}

// StopDaemon stops the daemon
func StopDaemon() error {
	client, err := NewClient()
	if err != nil {
		// Client creation failed, just kill processes
		killDaemonProcesses()
		cleanupDaemonFiles()
		return nil
	}

	// Try graceful shutdown first
	if err := client.Shutdown(); err == nil {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if !IsDaemonRunning() {
				killDaemonProcesses()
				cleanupDaemonFiles()
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Graceful shutdown failed, force kill
	killDaemonProcesses()
	cleanupDaemonFiles()
	return nil
}

// IsDaemonRunning checks if the daemon is running
func IsDaemonRunning() bool {
	paths, err := GetDaemonPaths()
	if err != nil {
		return false
	}

	if _, err := os.Stat(paths.SockFile); os.IsNotExist(err) {
		return false
	}

	client, err := NewClient()
	if err != nil {
		return false
	}

	return client.IsRunning()
}

// WaitForDaemonReady waits for the daemon to be ready
func WaitForDaemonReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsDaemonRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return vserrors.ErrDaemonStartTimeout
}

func GetDaemonStatus() (*DaemonStatusResponse, error) {
	if !IsDaemonRunning() {
		return nil, fmt.Errorf("%w", vserrors.ErrDaemonNotRunning)
	}

	client, err := NewClient()
	if err != nil {
		return nil, err
	}

	return client.DaemonStatus()
}

// killDaemonProcesses finds and kills daemon processes
func killDaemonProcesses() {
	// Pattern: "vibespace daemon"
	cmd := exec.Command("pgrep", "-f", "vibespace daemon$")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return
	}

	pids := strings.Fields(out.String())
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil {
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		process.Signal(syscall.SIGTERM)
	}

	if len(pids) > 0 {
		time.Sleep(500 * time.Millisecond)
	}

	// SIGKILL remaining
	cmd = exec.Command("pgrep", "-f", "vibespace daemon$")
	out.Reset()
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return
	}
	pids = strings.Fields(out.String())
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(strings.TrimSpace(pidStr))
		if err != nil {
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		process.Signal(syscall.SIGKILL)
	}
}

// cleanupDaemonFiles removes daemon state files
func cleanupDaemonFiles() {
	paths, err := GetDaemonPaths()
	if err != nil {
		return
	}
	os.Remove(paths.PidFile)
	os.Remove(paths.SockFile)
}

func WritePidFile(pid int) error {
	paths, err := GetDaemonPaths()
	if err != nil {
		return err
	}
	return os.WriteFile(paths.PidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}
