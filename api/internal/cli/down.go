package cli

import (
	"fmt"

	"vibespace/pkg/daemon"
)

// runDown stops the port-forward daemon for a vibespace
// Usage: vibespace <name> down
func runDown(vibespace string, args []string) error {
	// Check if daemon is running
	if !daemon.IsRunning(vibespace) {
		printWarning("Daemon is not running for %s", vibespace)
		return nil
	}

	printStep("Stopping port-forward daemon for %s...", vibespace)

	if err := daemon.StopDaemon(vibespace); err != nil {
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	printSuccess("Daemon stopped")
	return nil
}
