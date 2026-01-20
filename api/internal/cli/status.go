package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"vibespace/internal/platform"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show cluster status",
	Long:  `Display the current status of the vibespace cluster and its components.`,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
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
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	slog.Info("stop command started")

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

	printStep("Stopping cluster...")
	slog.Info("stopping cluster")
	ctx := context.Background()
	if err := manager.Stop(ctx); err != nil {
		slog.Error("failed to stop cluster", "error", err)
		return fmt.Errorf("failed to stop cluster: %w", err)
	}

	slog.Info("stop completed successfully")
	printSuccess("Cluster stopped")
	return nil
}
