package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibespace/internal/platform"
	"vibespace/pkg/daemon"
	"vibespace/pkg/k8s"
	"vibespace/pkg/model"
	"vibespace/pkg/vibespace"
)

// Common error messages
const (
	errClusterNotInitialized = "cluster not initialized. Run 'vibespace init' first"
	errClusterNotRunning     = "cluster is not running. Run 'vibespace init' to start it"
	errClusterUnreachable    = "cannot connect to cluster. Check if cluster is running with 'vibespace status'"
)

// checkClusterInitialized checks if the cluster has been initialized (kubeconfig exists)
func checkClusterInitialized() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	kubeconfig := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return fmt.Errorf(errClusterNotInitialized)
	}

	return nil
}

// checkClusterRunning checks if the cluster is actually running and reachable
func checkClusterRunning() error {
	// First check if initialized
	if err := checkClusterInitialized(); err != nil {
		return err
	}

	// Check if cluster manager reports running
	home, _ := os.UserHomeDir()
	vibespaceHome := filepath.Join(home, ".vibespace")

	p := platform.Detect()
	manager, err := platform.NewClusterManager(p, vibespaceHome)
	if err != nil {
		return fmt.Errorf(errClusterNotRunning)
	}

	running, err := manager.IsRunning()
	if err != nil || !running {
		return fmt.Errorf(errClusterNotRunning)
	}

	return nil
}

// getVibespaceServiceWithCheck creates the vibespace service with prerequisite checks
func getVibespaceServiceWithCheck() (*vibespace.Service, error) {
	// Check cluster is running
	if err := checkClusterRunning(); err != nil {
		return nil, err
	}

	// Get kubeconfig path
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	kubeconfig := filepath.Join(home, ".kube", "config")

	// Set KUBECONFIG environment variable for the k8s client
	os.Setenv("KUBECONFIG", kubeconfig)

	// Create k8s client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errClusterUnreachable, err)
	}

	// Create vibespace service
	svc := vibespace.NewService(k8sClient)
	return svc, nil
}

// checkVibespaceExists checks if a vibespace exists and returns it
func checkVibespaceExists(ctx context.Context, svc *vibespace.Service, name string) (*model.Vibespace, error) {
	vs, err := svc.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("vibespace '%s' not found. List available: vibespace list", name)
	}
	return vs, nil
}

// checkVibespaceRunning checks if a vibespace exists and is running
func checkVibespaceRunning(ctx context.Context, svc *vibespace.Service, name string) (*model.Vibespace, error) {
	vs, err := checkVibespaceExists(ctx, svc, name)
	if err != nil {
		return nil, err
	}

	switch vs.Status {
	case "running":
		return vs, nil
	case "creating":
		return nil, fmt.Errorf("vibespace '%s' is still creating. Wait for it to be ready or check: vibespace list", name)
	case "stopped":
		return nil, fmt.Errorf("vibespace '%s' is stopped. Start it with: vibespace %s start", name, name)
	case "error":
		return nil, fmt.Errorf("vibespace '%s' is in error state. Check logs or delete and recreate", name)
	default:
		return nil, fmt.Errorf("vibespace '%s' is %s. Start it with: vibespace %s start", name, vs.Status, name)
	}
}

// ensureDaemonRunning ensures the daemon is running for a vibespace and returns the local port for an agent.
// It will auto-start the daemon if it's not running.
// Returns the local port for the agent's ttyd/gotty forward (port 7681).
func ensureDaemonRunning(ctx context.Context, vibespaceNameOrID string, agentName string) (int, error) {
	// Get vibespace service with checks
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return 0, err
	}

	// Verify vibespace exists and is running
	_, err = checkVibespaceRunning(ctx, svc, vibespaceNameOrID)
	if err != nil {
		return 0, err
	}

	// Ensure daemon is running (auto-start if needed)
	if !daemon.IsRunning(vibespaceNameOrID) {
		printStep("Starting port-forward daemon...")
		if err := daemon.SpawnDaemon(vibespaceNameOrID); err != nil {
			return 0, fmt.Errorf("failed to start daemon: %w", err)
		}
	}

	// Query daemon for the agent's ttyd local port
	client, err := daemon.NewClient(vibespaceNameOrID)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	result, err := client.ListForwards()
	if err != nil {
		return 0, fmt.Errorf("failed to list forwards: %w", err)
	}

	// Find the agent's ttyd forward
	for _, agent := range result.Agents {
		if agent.Name == agentName {
			for _, fwd := range agent.Forwards {
				if fwd.Type == "ttyd" {
					// Auto-start stopped forwards
					if fwd.Status != "active" {
						printStep("Restarting ttyd forward...")
						if err := client.RestartForward(agentName, fwd.RemotePort); err != nil {
							return 0, fmt.Errorf("failed to restart forward: %w", err)
						}
						// Give it a moment to start
						time.Sleep(500 * time.Millisecond)
					}
					return fwd.LocalPort, nil
				}
			}
			return 0, fmt.Errorf("agent '%s' has no ttyd forward", agentName)
		}
	}

	return 0, fmt.Errorf("agent '%s' not found. Available agents: %s", agentName, formatAvailableAgents(result.Agents))
}

// formatAvailableAgents formats a list of agent names for error messages
func formatAvailableAgents(agents []daemon.AgentStatus) string {
	if len(agents) == 0 {
		return "(none)"
	}
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	return strings.Join(names, ", ")
}

// ensureDaemonRunningSimple ensures the daemon is running for a vibespace (auto-starts if needed).
// This is a simpler version that doesn't return the local port.
func ensureDaemonRunningSimple(ctx context.Context, vibespaceNameOrID string) error {
	// Get vibespace service with checks
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return err
	}

	// Verify vibespace exists and is running
	_, err = checkVibespaceRunning(ctx, svc, vibespaceNameOrID)
	if err != nil {
		return err
	}

	// Ensure daemon is running (auto-start if needed)
	if !daemon.IsRunning(vibespaceNameOrID) {
		printStep("Starting port-forward daemon...")
		if err := daemon.SpawnDaemon(vibespaceNameOrID); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
	}

	return nil
}
