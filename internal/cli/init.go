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

	"github.com/yagizdagabak/vibespace/internal/platform"
	"github.com/yagizdagabak/vibespace/pkg/daemon"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the vibespace cluster",
	Long: `Initialize the vibespace cluster by downloading required binaries
and starting the Kubernetes environment.

On macOS: Downloads and starts Colima (Lima VM with k3s)
On Linux: Downloads and starts Lima (Lima VM with k3s)
On Linux with --bare-metal: Installs k3s directly on the host (no VM)

Use --external to skip cluster installation and use an existing kubeconfig.
Use --bare-metal on Linux to skip the VM layer and install k3s directly.`,
	Example: `  vibespace init
  vibespace init --cpu 4 --memory 8 --disk 60
  vibespace init --bare-metal
  vibespace init --external --kubeconfig ~/.kube/config`,
	RunE: runInit,
}

var (
	initExternal   bool
	initKubeconfig string
	initCPU        int
	initMemory     int
	initDisk       int
	initBareMetal  bool
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
	initCmd.Flags().BoolVar(&initBareMetal, "bare-metal", false, "Install k3s directly on the host (Linux only, no VM)")
}

func runInit(cmd *cobra.Command, args []string) error {
	slog.Info("init command started")
	ctx := context.Background()

	// Warn if connected to a remote server
	if isRemoteConnected() {
		fmt.Println("Warning: you are connected to a remote server.")
		fmt.Println("This will initialize a LOCAL cluster. Use 'vibespace remote disconnect' first if unintended.")
		fmt.Println()
	}

	// Clean up any stale daemon from previous runs
	if daemon.IsDaemonRunning() {
		daemon.StopDaemon()
		slog.Debug("stopped stale daemon")
	}

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
		return fmt.Errorf("windows is not supported, please use WSL2")
	}

	if initBareMetal && p.OS != "linux" {
		return fmt.Errorf("--bare-metal is only supported on Linux")
	}

	// Get the appropriate cluster manager
	manager, err := platform.NewClusterManager(p, vibespaceHome, platform.ClusterManagerOptions{BareMetal: initBareMetal})
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
		spinner := NewSpinner("Downloading required binaries...")
		spinner.Start()
		slog.Debug("downloading binaries")
		if err := manager.Install(ctx); err != nil {
			spinner.Fail("Failed to install binaries")
			slog.Error("failed to install binaries", "error", err)
			return fmt.Errorf("failed to install binaries: %w", err)
		}
		slog.Debug("binaries installed")
		spinner.Success("Binaries installed")
	} else {
		slog.Debug("binaries already installed")
		printSuccess("Binaries already installed")
	}

	// Smart state-based init: detect VM state and take appropriate action
	state := manager.GetVMState(ctx)
	slog.Debug("detected VM state", "state", state)

	config := platform.ClusterConfig{
		CPU:    initCPU,
		Memory: initMemory,
		Disk:   initDisk,
	}

	switch state {
	case platform.VMStateRunning:
		// Already running, just verify and continue
		slog.Debug("cluster already running")
		printSuccess("Cluster already running")

	case platform.VMStateStopped:
		// Resume existing VM (preserves data)
		spinner := NewSpinner("Resuming stopped cluster...")
		spinner.Start()
		slog.Info("resuming stopped cluster")
		if err := manager.Resume(ctx); err != nil {
			spinner.Fail("Failed to resume cluster")
			slog.Error("failed to resume cluster", "error", err)
			return fmt.Errorf("failed to resume cluster: %w", err)
		}
		spinner.Success("Cluster resumed")

	case platform.VMStateBroken:
		// Recover from broken state, then create fresh
		spinner := NewSpinner("Recovering from broken state...")
		spinner.Start()
		slog.Warn("recovering from broken cluster state")
		if err := manager.Recover(ctx); err != nil {
			slog.Warn("recovery cleanup failed", "error", err)
		}
		spinner.Success("Recovery cleanup completed")

		// Now start fresh
		spinner = NewSpinner(fmt.Sprintf("Starting cluster (CPU: %d, Memory: %dGB, Disk: %dGB)...", initCPU, initMemory, initDisk))
		spinner.Start()
		slog.Info("starting fresh cluster after recovery", "cpu", initCPU, "memory_gb", initMemory, "disk_gb", initDisk)
		if err := manager.Start(ctx, config); err != nil {
			spinner.Fail("Failed to start cluster")
			slog.Error("failed to start cluster", "error", err)
			return fmt.Errorf("failed to start cluster: %w", err)
		}
		spinner.Success("Cluster started")

	case platform.VMStateNotExists:
		// Create fresh VM
		spinner := NewSpinner(fmt.Sprintf("Starting cluster (CPU: %d, Memory: %dGB, Disk: %dGB)...", initCPU, initMemory, initDisk))
		spinner.Start()
		slog.Info("starting fresh cluster", "cpu", initCPU, "memory_gb", initMemory, "disk_gb", initDisk)
		if err := manager.Start(ctx, config); err != nil {
			spinner.Fail("Failed to start cluster")
			slog.Error("failed to start cluster", "error", err)
			return fmt.Errorf("failed to start cluster: %w", err)
		}
		spinner.Success("Cluster started")
	}

	// Persist cluster mode for subsequent commands
	var mode platform.ClusterMode
	switch {
	case initBareMetal:
		mode = platform.ClusterModeBareMetal
	case p.OS == "darwin":
		mode = platform.ClusterModeColima
	default:
		mode = platform.ClusterModeLima
	}
	if err := platform.SaveClusterState(vibespaceHome, mode); err != nil {
		slog.Warn("failed to save cluster state", "error", err)
	}

	// Wait for cluster to be ready
	spinner := NewSpinner("Waiting for cluster to be ready...")
	spinner.Start()
	slog.Debug("waiting for cluster readiness")
	if err := waitForCluster(ctx, manager); err != nil {
		spinner.Fail("Cluster failed to become ready")
		slog.Error("cluster failed to become ready", "error", err)
		return fmt.Errorf("cluster failed to become ready: %w", err)
	}
	slog.Debug("cluster ready")
	spinner.Success("Cluster is ready")

	// Install cluster components (namespace)
	printStep("Installing cluster components...")
	slog.Debug("installing cluster components")
	if err := installClusterComponents(ctx, vibespaceHome, manager.KubeconfigPath()); err != nil {
		slog.Error("failed to install cluster components", "error", err)
		return fmt.Errorf("failed to install cluster components: %w", err)
	}
	slog.Debug("cluster components installed")
	printSuccess("Cluster components installed")

	// Start daemon after cluster is ready
	if !daemon.IsDaemonRunning() {
		printStep("Starting daemon...")
		if err := daemon.SpawnDaemon(); err != nil {
			// Non-fatal - daemon can be started later
			slog.Warn("failed to start daemon", "error", err)
			printWarning("Daemon failed to start (will start on first connect)")
		} else {
			slog.Info("daemon started")
			printSuccess("Daemon started")
		}
	}

	slog.Info("init completed successfully")

	// JSON output
	out := getOutput()
	if out.IsJSONMode() {
		return out.JSON(NewJSONOutput(true, InitOutput{
			Platform: p.OS,
			CPU:      initCPU,
			Memory:   initMemory,
			Disk:     initDisk,
		}, nil))
	}

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

	// Install cluster components using the isolated kubeconfig
	printStep("Installing cluster components...")
	ctx := context.Background()
	if err := installClusterComponents(ctx, vibespaceHome, vibespaceKubeconfig); err != nil {
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

func installClusterComponents(ctx context.Context, vibespaceHome, kubeconfig string) error {
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
