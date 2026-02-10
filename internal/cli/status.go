package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/vibespacehq/vibespace/internal/platform"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/remote"
	"github.com/vibespacehq/vibespace/pkg/ui"

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

	// Check remote connection status
	remoteState, _ := remote.LoadRemoteState()
	remoteConnected := remoteState != nil && remoteState.Connected

	// Check local cluster status
	p := platform.Detect()
	var localInstalled, localRunning bool
	manager, err := platform.NewClusterManager(p, vibespaceHome, platform.ClusterManagerOptions{})
	if err == nil {
		localInstalled, _ = manager.IsInstalled()
		if localInstalled {
			localRunning, _ = manager.IsRunning()
		}
	}

	// JSON output mode
	if out.IsJSONMode() {
		statusOut := StatusOutput{
			Cluster: ClusterStatus{
				Installed: localInstalled || remoteConnected,
				Running:   localRunning || remoteConnected,
				Platform:  p.OS,
			},
		}

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

		if remoteConnected {
			statusOut.Remote = &RemoteStatusOutput{
				Connected:   true,
				ServerHost:  remoteState.ServerHost,
				LocalIP:     remoteState.LocalIP,
				ServerIP:    remoteState.ServerIP,
				ConnectedAt: remoteState.ConnectedAt.Format("2006-01-02 15:04:05"),
			}
		}

		return out.JSON(NewJSONOutput(true, statusOut, nil))
	}

	// No cluster and no remote
	if !localInstalled && !remoteConnected {
		fmt.Printf("Cluster: %s\n", out.Orange("not installed"))
		fmt.Println()
		fmt.Printf("Run %s to set up the cluster\n", out.Teal("vibespace init"))
		return nil
	}

	// Local cluster exists but not running, and no remote
	if localInstalled && !localRunning && !remoteConnected {
		fmt.Printf("Cluster: %s\n", out.Yellow("stopped"))
		fmt.Println()
		fmt.Printf("Run %s to start the cluster\n", out.Teal("vibespace init"))
		return nil
	}

	// Styles
	tealStyle := lipgloss.NewStyle().Foreground(ui.Teal)
	pinkStyle := lipgloss.NewStyle().Foreground(ui.Pink)
	orangeStyle := lipgloss.NewStyle().Foreground(ui.Orange)
	boldStyle := lipgloss.NewStyle().Bold(true)

	// Determine cluster status label
	var clusterLabel string
	if remoteConnected && !localRunning {
		clusterLabel = fmt.Sprintf("%s %s", boldStyle.Render("Cluster"), tealStyle.Render("remote"))
	} else if localRunning && remoteConnected {
		clusterLabel = fmt.Sprintf("%s %s", boldStyle.Render("Cluster"), tealStyle.Render("running + remote"))
	} else {
		clusterLabel = fmt.Sprintf("%s %s", boldStyle.Render("Cluster"), tealStyle.Render("running"))
	}

	l := list.New(clusterLabel)

	// Remote connection section (show first when it's the primary cluster)
	if remoteConnected {
		l.Item(fmt.Sprintf("%s %s", boldStyle.Render("Remote"), tealStyle.Render("connected")))
		l.Item(list.New(
			fmt.Sprintf("server %s", pinkStyle.Render(remoteState.ServerHost)),
			fmt.Sprintf("local IP %s", pinkStyle.Render(remoteState.LocalIP)),
		))
	}

	// Daemon section
	if daemon.IsDaemonRunning() {
		daemonStatus, err := daemon.GetDaemonStatus()
		if err != nil {
			l.Item(fmt.Sprintf("%s %s", boldStyle.Render("Daemon"), out.Yellow("error")))
			l.Item(list.New(out.Dim(err.Error())))
		} else {
			l.Item(fmt.Sprintf("%s %s", boldStyle.Render("Daemon"), tealStyle.Render("running")))
			l.Item(list.New(
				fmt.Sprintf("uptime %s", pinkStyle.Render(daemonStatus.Uptime)),
				fmt.Sprintf("pid %s", pinkStyle.Render(fmt.Sprintf("%d", daemonStatus.Pid))),
			))

			if len(daemonStatus.Vibespaces) > 0 {
				var vsItems []any
				for name, vs := range daemonStatus.Vibespaces {
					agentCount := len(vs.Agents)
					suffix := ""
					if agentCount != 1 {
						suffix = "s"
					}
					vsItems = append(vsItems, fmt.Sprintf("%s %s", orangeStyle.Render(name), out.Dim(fmt.Sprintf("(%d agent%s)", agentCount, suffix))))
				}
				l.Item(fmt.Sprintf("%s %s", boldStyle.Render("Vibespaces"), out.Dim(fmt.Sprintf("(%d)", len(daemonStatus.Vibespaces)))))
				l.Item(list.New(vsItems...))
			}
		}
	} else if !remoteConnected {
		l.Item(fmt.Sprintf("%s %s", boldStyle.Render("Daemon"), out.Dim("not running")))
	}

	fmt.Println(l)
	return nil
}

