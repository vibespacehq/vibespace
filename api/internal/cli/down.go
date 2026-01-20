package cli

import (
	"fmt"
	"log/slog"

	"vibespace/pkg/daemon"
)

// runDown stops the port-forward daemon for a vibespace
// Usage: vibespace <name> down
func runDown(vibespace string, args []string) error {
	slog.Info("down command started", "vibespace", vibespace)

	// Check if daemon is running
	if !daemon.IsRunning(vibespace) {
		slog.Debug("daemon not running", "vibespace", vibespace)
		printWarning("Daemon is not running for %s", vibespace)
		return nil
	}

	printStep("Stopping port-forward daemon for %s...", vibespace)

	if err := daemon.StopDaemon(vibespace); err != nil {
		slog.Error("failed to stop daemon", "vibespace", vibespace, "error", err)
		return fmt.Errorf("failed to stop daemon: %w", err)
	}

	slog.Info("down command completed", "vibespace", vibespace)
	printSuccess("Daemon stopped")
	return nil
}
