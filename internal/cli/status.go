package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/yagizdagabak/vibespace/internal/platform"
	"github.com/yagizdagabak/vibespace/pkg/daemon"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status",
	Long:  `Display the current status of the vibespace cluster and its components.`,
	Example: `  vibespace status
  vibespace status --json`,
	RunE: runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	out := getOutput()
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	vibespaceHome := filepath.Join(home, ".vibespace")

	// Detect platform
	p := platform.Detect()

	// Get cluster manager
	manager, err := platform.NewClusterManager(p, vibespaceHome)
	if err != nil {
		return fmt.Errorf("failed to create cluster manager: %w", err)
	}

	// Check installation status
	installed, err := manager.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation: %w", err)
	}

	// JSON output mode - gather all data first
	if out.IsJSONMode() {
		statusOut := StatusOutput{
			Cluster: ClusterStatus{
				Installed: installed,
				Running:   false,
				Platform:  p.OS,
			},
		}

		if installed {
			running, _ := manager.IsRunning()
			statusOut.Cluster.Running = running

			if running {
				ctx := context.Background()
				components := checkClusterComponents(ctx, vibespaceHome)
				for name, ready := range components {
					statusOut.Components = append(statusOut.Components, ComponentStatus{
						Name:  name,
						Ready: ready,
					})
				}
			}
		}

		// Add daemon status
		if daemon.IsDaemonRunning() {
			daemonStatus, err := daemon.GetDaemonStatus()
			if err == nil {
				statusOut.Daemon = &DaemonStatus{
					Running:    true,
					Pid:        daemonStatus.Pid,
					Uptime:     daemonStatus.Uptime,
					Vibespaces: make(map[string]DaemonVibespace),
				}
				for name, vs := range daemonStatus.Vibespaces {
					statusOut.Daemon.Vibespaces[name] = DaemonVibespace{
						AgentCount: len(vs.Agents),
					}
				}
			}
		} else {
			statusOut.Daemon = &DaemonStatus{Running: false}
		}

		return out.JSON(JSONOutput{
			Success: true,
			Data:    statusOut,
		})
	}

	if !installed {
		fmt.Println("Cluster: not installed")
		fmt.Println()
		fmt.Println("Run 'vibespace init' to set up the cluster")
		return nil
	}

	// Check running status
	running, err := manager.IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check cluster status: %w", err)
	}

	if running {
		fmt.Printf("Cluster: %s\n", green("running"))
	} else {
		fmt.Printf("Cluster: %s\n", yellow("stopped"))
		fmt.Println()
		fmt.Println("Run 'vibespace init' to start the cluster")
		return nil
	}

	// Check cluster components
	ctx := context.Background()
	components := checkClusterComponents(ctx, vibespaceHome)

	fmt.Println()
	fmt.Println("Components:")
	for name, status := range components {
		if status {
			fmt.Printf("  %s: %s\n", name, green("ready"))
		} else {
			fmt.Printf("  %s: %s\n", name, yellow("not ready"))
		}
	}

	// Check daemon status
	fmt.Println()
	if daemon.IsDaemonRunning() {
		daemonStatus, err := daemon.GetDaemonStatus()
		if err != nil {
			fmt.Printf("Daemon: %s (%v)\n", yellow("error"), err)
		} else {
			fmt.Printf("Daemon: %s (uptime: %s, pid: %d)\n", green("running"), daemonStatus.Uptime, daemonStatus.Pid)
			if len(daemonStatus.Vibespaces) > 0 {
				fmt.Println("  Managed vibespaces:")
				for name, vs := range daemonStatus.Vibespaces {
					agentCount := len(vs.Agents)
					fmt.Printf("    %s: %d agent(s)\n", name, agentCount)
				}
			}
		}
	} else {
		fmt.Println("Daemon: not running")
	}

	return nil
}

func checkClusterComponents(ctx context.Context, vibespaceHome string) map[string]bool {
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubectlBin := filepath.Join(vibespaceHome, "bin", "kubectl")

	result := map[string]bool{
		"Namespace": false,
	}

	// Check if vibespace namespace exists
	cmd := exec.CommandContext(ctx, kubectlBin, "--kubeconfig", kubeconfig,
		"get", "namespace", "vibespace", "-o", "name")
	if err := cmd.Run(); err == nil {
		result["Namespace"] = true
	}

	return result
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the cluster",
	Long:  `Stop the vibespace cluster. Data is preserved and can be started again with 'vibespace init'.`,
	Example: `  vibespace stop`,
	RunE: runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	slog.Info("stop command started")

	// Stop daemon first
	if daemon.IsDaemonRunning() {
		printStep("Stopping daemon...")
		if err := daemon.StopDaemon(); err != nil {
			slog.Warn("failed to stop daemon", "error", err)
		} else {
			printSuccess("Daemon stopped")
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to get home directory", "error", err)
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	vibespaceHome := filepath.Join(home, ".vibespace")

	// Detect platform
	p := platform.Detect()
	slog.Debug("platform detected", "os", p.OS, "arch", p.Arch)

	// Get cluster manager
	manager, err := platform.NewClusterManager(p, vibespaceHome)
	if err != nil {
		slog.Error("failed to create cluster manager", "error", err)
		return fmt.Errorf("failed to create cluster manager: %w", err)
	}

	// Check if running
	running, err := manager.IsRunning()
	if err != nil {
		slog.Error("failed to check cluster status", "error", err)
		return fmt.Errorf("failed to check cluster status: %w", err)
	}
	slog.Debug("cluster status check", "running", running)

	if !running {
		slog.Debug("cluster already stopped")
		fmt.Println("Cluster is already stopped")
		return nil
	}

	spinner := NewSpinner("Stopping cluster...")
	spinner.Start()
	slog.Info("stopping cluster")
	ctx := context.Background()
	if err := manager.Stop(ctx); err != nil {
		spinner.Fail("Failed to stop cluster")
		slog.Error("failed to stop cluster", "error", err)
		return fmt.Errorf("failed to stop cluster: %w", err)
	}

	slog.Info("stop completed successfully")
	spinner.Success("Cluster stopped")
	return nil
}
