package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/k8s"
	"github.com/vibespacehq/vibespace/pkg/portforward"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Run the port-forward daemon (internal)",
	Hidden: true, // Hidden from help output
	RunE:   runDaemon,
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Setup logging with rotation (JSON format for daemon)
	cleanup := setupLogging(LogConfig{
		Mode: LogModeDaemon,
		Name: "daemon",
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
	if err := daemon.WritePidFile(os.Getpid()); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Create k8s client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	// Create desired state manager
	desiredMgr, err := daemon.NewDesiredStateManager()
	if err != nil {
		return fmt.Errorf("failed to create desired state manager: %w", err)
	}

	// Create daemon state
	state := daemon.NewDaemonState()

	// Create port-forward manager
	manager := portforward.NewManager(portforward.ManagerConfig{
		Vibespace:        "daemon",
		Config:           k8sClient.Config(),
		ReconnectEnabled: true,
		MaxReconnects:    10,
		OnStateChange: func(agentName string, remotePort int, status portforward.ForwardStatus, errMsg string) {
			// State is updated per-vibespace
			// This is a simplified callback - the reconciler handles state updates
			slog.Debug("forward state changed", "agent", agentName, "port", remotePort, "status", status)
		},
	})

	// Create pod watcher
	watcher := daemon.NewPodWatcher(k8sClient.Clientset())

	// Create reconciler
	reconciler := daemon.NewReconciler(daemon.ReconcilerConfig{
		DesiredMgr: desiredMgr,
		State:      state,
		Manager:    manager,
		Clientset:  k8sClient.Clientset(),
	})

	// Create and start server
	server, err := daemon.NewServer(daemon.ServerConfig{
		Watcher:    watcher,
		Reconciler: reconciler,
		State:      state,
		DesiredMgr: desiredMgr,
		Manager:    manager,
	})
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Initial reconciliation
	slog.Info("performing initial reconciliation")
	if err := reconciler.Reconcile(ctx); err != nil {
		slog.Warn("initial reconciliation failed", "error", err)
	}

	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Wire DNS server into reconciler now that it's started
	reconciler.SetDNSServer(server.DNSServer())

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
			cleanupDaemonFiles()
			close(shutdownDone)
		}()

		// Wait for graceful shutdown or second signal
		select {
		case <-shutdownDone:
			slog.Info("daemon stopped gracefully")
		case sig := <-sigChan:
			slog.Warn("received second signal, forcing shutdown", "signal", sig)
			fmt.Fprintln(os.Stderr, "Forcing shutdown...")
			cleanupDaemonFiles()
		case <-time.After(10 * time.Second):
			slog.Warn("graceful shutdown timeout, forcing")
			cleanupDaemonFiles()
		}
	case <-ctx.Done():
		slog.Info("context cancelled")
		// Cleanup
		slog.Info("shutting down daemon")
		manager.StopAll()
		server.Stop()
		cleanupDaemonFiles()
	}

	slog.Info("daemon stopped")
	return nil
}

// cleanupDaemonFiles removes daemon state files
func cleanupDaemonFiles() {
	paths, err := daemon.GetDaemonPaths()
	if err != nil {
		return
	}
	os.Remove(paths.PidFile)
	os.Remove(paths.SockFile)
}
