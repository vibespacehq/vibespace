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

// ensureDaemonRunningSSHAnyAgent ensures the daemon is running and returns the local SSH port.
// If agentName is empty, it uses the first available agent (all agents share the same PVC).
func ensureDaemonRunningSSHAnyAgent(ctx context.Context, vibespaceNameOrID string, agentName string) (int, error) {
	if agentName != "" {
		return ensureDaemonRunningForType(ctx, vibespaceNameOrID, agentName, "ssh")
	}

	// No agent specified - find first available
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

	// Ensure daemon is running
	if !daemon.IsRunning(vibespaceNameOrID) {
		printStep("Starting port-forward daemon...")
		if err := daemon.SpawnDaemon(vibespaceNameOrID); err != nil {
			return 0, fmt.Errorf("failed to start daemon: %w", err)
		}
	}

	// Query daemon for available agents
	client, err := daemon.NewClient(vibespaceNameOrID)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	result, err := client.ListForwards()
	if err != nil {
		return 0, fmt.Errorf("failed to list forwards: %w", err)
	}

	// Find first agent with active SSH forward
	for _, agent := range result.Agents {
		for _, fwd := range agent.Forwards {
			if fwd.Type == "ssh" && fwd.Status == "active" {
				return fwd.LocalPort, nil
			}
		}
	}

	// No active SSH forward found - try to restart first available agent's SSH
	for _, agent := range result.Agents {
		for _, fwd := range agent.Forwards {
			if fwd.Type == "ssh" {
				printStep("Restarting ssh forward for %s...", agent.Name)
				if err := client.RestartForward(agent.Name, fwd.RemotePort); err != nil {
					// Try refresh
					printStep("Refreshing pods...")
					if refreshErr := client.Refresh(); refreshErr != nil {
						continue // Try next agent
					}
					time.Sleep(2 * time.Second)
					if err := client.RestartForward(agent.Name, fwd.RemotePort); err != nil {
						continue // Try next agent
					}
				}
				time.Sleep(500 * time.Millisecond)
				return fwd.LocalPort, nil
			}
		}
	}

	return 0, fmt.Errorf("no agents available. Create a vibespace first")
}

// ensureDaemonRunningTTYD ensures the daemon is running and returns the local ttyd port for an agent.
// It will auto-start the daemon if it's not running.
// Returns the local port for the agent's ttyd forward (port 7681).
func ensureDaemonRunningTTYD(ctx context.Context, vibespaceNameOrID string, agentName string) (int, error) {
	return ensureDaemonRunningForType(ctx, vibespaceNameOrID, agentName, "ttyd")
}

// ensureDaemonRunningForType ensures the daemon is running and returns the local port for an agent's forward of the given type.
func ensureDaemonRunningForType(ctx context.Context, vibespaceNameOrID string, agentName string, forwardType string) (int, error) {
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

	// Query daemon for the agent's forward
	client, err := daemon.NewClient(vibespaceNameOrID)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	result, err := client.ListForwards()
	if err != nil {
		return 0, fmt.Errorf("failed to list forwards: %w", err)
	}

	// Find the agent's forward of the requested type
	for _, agent := range result.Agents {
		if agent.Name == agentName {
			for _, fwd := range agent.Forwards {
				if fwd.Type == forwardType {
					// Auto-start stopped forwards
					if fwd.Status != "active" {
						printStep("Restarting %s forward...", forwardType)
						if err := client.RestartForward(agentName, fwd.RemotePort); err != nil {
							// Restart failed - pod may have been scaled down
							// Try refreshing pods and restarting
							printStep("Refreshing pods (deployment may be scaled down)...")
							if refreshErr := client.Refresh(); refreshErr != nil {
								return 0, fmt.Errorf("failed to refresh: %w (original error: %v)", refreshErr, err)
							}
							// Give pods time to start
							time.Sleep(2 * time.Second)
							// Try restart again
							if err := client.RestartForward(agentName, fwd.RemotePort); err != nil {
								return 0, fmt.Errorf("failed to restart forward after refresh: %w", err)
							}
						}
						// Give it a moment to start
						time.Sleep(500 * time.Millisecond)
					}
					return fwd.LocalPort, nil
				}
			}
			return 0, fmt.Errorf("agent '%s' has no %s forward", agentName, forwardType)
		}
	}

	// Agent not found - try refreshing pods (deployment may be scaled down)
	printStep("Agent '%s' not found, refreshing pods...", agentName)
	if err := client.Refresh(); err != nil {
		return 0, fmt.Errorf("agent '%s' not found and refresh failed: %w", agentName, err)
	}

	// Wait for pod to be ready
	time.Sleep(3 * time.Second)

	// Try again
	result, err = client.ListForwards()
	if err != nil {
		return 0, fmt.Errorf("failed to list forwards after refresh: %w", err)
	}

	for _, agent := range result.Agents {
		if agent.Name == agentName {
			for _, fwd := range agent.Forwards {
				if fwd.Type == forwardType {
					if fwd.Status != "active" {
						printStep("Starting %s forward...", forwardType)
						if err := client.RestartForward(agentName, fwd.RemotePort); err != nil {
							return 0, fmt.Errorf("failed to start forward: %w", err)
						}
						time.Sleep(500 * time.Millisecond)
					}
					return fwd.LocalPort, nil
				}
			}
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
