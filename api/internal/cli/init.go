package cli

import (
	"context"
	"fmt"
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
			return fmt.Errorf("failed to install binaries: %w", err)
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
		printStep("Starting cluster (CPU: %d, Memory: %dGB, Disk: %dGB)...", initCPU, initMemory, initDisk)
		config := platform.ClusterConfig{
			CPU:    initCPU,
			Memory: initMemory,
			Disk:   initDisk,
		}
		if err := manager.Start(ctx, config); err != nil {
			return fmt.Errorf("failed to start cluster: %w", err)
		}
	}

	// Wait for cluster to be ready
	printStep("Waiting for cluster to be ready...")
	if err := waitForCluster(ctx, manager); err != nil {
		return fmt.Errorf("cluster failed to become ready: %w", err)
	}
	printSuccess("Cluster is ready")

	// Install cluster components (Knative)
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
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubectlBin := filepath.Join(vibespaceHome, "bin", "kubectl")

	// Helper to run kubectl commands
	runKubectl := func(args ...string) error {
		allArgs := append([]string{"--kubeconfig", kubeconfig}, args...)
		cmd := exec.CommandContext(ctx, kubectlBin, allArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Helper to apply manifests
	applyManifest := func(path string) error {
		return runKubectl("apply", "-f", path)
	}

	// 1. Create vibespace namespace
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

	// 2. Get the manifest directory path
	// First check if running from source (development)
	manifestDir := ""

	// Try relative to current working directory (development)
	cwd, _ := os.Getwd()
	devManifestDir := filepath.Join(cwd, "pkg", "k8s", "manifests")
	if _, err := os.Stat(devManifestDir); err == nil {
		manifestDir = devManifestDir
	}

	// Try relative to executable (installed binary)
	if manifestDir == "" {
		exePath, _ := os.Executable()
		exeDir := filepath.Dir(exePath)
		installedManifestDir := filepath.Join(exeDir, "manifests")
		if _, err := os.Stat(installedManifestDir); err == nil {
			manifestDir = installedManifestDir
		}
	}

	// Try in vibespace home
	if manifestDir == "" {
		homeManifestDir := filepath.Join(vibespaceHome, "manifests")
		if _, err := os.Stat(homeManifestDir); err == nil {
			manifestDir = homeManifestDir
		}
	}

	if manifestDir == "" {
		printWarning("Manifests not found, skipping Knative installation")
		printWarning("You can install them manually or run from the source directory")
		return nil
	}

	// 3. Install Knative Serving CRDs
	knativeCRDs := filepath.Join(manifestDir, "knative", "serving-crds.yaml")
	if _, err := os.Stat(knativeCRDs); err == nil {
		printStep("Installing Knative CRDs...")
		if err := applyManifest(knativeCRDs); err != nil {
			return fmt.Errorf("failed to install Knative CRDs: %w", err)
		}
	}

	// 4. Install Knative Serving Core
	knativeCore := filepath.Join(manifestDir, "knative", "serving-core.yaml")
	if _, err := os.Stat(knativeCore); err == nil {
		printStep("Installing Knative Serving...")
		if err := applyManifest(knativeCore); err != nil {
			return fmt.Errorf("failed to install Knative Serving: %w", err)
		}
	}

	// 5. Wait for Knative controller to be ready
	printStep("Waiting for Knative to be ready...")
	knativeReady := false
	for i := 0; i < 30; i++ {
		cmd := exec.CommandContext(ctx, kubectlBin,
			"--kubeconfig", kubeconfig,
			"-n", "knative-serving",
			"get", "deploy", "controller",
			"-o", "jsonpath={.status.readyReplicas}",
		)
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "1" {
			knativeReady = true
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	if !knativeReady {
		printWarning("Knative may still be starting up. Check with: kubectl -n knative-serving get pods")
		return nil
	}

	// 6. Wait for webhook to be ready (required for ConfigMap validation)
	printStep("Waiting for Knative webhook...")
	for i := 0; i < 30; i++ {
		cmd := exec.CommandContext(ctx, kubectlBin,
			"--kubeconfig", kubeconfig,
			"-n", "knative-serving",
			"get", "deploy", "webhook",
			"-o", "jsonpath={.status.readyReplicas}",
		)
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "1" {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}

	// 7. Configure Knative features for vibespace (PVC support, init containers)
	printStep("Configuring Knative features...")
	featuresPatch := `{"data":{"kubernetes.podspec-persistent-volume-claim":"enabled","kubernetes.podspec-persistent-volume-write":"enabled","kubernetes.podspec-init-containers":"enabled"}}`
	cmd = exec.CommandContext(ctx, kubectlBin,
		"--kubeconfig", kubeconfig,
		"-n", "knative-serving",
		"patch", "configmap", "config-features",
		"--type", "merge",
		"-p", featuresPatch,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to configure Knative features: %w", err)
	}

	// 8. Configure Knative defaults (revision timeout)
	defaultsPatch := `{"data":{"revision-timeout-seconds":"600"}}`
	cmd = exec.CommandContext(ctx, kubectlBin,
		"--kubeconfig", kubeconfig,
		"-n", "knative-serving",
		"patch", "configmap", "config-defaults",
		"--type", "merge",
		"-p", defaultsPatch,
	)
	if err := cmd.Run(); err != nil {
		printWarning("Failed to configure Knative defaults: %v", err)
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
