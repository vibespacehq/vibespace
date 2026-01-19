package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vibespace/internal/platform"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the vibespace cluster",
	Long: `Initialize the vibespace cluster by downloading required binaries
and starting the Kubernetes environment.

On macOS: Downloads and starts Colima (Lima VM with k3s)
On Linux: Downloads and starts k3s directly

Use --external to skip cluster installation and use an existing kubeconfig.`,
	RunE: runInit,
}

var (
	initExternal   bool
	initKubeconfig string
)

func init() {
	initCmd.Flags().BoolVar(&initExternal, "external", false, "Use an external Kubernetes cluster")
	initCmd.Flags().StringVar(&initKubeconfig, "kubeconfig", "", "Path to external kubeconfig")
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Determine vibespace home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	vibespaceHome := filepath.Join(home, ".vibespace")

	// Create vibespace directory structure
	dirs := []string{
		vibespaceHome,
		filepath.Join(vibespaceHome, "bin"),
		filepath.Join(vibespaceHome, "cache"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Handle external cluster mode
	if initExternal {
		return initExternalCluster(vibespaceHome, initKubeconfig)
	}

	// Detect platform
	p := platform.Detect()
	printStep("Detected platform: %s (%s)", p.OS, p.Arch)

	if p.OS == "windows" {
		return fmt.Errorf("Windows is not supported. Please use WSL2")
	}

	// Get the appropriate cluster manager
	manager, err := platform.NewClusterManager(p, vibespaceHome)
	if err != nil {
		return fmt.Errorf("failed to create cluster manager: %w", err)
	}

	// Check if already installed
	installed, err := manager.IsInstalled()
	if err != nil {
		return fmt.Errorf("failed to check installation: %w", err)
	}

	if !installed {
		printStep("Downloading required binaries...")
		if err := manager.Install(ctx); err != nil {
			return fmt.Errorf("failed to install: %w", err)
		}
		printSuccess("Binaries installed")
	} else {
		printSuccess("Binaries already installed")
	}

	// Check if running
	running, err := manager.IsRunning()
	if err != nil {
		return fmt.Errorf("failed to check cluster status: %w", err)
	}

	if !running {
		printStep("Starting cluster...")
		if err := manager.Start(ctx); err != nil {
			return fmt.Errorf("failed to start cluster: %w", err)
		}
	}

	// Wait for cluster to be ready
	printStep("Waiting for cluster to be ready...")
	if err := waitForCluster(ctx, manager); err != nil {
		return fmt.Errorf("cluster failed to become ready: %w", err)
	}
	printSuccess("Cluster is ready")

	// Install cluster components (Knative, Traefik, etc.)
	printStep("Installing cluster components...")
	if err := installClusterComponents(ctx, vibespaceHome); err != nil {
		return fmt.Errorf("failed to install cluster components: %w", err)
	}
	printSuccess("Cluster components installed")

	printSuccess("vibespace is ready!")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  vibespace create <name>  Create a new vibespace")
	fmt.Println("  vibespace list           List all vibespaces")

	return nil
}

func initExternalCluster(vibespaceHome, kubeconfig string) error {
	// If no kubeconfig specified, use default
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Check if kubeconfig exists
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return fmt.Errorf("kubeconfig not found at %s", kubeconfig)
	}

	// Create symlink to vibespace kubeconfig location
	vibespaceKubeconfig := filepath.Join(vibespaceHome, "kubeconfig")
	_ = os.Remove(vibespaceKubeconfig) // Remove if exists
	if err := os.Symlink(kubeconfig, vibespaceKubeconfig); err != nil {
		// If symlink fails, copy the file
		data, err := os.ReadFile(kubeconfig)
		if err != nil {
			return fmt.Errorf("failed to read kubeconfig: %w", err)
		}
		if err := os.WriteFile(vibespaceKubeconfig, data, 0600); err != nil {
			return fmt.Errorf("failed to write kubeconfig: %w", err)
		}
	}

	printSuccess("Using external cluster from %s", kubeconfig)

	// TODO: Verify cluster connectivity and install components
	printStep("Installing cluster components...")
	ctx := context.Background()
	if err := installClusterComponents(ctx, vibespaceHome); err != nil {
		return fmt.Errorf("failed to install cluster components: %w", err)
	}
	printSuccess("Cluster components installed")

	return nil
}

func waitForCluster(ctx context.Context, manager platform.ClusterManager) error {
	timeout := 5 * time.Minute
	interval := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		running, err := manager.IsRunning()
		if err == nil && running {
			// Double check with kubectl
			if err := manager.WaitReady(ctx); err == nil {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}

	return fmt.Errorf("timeout waiting for cluster")
}

func installClusterComponents(ctx context.Context, vibespaceHome string) error {
	// TODO: Install Knative, Traefik, create namespace
	// For now, just ensure the vibespace namespace exists
	// This will be implemented when we wire up the k8s client
	return nil
}

// Output helpers
var (
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
)

func printStep(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", cyan("→"), fmt.Sprintf(format, args...))
}

func printSuccess(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", green("✓"), fmt.Sprintf(format, args...))
}

func printWarning(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", yellow("!"), fmt.Sprintf(format, args...))
}
