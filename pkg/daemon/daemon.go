package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
)

// SpawnDaemon spawns a new daemon process for a vibespace
// It forks the current process with the "daemon" subcommand
func SpawnDaemon(vibespace string) error {
	// Ensure daemon directory exists
	if err := EnsureDaemonDir(); err != nil {
		return err
	}

	// Check if already running via socket
	if IsRunning(vibespace) {
		return fmt.Errorf("%s: %w", vibespace, vserrors.ErrDaemonAlreadyRunning)
	}

	// Kill any stale daemon process before spawning new one
	// This handles the case where socket is dead but process still exists
	if pid, err := ReadPidFile(vibespace); err == nil {
		if process, err := os.FindProcess(pid); err == nil {
			// Check if process is actually running
			if err := process.Signal(syscall.Signal(0)); err == nil {
				// Process exists, kill it
				process.Signal(syscall.SIGTERM)
				time.Sleep(500 * time.Millisecond)
				process.Signal(syscall.SIGKILL)
			}
		}
	}
	// Clean up stale files
	CleanupDaemonFiles(vibespace)

	// Get the path to the current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build command: vibespace daemon --vibespace=<name>
	cmd := exec.Command(executable, "daemon", "--vibespace="+vibespace)

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
	client, err := NewClient(vibespace)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Poll until daemon is ready or timeout
	// Daemon may take a while to start if it needs to wake up scaled-down pods
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if client.IsRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return vserrors.ErrDaemonStartTimeout
}

// StopDaemon stops a running daemon
func StopDaemon(vibespace string) error {
	client, err := NewClient(vibespace)
	if err != nil {
		return err
	}

	// Try graceful shutdown first
	if err := client.Shutdown(); err == nil {
		// Wait for daemon to stop
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if !client.IsRunning() {
				// Cleanup state files
				CleanupDaemonFiles(vibespace)
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Graceful shutdown failed, try killing the process
	pid, err := ReadPidFile(vibespace)
	if err != nil {
		// No PID file, just cleanup
		CleanupDaemonFiles(vibespace)
		return nil
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		CleanupDaemonFiles(vibespace)
		return nil
	}

	// Check if process is still running
	if err := process.Signal(syscall.Signal(0)); err != nil {
		// Process not running
		CleanupDaemonFiles(vibespace)
		return nil
	}

	// Send SIGTERM
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Try SIGKILL
		process.Signal(syscall.SIGKILL)
	}

	// Wait for process to exit
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := process.Signal(syscall.Signal(0)); err != nil {
			// Process exited
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Cleanup state files
	CleanupDaemonFiles(vibespace)
	return nil
}

// IsRunning checks if a daemon is running for a vibespace
func IsRunning(vibespace string) bool {
	// First check if socket exists
	paths, err := GetDaemonPaths(vibespace)
	if err != nil {
		return false
	}

	if _, err := os.Stat(paths.SockFile); os.IsNotExist(err) {
		return false
	}

	// Try to connect
	client, err := NewClient(vibespace)
	if err != nil {
		return false
	}

	return client.IsRunning()
}

// GetStatus gets the status of a daemon
func GetStatus(vibespace string) (*StatusResponse, error) {
	if !IsRunning(vibespace) {
		return nil, fmt.Errorf("%s: %w", vibespace, vserrors.ErrDaemonNotRunning)
	}

	client, err := NewClient(vibespace)
	if err != nil {
		return nil, err
	}

	return client.Status()
}

// WaitForReady waits for a daemon to be ready
func WaitForReady(vibespace string, timeout time.Duration) error {
	client, err := NewClient(vibespace)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if client.IsRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return vserrors.ErrDaemonStartTimeout
}

// StopAllDaemons stops all running daemons and cleans up state files
func StopAllDaemons() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	daemonDir := fmt.Sprintf("%s/.vibespace/daemons", home)

	// Find all .pid files in the daemon directory
	entries, err := os.ReadDir(daemonDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		// Look for .pid files to identify vibespaces with daemons
		if len(name) > 4 && name[len(name)-4:] == ".pid" {
			vibespace := name[:len(name)-4]
			StopDaemon(vibespace)
		}
	}

	// Also clean up any orphaned state files
	for _, entry := range entries {
		name := entry.Name()
		if len(name) > 5 && (name[len(name)-5:] == ".sock" || name[len(name)-5:] == ".json") {
			os.Remove(fmt.Sprintf("%s/%s", daemonDir, name))
		}
		if len(name) > 4 && name[len(name)-4:] == ".log" {
			// Keep log files for debugging
		}
	}
}
