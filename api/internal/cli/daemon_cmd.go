package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vibespace/pkg/daemon"
	"vibespace/pkg/k8s"
	"vibespace/pkg/portforward"
	"vibespace/pkg/vibespace"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Run the port-forward daemon (internal)",
	Hidden: true, // Hidden from help output
	RunE:   runDaemon,
}

var daemonVibespace string

func init() {
	daemonCmd.Flags().StringVar(&daemonVibespace, "vibespace", "", "Vibespace name")
	daemonCmd.MarkFlagRequired("vibespace")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	if daemonVibespace == "" {
		return fmt.Errorf("--vibespace is required")
	}

	// Setup logging with rotation (JSON format for daemon)
	cleanup := setupLogging(LogConfig{
		Mode: LogModeDaemon,
		Name: daemonVibespace,
	})
	defer cleanup()

	slog.Info("daemon starting", "pid", os.Getpid())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ensure daemon directory exists
	if err := daemon.EnsureDaemonDir(); err != nil {
		return fmt.Errorf("failed to create daemon directory: %w", err)
	}

	// Write PID file
	if err := daemon.WritePidFile(daemonVibespace, os.Getpid()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Create k8s client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Resolve vibespace name to internal ID
	svc := vibespace.NewService(k8sClient)
	vs, err := svc.Get(ctx, daemonVibespace)
	if err != nil {
		return fmt.Errorf("failed to get vibespace: %w", err)
	}
	vibespaceID := vs.ID
	slog.Info("resolved vibespace", "name", daemonVibespace, "id", vibespaceID)

	// Create daemon state
	state, err := daemon.NewDaemonState(daemonVibespace)
	if err != nil {
		return fmt.Errorf("failed to create state: %w", err)
	}

	// Create port-forward manager
	manager := portforward.NewManager(portforward.ManagerConfig{
		Vibespace:        daemonVibespace,
		Config:           k8sClient.Config(),
		ReconnectEnabled: true,
		MaxReconnects:    10,
		OnStateChange: func(agentName string, remotePort int, status portforward.ForwardStatus, errMsg string) {
			state.UpdateForwardStatus(agentName, remotePort, status, errMsg)
			state.Save()
		},
	})

	// Discover pods for this vibespace
	slog.Info("discovering pods", "id", vibespaceID)
	agents, err := discoverVibespaceAgents(ctx, k8sClient, vibespaceID)
	if err != nil {
		slog.Warn("failed to discover agents", "error", err)
	} else {
		for agentName, podName := range agents {
			slog.Info("discovered agent", "agent", agentName, "pod", podName)
			manager.SetAgentPod(agentName, podName)
			state.SetAgentPod(agentName, podName)

			// Start SSH forward (primary - for CLI terminal access)
			sshLocalPort, err := manager.AddForward(agentName, portforward.DefaultSSHPort, portforward.TypeSSH, 0)
			if err != nil {
				slog.Error("failed to add SSH forward", "agent", agentName, "error", err)
			} else {
				state.AddForward(agentName, &daemon.ForwardState{
					LocalPort:  sshLocalPort,
					RemotePort: portforward.DefaultSSHPort,
					Type:       portforward.TypeSSH,
					Status:     portforward.StatusActive,
				})
				slog.Info("SSH forward started", "agent", agentName, "local_port", sshLocalPort)
			}

			// Start ttyd forward (fallback - for browser terminal access)
			ttydLocalPort, err := manager.AddForward(agentName, portforward.DefaultTTYDPort, portforward.TypeTTYD, 0)
			if err != nil {
				slog.Error("failed to add ttyd forward", "agent", agentName, "error", err)
			} else {
				state.AddForward(agentName, &daemon.ForwardState{
					LocalPort:  ttydLocalPort,
					RemotePort: portforward.DefaultTTYDPort,
					Type:       portforward.TypeTTYD,
					Status:     portforward.StatusActive,
				})
				slog.Info("ttyd forward started", "agent", agentName, "local_port", ttydLocalPort)
			}
		}
	}

	// Save initial state
	if err := state.Save(); err != nil {
		slog.Error("failed to save state", "error", err)
	}

	// Create refresh callback that re-discovers pods
	refreshCallback := func() (map[string]string, error) {
		return discoverVibespaceAgents(ctx, k8sClient, vibespaceID)
	}

	// Create and start server
	server, err := daemon.NewServer(daemon.ServerConfig{
		Vibespace: daemonVibespace,
		Manager:   manager,
		State:     state,
		OnRefresh: refreshCallback,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	slog.Info("daemon ready")

	// Wait for shutdown signal with double Ctrl-C support
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		slog.Info("received signal, starting graceful shutdown", "signal", sig)
		fmt.Fprintln(os.Stderr, "Shutting down gracefully... (press Ctrl-C again to force)")

		// Start graceful shutdown with timeout
		shutdownDone := make(chan struct{})
		go func() {
			// Cleanup
			slog.Info("shutting down daemon")
			manager.StopAll()
			server.Stop()
			daemon.CleanupDaemonFiles(daemonVibespace)
			close(shutdownDone)
		}()

		// Wait for graceful shutdown or second signal
		select {
		case <-shutdownDone:
			slog.Info("daemon stopped gracefully")
		case sig := <-sigChan:
			slog.Warn("received second signal, forcing shutdown", "signal", sig)
			fmt.Fprintln(os.Stderr, "Forcing shutdown...")
			daemon.CleanupDaemonFiles(daemonVibespace)
		case <-time.After(10 * time.Second):
			slog.Warn("graceful shutdown timeout, forcing")
			daemon.CleanupDaemonFiles(daemonVibespace)
		}
	case <-ctx.Done():
		slog.Info("context cancelled")
		// Cleanup
		slog.Info("shutting down daemon")
		manager.StopAll()
		server.Stop()
		daemon.CleanupDaemonFiles(daemonVibespace)
	}

	slog.Info("daemon stopped")
	return nil
}

// discoverVibespaceAgents discovers all agents (pods) for a vibespace
// Uses the vibespace.dev/claude-id label to identify agents
// Scales up any deployments with replicas=0
func discoverVibespaceAgents(ctx context.Context, k8sClient *k8s.Client, vibespaceID string) (map[string]string, error) {
	// First, scale up any stopped deployments
	expectedCount, err := scaleUpDeployments(ctx, k8sClient, vibespaceID)
	if err != nil {
		slog.Warn("failed to scale up deployments", "error", err)
	}
	if expectedCount == 0 {
		expectedCount = 1 // At least expect one agent
	}
	slog.Info("expecting agents", "count", expectedCount)

	// Wait a bit for pods to start
	time.Sleep(2 * time.Second)

	// Poll for pods to be ready (up to 30 seconds)
	var agents map[string]string
	var lastErr error
	for i := 0; i < 15; i++ {
		pods, err := k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).List(ctx, listOptionsForVibespace(vibespaceID))
		if err != nil {
			lastErr = fmt.Errorf("failed to list pods: %w", err)
			time.Sleep(2 * time.Second)
			continue
		}

		agents = make(map[string]string)
		for _, pod := range pods.Items {
			// Skip pods that aren't running
			if pod.Status.Phase != "Running" {
				continue
			}

			// Try to get claude-id from pod labels
			claudeID := "1" // Default for backward compatibility
			if labels := pod.Labels; labels != nil {
				if cid, ok := labels["vibespace.dev/claude-id"]; ok && cid != "" {
					claudeID = cid
				}
			}

			agentName := fmt.Sprintf("claude-%s", claudeID)

			// Skip if we already have this agent (shouldn't happen, but be defensive)
			if _, exists := agents[agentName]; exists {
				slog.Warn("duplicate agent discovered", "agent", agentName, "pod", pod.Name)
				continue
			}

			agents[agentName] = pod.Name
		}

		// Wait until we have all expected agents (or at least some if we've waited long enough)
		if len(agents) >= expectedCount {
			slog.Info("all expected agents found", "count", len(agents))
			return agents, nil
		}

		// After 10 attempts (20 seconds), return what we have if we found any
		if i >= 10 && len(agents) > 0 {
			slog.Info("returning partial agents after timeout", "found", len(agents), "expected", expectedCount)
			return agents, nil
		}

		slog.Info("waiting for pods to be ready", "attempt", i+1, "found", len(agents), "expected", expectedCount)
		time.Sleep(2 * time.Second)
	}

	// Return whatever we found, even if incomplete
	if len(agents) > 0 {
		slog.Info("returning agents after max attempts", "found", len(agents), "expected", expectedCount)
		return agents, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no running pods found for vibespace %s after waiting", vibespaceID)
}

// scaleUpDeployments scales up any deployments with replicas=0
// Returns the total number of deployments (expected agent count)
func scaleUpDeployments(ctx context.Context, k8sClient *k8s.Client, vibespaceID string) (int, error) {
	deployments, err := k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vibespaceID),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list deployments: %w", err)
	}

	// Scale up any stopped deployments
	scaledUp := 0
	for _, deploy := range deployments.Items {
		if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 0 {
			slog.Info("scaling up deployment", "name", deploy.Name)
			one := int32(1)
			deploy.Spec.Replicas = &one
			_, err := k8sClient.Clientset().AppsV1().Deployments(k8s.VibespaceNamespace).Update(ctx, &deploy, metav1.UpdateOptions{})
			if err != nil {
				slog.Warn("failed to scale deployment", "name", deploy.Name, "error", err)
			} else {
				scaledUp++
			}
		}
	}

	if scaledUp > 0 {
		slog.Info("scaled up deployments", "count", scaledUp, "total", len(deployments.Items))
	}
	return len(deployments.Items), nil
}

// listOptionsForVibespace creates a list options with label selector for vibespace
func listOptionsForVibespace(vibespaceID string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vibespaceID),
	}
}
