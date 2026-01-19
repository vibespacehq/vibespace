package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"vibespace/pkg/daemon"
	"vibespace/pkg/k8s"
	"vibespace/pkg/portforward"

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

	// Setup logging to file
	logFile, err := setupDaemonLogging(daemonVibespace)
	if err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}

	slog.Info("daemon starting", "vibespace", daemonVibespace, "pid", os.Getpid())

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
	slog.Info("discovering pods", "vibespace", daemonVibespace)
	agents, err := discoverVibespaceAgents(ctx, k8sClient, daemonVibespace)
	if err != nil {
		slog.Warn("failed to discover agents", "error", err)
	} else {
		for agentName, podName := range agents {
			slog.Info("discovered agent", "agent", agentName, "pod", podName)
			manager.SetAgentPod(agentName, podName)
			state.SetAgentPod(agentName, podName)

			// Start default ttyd forward for each agent
			localPort, err := manager.AddForward(agentName, portforward.DefaultTTYDPort, portforward.TypeTTYD, 0)
			if err != nil {
				slog.Error("failed to add ttyd forward", "agent", agentName, "error", err)
			} else {
				state.AddForward(agentName, &daemon.ForwardState{
					LocalPort:  localPort,
					RemotePort: portforward.DefaultTTYDPort,
					Type:       portforward.TypeTTYD,
					Status:     portforward.StatusActive,
				})
				slog.Info("ttyd forward started", "agent", agentName, "local_port", localPort)
			}
		}
	}

	// Save initial state
	if err := state.Save(); err != nil {
		slog.Error("failed to save state", "error", err)
	}

	// Create and start server
	server, err := daemon.NewServer(daemon.ServerConfig{
		Vibespace: daemonVibespace,
		Manager:   manager,
		State:     state,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	slog.Info("daemon ready", "vibespace", daemonVibespace)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		slog.Info("received signal", "signal", sig)
	case <-ctx.Done():
		slog.Info("context cancelled")
	}

	// Cleanup
	slog.Info("shutting down daemon")
	manager.StopAll()
	server.Stop()
	daemon.CleanupDaemonFiles(daemonVibespace)

	slog.Info("daemon stopped", "vibespace", daemonVibespace)
	return nil
}

// setupDaemonLogging sets up logging for the daemon
func setupDaemonLogging(vibespace string) (*os.File, error) {
	paths, err := daemon.GetDaemonPaths(vibespace)
	if err != nil {
		return nil, err
	}

	logPath := paths.Dir + "/" + vibespace + ".log"
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	handler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))

	return logFile, nil
}

// discoverVibespaceAgents discovers all agents (pods) for a vibespace
func discoverVibespaceAgents(ctx context.Context, k8sClient *k8s.Client, vibespaceID string) (map[string]string, error) {
	// For now, assume single agent named "claude-1"
	// TODO: Support multi-agent discovery using pod labels

	pods, err := k8sClient.Clientset().CoreV1().Pods(k8s.VibespaceNamespace).List(ctx, listOptionsForVibespace(vibespaceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	agents := make(map[string]string)
	for i, pod := range pods.Items {
		// Name agents as claude-1, claude-2, etc.
		agentName := fmt.Sprintf("claude-%d", i+1)
		agents[agentName] = pod.Name
	}

	if len(agents) == 0 {
		return nil, fmt.Errorf("no pods found for vibespace %s", vibespaceID)
	}

	return agents, nil
}

// listOptionsForVibespace creates a list options with label selector for vibespace
func listOptionsForVibespace(vibespaceID string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("vibespace.dev/id=%s", vibespaceID),
	}
}
