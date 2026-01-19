package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// SpawnDaemon spawns a new daemon process for a vibespace
// It forks the current process with the "daemon" subcommand
func SpawnDaemon(vibespace string) error {
	// Ensure daemon directory exists
	if err := EnsureDaemonDir(); err != nil {
		return err
	}

	// Check if already running
	if IsRunning(vibespace) {
		return fmt.Errorf("daemon already running for %s", vibespace)
	}

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
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if client.IsRunning() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("daemon failed to start within timeout")
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
		return nil, fmt.Errorf("daemon not running for %s", vibespace)
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

	return fmt.Errorf("daemon not ready within timeout")
}

// EnsureRunning ensures a daemon is running, starting it if necessary
func EnsureRunning(vibespace string) error {
	if IsRunning(vibespace) {
		return nil
	}

	return SpawnDaemon(vibespace)
}
