package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"vibespace/internal/platform"
	"vibespace/pkg/daemon"
	"vibespace/pkg/k8s"
	"vibespace/pkg/model"
	"vibespace/pkg/vibespace"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Common error messages
const (
	errClusterNotInitialized = "cluster not initialized. Run 'vibespace init' first"
	errClusterNotRunning     = "cluster is not running. Run 'vibespace init' to start it"
	errClusterUnreachable    = "cannot connect to cluster. Check if cluster is running with 'vibespace status'"
)

// checkClusterInitialized checks if the cluster has been initialized (kubeconfig exists)
func checkClusterInitialized() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	kubeconfig := filepath.Join(home, ".kube", "config")
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		return fmt.Errorf(errClusterNotInitialized)
	}

	return nil
}

// checkClusterRunning checks if the cluster is actually running and reachable
func checkClusterRunning() error {
	// First check if initialized
	if err := checkClusterInitialized(); err != nil {
		return err
	}

	// Check if cluster manager reports running
	home, _ := os.UserHomeDir()
	vibespaceHome := filepath.Join(home, ".vibespace")

	p := platform.Detect()
	manager, err := platform.NewClusterManager(p, vibespaceHome)
	if err != nil {
		return fmt.Errorf(errClusterNotRunning)
	}

	running, err := manager.IsRunning()
	if err != nil || !running {
		return fmt.Errorf(errClusterNotRunning)
	}

	return nil
}

// checkClusterReachable does a quick connectivity test to the k8s API
func checkClusterReachable() error {
	if err := checkClusterRunning(); err != nil {
		return err
	}

	// Try to create a k8s client and do a quick health check
	client, err := k8s.NewClient()
	if err != nil {
		return fmt.Errorf(errClusterUnreachable)
	}

	// Quick timeout context for health check
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to list namespaces as a quick connectivity test
	_, err = client.Clientset().CoreV1().Namespaces().List(ctx, k8sListOptions())
	if err != nil {
		return fmt.Errorf("%s: %w", errClusterUnreachable, err)
	}

	return nil
}

// k8sListOptions returns empty list options
func k8sListOptions() metav1.ListOptions {
	return metav1.ListOptions{}
}

// getVibespaceServiceWithCheck creates the vibespace service with prerequisite checks
func getVibespaceServiceWithCheck() (*vibespace.Service, error) {
	// Check cluster is running
	if err := checkClusterRunning(); err != nil {
		return nil, err
	}

	// Get kubeconfig path
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	kubeconfig := filepath.Join(home, ".kube", "config")

	// Set KUBECONFIG environment variable for the k8s client
	os.Setenv("KUBECONFIG", kubeconfig)

	// Create k8s client
	k8sClient, err := k8s.NewClient()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errClusterUnreachable, err)
	}

	// Create vibespace service
	svc := vibespace.NewService(k8sClient)
	return svc, nil
}

// checkVibespaceExists checks if a vibespace exists and returns it
func checkVibespaceExists(ctx context.Context, svc *vibespace.Service, name string) (*model.Vibespace, error) {
	vs, err := svc.Get(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("vibespace '%s' not found. List available: vibespace list", name)
	}
	return vs, nil
}

// checkVibespaceRunning checks if a vibespace exists and is running
func checkVibespaceRunning(ctx context.Context, svc *vibespace.Service, name string) (*model.Vibespace, error) {
	vs, err := checkVibespaceExists(ctx, svc, name)
	if err != nil {
		return nil, err
	}

	switch vs.Status {
	case "running":
		return vs, nil
	case "creating":
		return nil, fmt.Errorf("vibespace '%s' is still creating. Wait for it to be ready or check: vibespace list", name)
	case "stopped":
		return nil, fmt.Errorf("vibespace '%s' is stopped. Start it with: vibespace %s start", name, name)
	case "error":
		return nil, fmt.Errorf("vibespace '%s' is in error state. Check logs or delete and recreate", name)
	default:
		return nil, fmt.Errorf("vibespace '%s' is %s. Start it with: vibespace %s start", name, vs.Status, name)
	}
}

// checkDaemonRunning checks if the daemon is running for a vibespace
func checkDaemonRunning(name string) error {
	if !daemon.IsRunning(name) {
		return fmt.Errorf("port-forward daemon not running. Start it with: vibespace %s up", name)
	}
	return nil
}

// checkDaemonRunningWithHint checks daemon and suggests log location on failure
func checkDaemonRunningWithHint(name string) error {
	if !daemon.IsRunning(name) {
		paths, _ := daemon.GetDaemonPaths(name)
		logPath := filepath.Join(paths.Dir, name+".log")
		return fmt.Errorf("port-forward daemon not running. Start it with: vibespace %s up\nIf it crashed, check logs: %s", name, logPath)
	}
	return nil
}

// wrapKubectlError wraps kubectl errors with helpful context
func wrapKubectlError(err error, operation, vibespace string) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Connection refused - cluster not running
	if strings.Contains(errStr, "connection refused") {
		return fmt.Errorf("cannot %s: cluster not reachable. Check: vibespace status", operation)
	}

	// No resources found
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "No resources") {
		return fmt.Errorf("cannot %s: no pods found for '%s'. Check: vibespace %s agents", operation, vibespace, vibespace)
	}

	// Timeout
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return fmt.Errorf("cannot %s: operation timed out. Cluster may be overloaded. Try again", operation)
	}

	// Default: wrap with context
	return fmt.Errorf("failed to %s: %w", operation, err)
}

// findPodForVibespace finds a running pod for a vibespace with helpful errors
func findPodForVibespace(ctx context.Context, vibespace string) (string, error) {
	home, _ := os.UserHomeDir()
	kubeconfig := filepath.Join(home, ".kube", "config")
	kubectlBin := filepath.Join(home, ".vibespace", "bin", "kubectl")

	// Check kubectl exists
	if _, err := os.Stat(kubectlBin); os.IsNotExist(err) {
		return "", fmt.Errorf("kubectl not found. Run 'vibespace init' to install it")
	}

	podSelector := fmt.Sprintf("vibespace.dev/id=%s", vibespace)

	findCmd := exec.CommandContext(ctx, kubectlBin,
		"--kubeconfig", kubeconfig,
		"-n", "vibespace",
		"get", "pod",
		"-l", podSelector,
		"-o", "jsonpath={.items[0].metadata.name}",
	)

	podNameBytes, err := findCmd.Output()
	if err != nil {
		return "", wrapKubectlError(err, "find pod", vibespace)
	}

	podName := strings.TrimSpace(string(podNameBytes))
	if podName == "" {
		return "", fmt.Errorf("no running pod found for '%s'. The vibespace may be scaled to zero or starting up.\nCheck status: vibespace list\nStart it: vibespace %s start", vibespace, vibespace)
	}

	return podName, nil
}

// printErrorWithHint prints an error with a helpful hint
func printErrorWithHint(err error, hint string) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	if hint != "" {
		fmt.Fprintf(os.Stderr, "Hint: %s\n", hint)
	}
}
