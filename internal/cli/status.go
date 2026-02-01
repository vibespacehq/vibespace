package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/list"
	"github.com/yagizdagabak/vibespace/internal/platform"
	"github.com/yagizdagabak/vibespace/pkg/daemon"
	"github.com/yagizdagabak/vibespace/pkg/remote"
	"github.com/yagizdagabak/vibespace/pkg/ui"

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

	// Check remote connection status
	remoteState, _ := remote.LoadRemoteState()

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

		// Add remote status
		if remoteState != nil && remoteState.Connected {
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

	if !installed {
		fmt.Printf("Cluster: %s\n", out.Orange("not installed"))
		fmt.Println()
		fmt.Printf("Run %s to set up the cluster\n", out.Teal("vibespace init"))
		return nil
	}

	// Check running status
	running, err := manager.IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check cluster status: %w", err)
	}

	if !running {
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

	// Main list
	l := list.New(
		fmt.Sprintf("%s %s", boldStyle.Render("Cluster"), tealStyle.Render("running")),
	)

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

			// Vibespaces as separate top-level item
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
	} else {
		l.Item(fmt.Sprintf("%s %s", boldStyle.Render("Daemon"), out.Dim("not running")))
	}

	// Remote connection section
	if remoteState != nil && remoteState.Connected {
		l.Item(fmt.Sprintf("%s %s", boldStyle.Render("Remote"), tealStyle.Render("connected")))
		l.Item(list.New(
			fmt.Sprintf("server %s", pinkStyle.Render(remoteState.ServerHost)),
			fmt.Sprintf("local IP %s", pinkStyle.Render(remoteState.LocalIP)),
		))
	}

	fmt.Println(l)
	return nil
}


var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the cluster",
	Long:  `Stop the vibespace cluster. Data is preserved and can be started again with 'vibespace init'.`,
	Example: `  vibespace stop`,
	RunE: runClusterStop,
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove vibespace and all cluster data",
	Long: `Completely remove vibespace including:
- The Colima VM (macOS) or k3s cluster (Linux)
- All vibespace data in ~/.vibespace/
- All vibespaces and their data

This action cannot be undone. Your ~/.kube/config is NOT affected.`,
	Example: `  vibespace uninstall`,
	RunE:    runUninstall,
}

func runClusterStop(cmd *cobra.Command, args []string) error {
	slog.Info("stop command started")
	out := getOutput()

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

	fmt.Println("This will remove ALL vibespace data including:")
	fmt.Println("  - The Colima VM (macOS) or k3s cluster (Linux)")
	fmt.Println("  - All vibespaces and their data")
	fmt.Println("  - All downloaded binaries")
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

	// Detect platform and get manager
	p := platform.Detect()
	slog.Debug("platform detected", "os", p.OS, "arch", p.Arch)

	// Only uninstall cluster if binaries exist
	manager, err := platform.NewClusterManager(p, vibespaceHome)
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