var stopCmd = &cobra.Command{
	Use:     "stop",
	Short:   "Stop the cluster",
	Long:    `Stop the vibespace cluster. Data is preserved and can be started again with 'vibespace init'.`,
	Example: `  vibespace stop`,
	RunE:    runClusterStop,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove vibespace and all cluster data",
	Long: `Completely remove vibespace including:
- The Colima VM (macOS), Lima VM (Linux), or bare metal k3s (Linux --bare-metal)
- All vibespace data in ~/.vibespace/
- All vibespaces and their data

This action cannot be undone. Your ~/.kube/config is NOT affected.`,
	Example: `  vibespace uninstall
  vibespace uninstall --force`,
	RunE: runUninstall,
}

var uninstallForce bool

func init() {
	uninstallCmd.Flags().BoolVarP(&uninstallForce, "force", "f", false, "Skip confirmation prompt")
}

func runClusterStop(cmd *cobra.Command, args []string) error {
	slog.Info("stop command started")
	out := getOutput()

	// Check if in remote mode with no local cluster
	if isRemoteConnected() {
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(false, nil, &JSONError{Message: "connected to remote server — use 'vibespace remote disconnect' instead"}))
		}
		fmt.Printf("Connected to remote server. Use %s to disconnect.\n", out.Teal("vibespace remote disconnect"))
		return nil
	}

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
	manager, err := platform.NewClusterManager(p, vibespaceHome, platform.ClusterManagerOptions{})
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
		if out.IsJSONMode() {
			return out.JSON(NewJSONOutput(true, StopOutput{
				Stopped: true,
				Target:  "cluster",
			}, nil))
		}
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

	// JSON output
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, StopOutput{
			Stopped: true,
			Target:  "cluster",
		}, nil))
	}

	return nil
}

func runUninstall(cmd *cobra.Command, args []string) error {
	slog.Info("uninstall command started")

	if !uninstallForce {
		fmt.Println("This will remove ALL vibespace data including:")
		fmt.Println("  - The Colima VM (macOS) or k3s cluster (Linux)")
		fmt.Println("  - All vibespaces and their data")
		fmt.Println("  - All downloaded binaries")
		fmt.Println("  - WireGuard interfaces and configuration")
		fmt.Println()
		fmt.Println("Your ~/.kube/config will NOT be affected.")
		fmt.Println()
		fmt.Print("Continue? [y/N] ")

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Aborted.")
			return nil
		}
	}

	// Stop daemon first
	if daemon.IsDaemonRunning() {
		printStep("Stopping daemon...")
		if err := daemon.StopDaemon(); err != nil {
			slog.Warn("failed to stop daemon", "error", err)
		} else {
			printSuccess("Daemon stopped")
		}
	}

	// Kill serve daemon process if running
	if remote.IsServeRunning() {
		printStep("Stopping serve process...")
		if err := remote.KillServeProcess(); err != nil {
			slog.Warn("failed to stop serve process", "error", err)
		} else {
			printSuccess("Serve process stopped")
		}
	}

	// Tear down WireGuard tunnel and remote connections
	if remote.IsInterfaceUp() {
		printStep("Stopping WireGuard tunnel...")
		if err := remote.QuickDown(); err != nil {
			slog.Warn("failed to stop WireGuard", "error", err)
		} else {
			printSuccess("WireGuard tunnel stopped")
		}
	}

	// Remove WireGuard config from /etc/wireguard (requires sudo)
	remote.CleanupWireGuardConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to get home directory", "error", err)
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	vibespaceHome := filepath.Join(home, ".vibespace")

	// Detect platform and get manager
	p := platform.Detect()
	slog.Debug("platform detected", "os", p.OS, "arch", p.Arch)

	// Only uninstall cluster if binaries exist
	manager, err := platform.NewClusterManager(p, vibespaceHome, platform.ClusterManagerOptions{})
	if err != nil {
		slog.Warn("failed to create cluster manager", "error", err)
	} else {
		installed, _ := manager.IsInstalled()
		if installed {
			spinner := NewSpinner("Removing cluster...")
			spinner.Start()
			slog.Info("uninstalling cluster")
			ctx := context.Background()
			if err := manager.Uninstall(ctx); err != nil {
				slog.Warn("cluster uninstall had errors", "error", err)
			}
			spinner.Success("Cluster removed")
		}
	}

	// Remove entire ~/.vibespace/ directory
	spinner := NewSpinner("Removing vibespace data...")
	spinner.Start()
	slog.Info("removing vibespace home directory", "path", vibespaceHome)
	if err := os.RemoveAll(vibespaceHome); err != nil {
		spinner.Fail("Failed to remove vibespace data")
		slog.Error("failed to remove vibespace home", "error", err)
		return fmt.Errorf("failed to remove %s: %w", vibespaceHome, err)
	}
	spinner.Success("Vibespace data removed")

	slog.Info("uninstall completed successfully")
	printSuccess("vibespace has been completely removed")
	return nil
}
