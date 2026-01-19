package cli

import (
	"context"
	"fmt"
	"path/filepath"

	"vibespace/pkg/daemon"
)

// runUp starts the port-forward daemon for a vibespace
// Usage: vibespace <name> up
func runUp(vibespace string, args []string) error {
	ctx := context.Background()

	// Check cluster is running first
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	// Check vibespace exists and is running
	_, err = checkVibespaceRunning(ctx, svc, vibespace)
	if err != nil {
		return err
	}

	// Check if daemon is already running
	if daemon.IsRunning(vibespace) {
		status, err := daemon.GetStatus(vibespace)
		if err == nil {
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
		return fmt.Errorf("failed to start daemon: %w\nCheck logs: %s", err, logPath)
	}

	// Wait for daemon to be ready and get status
	if err := daemon.WaitForReady(vibespace, 10e9); err != nil {
		paths, _ := daemon.GetDaemonPaths(vibespace)
		logPath := filepath.Join(paths.Dir, vibespace+".log")
		return fmt.Errorf("daemon failed to become ready: %w\nCheck logs: %s\nTry: vibespace %s down && vibespace %s up", err, logPath, vibespace, vibespace)
	}

	status, err := daemon.GetStatus(vibespace)
	if err != nil {
		printSuccess("Daemon started")
		return nil
	}

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
