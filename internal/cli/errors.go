package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
	"github.com/yagizdagabak/vibespace/internal/platform"
	"github.com/yagizdagabak/vibespace/pkg/daemon"
	"github.com/yagizdagabak/vibespace/pkg/k8s"
	"github.com/yagizdagabak/vibespace/pkg/model"
	"github.com/yagizdagabak/vibespace/pkg/vibespace"
)


// checkClusterInitialized checks if the cluster has been initialized (kubeconfig exists)
func checkClusterInitialized() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Use isolated kubeconfig at ~/.vibespace/kubeconfig
	kubeconfig := filepath.Join(home, ".vibespace", "kubeconfig")
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return fmt.Errorf("run 'vibespace init' first: %w", vserrors.ErrClusterNotInitialized)
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
		return fmt.Errorf("run 'vibespace init' to start it: %w", vserrors.ErrClusterNotRunning)
	}

	running, err := manager.IsRunning()
	if err != nil || !running {
		return fmt.Errorf("run 'vibespace init' to start it: %w", vserrors.ErrClusterNotRunning)
	}

	return nil
}

// getVibespaceServiceWithCheck creates the vibespace service with prerequisite checks
func getVibespaceServiceWithCheck() (*vibespace.Service, error) {
	// Check cluster is running
	if err := checkClusterRunning(); err != nil {
		return nil, err
	}

	// Get isolated kubeconfig path
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	kubeconfig := filepath.Join(home, ".vibespace", "kubeconfig")

	// Set KUBECONFIG environment variable for the k8s client
	os.Setenv("KUBECONFIG", kubeconfig)

	// Create k8s client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		return nil, fmt.Errorf("check if cluster is running with 'vibespace status': %w", vserrors.ErrClusterUnreachable)
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

// ensureDaemonRunningForAgent ensures the daemon is running and returns the local port for an agent's forward.
// If the daemon has stale state (pod doesn't exist), it will rediscover agents.
func ensureDaemonRunningForAgent(ctx context.Context, vibespaceNameOrID string, agentName string, forwardType string) (int, error) {
	// Get vibespace service with checks
	svc, err := getVibespaceServiceWithCheck()
	if err != nil {
		return 0, err
	}

	// Verify vibespace exists and is running
	vs, err := checkVibespaceRunning(ctx, svc, vibespaceNameOrID)
	if err != nil {
		return 0, err
	}

	// Ensure daemon is running (auto-start if needed)
	if !daemon.IsDaemonRunning() {
		printStep("Starting daemon...")
		if err := daemon.SpawnDaemon(); err != nil {
			return 0, fmt.Errorf("failed to start daemon: %w", err)
		}
		// Wait for daemon to be ready and reconcile
		time.Sleep(2 * time.Second)
	}

	// Query daemon for the agent's forward
	client, err := daemon.NewClient()
	if err != nil {
		return 0, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	result, err := client.ListForwardsForVibespace(vs.Name)
	if err != nil {
		return 0, fmt.Errorf("failed to list forwards: %w", err)
	}

	// Find the agent's forward of the requested type
	for _, agent := range result.Agents {
		if agent.Name == agentName {
			for _, fwd := range agent.Forwards {
				if fwd.Type == forwardType {
					// If forward is active, use it
					if fwd.Status == "active" {
						return fwd.LocalPort, nil
					}
					// Forward not active - trigger refresh and wait for reconciliation
					printStep("Forward not active, triggering reconciliation...")
					if err := client.Refresh(); err != nil {
						return 0, fmt.Errorf("failed to refresh daemon: %w", err)
					}
					time.Sleep(2 * time.Second)
					// Re-check after refresh
					result, err = client.ListForwardsForVibespace(vs.Name)
					if err != nil {
						return 0, fmt.Errorf("failed to list forwards: %w", err)
					}
					for _, a := range result.Agents {
						if a.Name == agentName {
							for _, f := range a.Forwards {
								if f.Type == forwardType && f.Status == "active" {
									return f.LocalPort, nil
								}
							}
						}
					}
					return 0, fmt.Errorf("forward for %s is not ready after reconciliation", agentName)
				}
			}
			return 0, fmt.Errorf("agent '%s' has no %s forward", agentName, forwardType)
		}
	}

	// Agent not found in daemon state - trigger refresh
	printStep("Agent '%s' not found, refreshing daemon state...", agentName)
	if err := client.Refresh(); err != nil {
		return 0, fmt.Errorf("failed to refresh daemon: %w", err)
	}
	time.Sleep(2 * time.Second)

	// Try again after refresh
	result, err = client.ListForwardsForVibespace(vs.Name)
	if err != nil {
		return 0, fmt.Errorf("failed to list forwards: %w", err)
	}

	for _, agent := range result.Agents {
		if agent.Name == agentName {
			for _, fwd := range agent.Forwards {
				if fwd.Type == forwardType && fwd.Status == "active" {
					return fwd.LocalPort, nil
				}
			}
			return 0, fmt.Errorf("agent '%s' has no active %s forward", agentName, forwardType)
		}
	}

	return 0, fmt.Errorf("agent '%s' not found. Available: %s", agentName, formatAvailableAgents(result.Agents))
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

// ensureDaemonRunningSimple ensures the daemon is running (auto-starts if needed).
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
	if !daemon.IsDaemonRunning() {
		printStep("Starting daemon...")
		if err := daemon.SpawnDaemon(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
	}

	return nil
}

