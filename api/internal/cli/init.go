package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	initCPU        int
	initMemory     int
	initDisk       int
)

// Default cluster resource values - can be overridden via environment variables
const (
	DefaultClusterCPU    = 4
	DefaultClusterMemory = 8
	DefaultClusterDisk   = 60
)

// getEnvOrDefaultInt returns the environment variable as int or a default
func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func init() {
	// Read defaults from environment variables, falling back to constants
	cpuDefault := getEnvOrDefaultInt("VIBESPACE_CLUSTER_CPU", DefaultClusterCPU)
	memoryDefault := getEnvOrDefaultInt("VIBESPACE_CLUSTER_MEMORY", DefaultClusterMemory)
	diskDefault := getEnvOrDefaultInt("VIBESPACE_CLUSTER_DISK", DefaultClusterDisk)

	initCmd.Flags().BoolVar(&initExternal, "external", false, "Use an external Kubernetes cluster")
	initCmd.Flags().StringVar(&initKubeconfig, "kubeconfig", "", "Path to external kubeconfig")
	initCmd.Flags().IntVar(&initCPU, "cpu", cpuDefault, "Number of CPU cores for the cluster VM")
	initCmd.Flags().IntVar(&initMemory, "memory", memoryDefault, "Memory in GB for the cluster VM")
	initCmd.Flags().IntVar(&initDisk, "disk", diskDefault, "Disk size in GB for the cluster VM")
}

func runInit(cmd *cobra.Command, args []string) error {
	slog.Info("init command started")
	ctx := context.Background()

	// Determine vibespace home directory
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("failed to get home directory", "error", err)
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	vibespaceHome := filepath.Join(home, ".vibespace")
	slog.Debug("vibespace home directory", "path", vibespaceHome)

	// Create vibespace directory structure
	dirs := []string{
		vibespaceHome,
		filepath.Join(vibespaceHome, "bin"),
		filepath.Join(vibespaceHome, "cache"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			slog.Error("failed to create directory", "dir", dir, "error", err)
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	slog.Debug("directory structure created")

	// Handle external cluster mode
	if initExternal {
		slog.Info("using external cluster mode", "kubeconfig", initKubeconfig)
		return initExternalCluster(vibespaceHome, initKubeconfig)
	}

	// Detect platform
	p := platform.Detect()
	slog.Debug("platform detected", "os", p.OS, "arch", p.Arch)
	printStep("Detected platform: %s (%s)", p.OS, p.Arch)

	if p.OS == "windows" {
		slog.Error("unsupported platform", "os", p.OS)
		return fmt.Errorf("Windows is not supported. Please use WSL2")
	}

	// Get the appropriate cluster manager
	manager, err := platform.NewClusterManager(p, vibespaceHome)
	if err != nil {
		slog.Error("failed to create cluster manager", "error", err)
		return fmt.Errorf("failed to create cluster manager: %w", err)
	}

	// Check if already installed
	installed, err := manager.IsInstalled()
	if err != nil {
		slog.Error("failed to check installation", "error", err)
		return fmt.Errorf("failed to check installation: %w", err)
	}
	slog.Debug("installation check complete", "installed", installed)

	if !installed {
		printStep("Downloading required binaries...")
		slog.Debug("downloading binaries")
		if err := manager.Install(ctx); err != nil {
			slog.Error("failed to install binaries", "error", err)
			return fmt.Errorf("failed to install binaries: %w", err)
		}
		slog.Debug("binaries installed")
		printSuccess("Binaries installed")
	} else {
		slog.Debug("binaries already installed")
		printSuccess("Binaries already installed")
	}

	// Check if running
	running, err := manager.IsRunning()
	if err != nil {
		slog.Error("failed to check cluster status", "error", err)
		return fmt.Errorf("failed to check cluster status: %w", err)
	}
	slog.Debug("cluster status check", "running", running)

	if !running {
		printStep("Starting cluster (CPU: %d, Memory: %dGB, Disk: %dGB)...", initCPU, initMemory, initDisk)
		slog.Info("starting cluster", "cpu", initCPU, "memory_gb", initMemory, "disk_gb", initDisk)
		config := platform.ClusterConfig{
			CPU:    initCPU,
			Memory: initMemory,
			Disk:   initDisk,
		}
		if err := manager.Start(ctx, config); err != nil {
			slog.Error("failed to start cluster", "error", err)
			return fmt.Errorf("failed to start cluster: %w", err)
		}
		slog.Debug("cluster start command completed")
	} else {
		slog.Debug("cluster already running")
	}

	// Wait for cluster to be ready
	printStep("Waiting for cluster to be ready...")
	slog.Debug("waiting for cluster readiness")
	if err := waitForCluster(ctx, manager); err != nil {
		slog.Error("cluster failed to become ready", "error", err)
		return fmt.Errorf("cluster failed to become ready: %w", err)
	}
	slog.Debug("cluster ready")
	printSuccess("Cluster is ready")

	// Install cluster components (namespace)
	printStep("Installing cluster components...")
	slog.Debug("installing cluster components")
	if err := installClusterComponents(ctx, vibespaceHome); err != nil {
		slog.Error("failed to install cluster components", "error", err)
		return fmt.Errorf("failed to install cluster components: %w", err)
	}
	slog.Debug("cluster components installed")
	printSuccess("Cluster components installed")

	slog.Info("init completed successfully")
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
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubectlBin := filepath.Join(vibespaceHome, "bin", "kubectl")

	// Create vibespace namespace
	printStep("Creating vibespace namespace...")
	namespaceYAML := `apiVersion: v1
kind: Namespace
metadata:
  name: vibespace
  labels:
    app.kubernetes.io/name: vibespace`

	cmd := exec.CommandContext(ctx, kubectlBin, "--kubeconfig", kubeconfig, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(namespaceYAML)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	return nil
}

// Output helpers
var (
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
	cyan   = color.New(color.FgCyan).SprintFunc()
)

func printStep(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", cyan("→"), fmt.Sprintf(format, args...))
}

func printSuccess(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", green("✓"), fmt.Sprintf(format, args...))
}

func printWarning(format string, args ...interface{}) {
	fmt.Printf("%s %s\n", yellow("⚠"), fmt.Sprintf(format, args...))
}

func printError(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s %s\n", red("✗"), fmt.Sprintf(format, args...))
}
