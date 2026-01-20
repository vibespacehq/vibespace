package cli

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"vibespace/pkg/daemon"
)

// runUp starts the port-forward daemon for a vibespace
// Usage: vibespace <name> up
func runUp(vibespace string, args []string) error {
	ctx := context.Background()

	slog.Info("up command started", "vibespace", vibespace)

	// Check cluster is running first
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		slog.Error("failed to get vibespace service", "error", err)
		return err
	}

	// Check vibespace exists and is running
	_, err = checkVibespaceRunning(ctx, svc, vibespace)
	if err != nil {
		slog.Error("vibespace not running", "vibespace", vibespace, "error", err)
		return err
	}

	// Check if daemon is already running
	if daemon.IsRunning(vibespace) {
		status, err := daemon.GetStatus(vibespace)
		if err == nil {
			slog.Debug("daemon already running", "vibespace", vibespace)
			printSuccess("Daemon already running")
			printDaemonStatus(status)
			return nil
		}
	}

	// Start the daemon
	printStep("Starting port-forward daemon for %s...", vibespace)

	if err := daemon.SpawnDaemon(vibespace); err != nil {
		paths, _ := daemon.GetDaemonPaths(vibespace)
		logPath := filepath.Join(paths.Dir, vibespace+".log")
		slog.Error("failed to spawn daemon", "vibespace", vibespace, "error", err, "log_path", logPath)
		return fmt.Errorf("failed to start daemon: %w\nCheck logs: %s", err, logPath)
	}

	// Wait for daemon to be ready and get status
	if err := daemon.WaitForReady(vibespace, 10e9); err != nil {
		paths, _ := daemon.GetDaemonPaths(vibespace)
		logPath := filepath.Join(paths.Dir, vibespace+".log")
		slog.Error("daemon failed to become ready", "vibespace", vibespace, "error", err, "log_path", logPath)
		return fmt.Errorf("daemon failed to become ready: %w\nCheck logs: %s\nTry: vibespace %s down && vibespace %s up", err, logPath, vibespace, vibespace)
	}

	status, err := daemon.GetStatus(vibespace)
	if err != nil {
		slog.Info("up command completed", "vibespace", vibespace)
		printSuccess("Daemon started")
		return nil
	}

	slog.Info("up command completed", "vibespace", vibespace, "active_ports", status.ActivePorts, "total_ports", status.TotalPorts)
	printSuccess("Daemon started")
	printDaemonStatus(status)

	return nil
}

// printDaemonStatus prints the daemon status
func printDaemonStatus(status *daemon.StatusResponse) {
	fmt.Println()
	fmt.Printf("Vibespace: %s\n", status.Vibespace)
	fmt.Printf("Uptime: %s\n", status.Uptime)
	fmt.Printf("Active ports: %d/%d\n", status.ActivePorts, status.TotalPorts)

	if len(status.Agents) > 0 {
		fmt.Println()
		fmt.Println("Agents:")
		for _, agent := range status.Agents {
			fmt.Printf("  %s (%s)\n", agent.Name, agent.PodName)
			for _, fwd := range agent.Forwards {
				statusIcon := green("●")
				if fwd.Status != "active" {
					statusIcon = yellow("○")
				}
				fmt.Printf("    %s localhost:%d → %d (%s)\n", statusIcon, fwd.LocalPort, fwd.RemotePort, fwd.Type)
			}
		}
	}

	fmt.Println()
	fmt.Println("Commands:")
	fmt.Printf("  vibespace %s forward list     List all port-forwards\n", status.Vibespace)
	fmt.Printf("  vibespace %s forward add PORT Add a port-forward\n", status.Vibespace)
	fmt.Printf("  vibespace %s down             Stop the daemon\n", status.Vibespace)
}
