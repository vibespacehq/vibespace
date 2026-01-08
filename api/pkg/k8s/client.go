package k8s

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	VibespaceNamespace = "vibespace"
)

// PortForward represents an active kubectl port-forward
type PortForward struct {
	Service    string
	Namespace  string
	LocalPort  int
	RemotePort int
	cmd        *exec.Cmd
	cancel     context.CancelFunc
}

// Client wraps the Kubernetes client
type Client struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	config        *rest.Config
	portForwards  map[string]*PortForward
	pfMutex       sync.Mutex
}

// NewClient creates a new Kubernetes client
func NewClient() (*Client, error) {
	config, err := getK8sConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s clientset: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s dynamic client: %w", err)
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynClient,
		config:        config,
		portForwards:  make(map[string]*PortForward),
	}, nil
}

// getK8sConfig returns the Kubernetes config for bundled Kubernetes (LOCAL MODE only).
//
// DEPLOYMENT MODE: This function is for LOCAL MODE, where bundled Kubernetes
// (Colima on macOS, k3s on Linux) runs on the same machine as the API server.
//
// With ADR 0006, we use bundled Kubernetes with known kubeconfig locations
// instead of detecting external installations.
//
// For REMOTE MODE (planned Post-MVP), the API server would run on a VPS and
// access k8s using in-cluster config or a provided kubeconfig path.
func getK8sConfig() (*rest.Config, error) {
	// Get bundled kubeconfig path
	kubeconfig, err := getBundledKubeconfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get bundled kubeconfig path: %w", err)
	}

	// Build config from bundled kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from bundled kubeconfig %s: %w", kubeconfig, err)
	}

	return config, nil
}

// getBundledKubeconfigPath returns the kubeconfig path for bundled Kubernetes (LOCAL MODE).
// - macOS (Colima): ~/.kube/config (Colima updates the standard kubeconfig with "colima" context)
// - Linux (k3s): ~/.kube/config
//
// For REMOTE MODE (planned Post-MVP), this would likely use in-cluster config or
// a configurable kubeconfig path from environment variables.
func getBundledKubeconfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Both macOS (Colima) and Linux (k3s) use ~/.kube/config
	// Colima updates this file with the "colima" context instead of creating a separate file
	return filepath.Join(home, ".kube", "config"), nil
}

// Clientset returns the underlying Kubernetes clientset
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}

// DynamicClient returns the underlying Kubernetes dynamic client
func (c *Client) DynamicClient() dynamic.Interface {
	return c.dynamicClient
}

// Config returns the underlying Kubernetes REST config
func (c *Client) Config() *rest.Config {
	return c.config
}

// EnsureNamespace ensures the vibespace namespace exists
func (c *Client) EnsureNamespace(ctx context.Context) error {
	namespaces := c.clientset.CoreV1().Namespaces()

	_, err := namespaces.Get(ctx, VibespaceNamespace, metav1.GetOptions{})
	if err == nil {
		// Namespace already exists
		return nil
	}

	// Create namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: VibespaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "vibespace",
			},
		},
	}

	_, err = namespaces.Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	slog.Info("vibespace namespace created", "namespace", VibespaceNamespace)
	return nil
}

// StartPortForward starts a kubectl port-forward to a service
// Returns immediately after starting the port-forward process in the background
func (c *Client) StartPortForward(ctx context.Context, namespace, service string, localPort, remotePort int) error {
	return c.startPortForwardToResource(ctx, namespace, fmt.Sprintf("svc/%s", service), service, localPort, remotePort)
}

// StartPortForwardToPod starts a kubectl port-forward to a pod
// Returns immediately after starting the port-forward process in the background
func (c *Client) StartPortForwardToPod(ctx context.Context, namespace, podName string, localPort, remotePort int) error {
	return c.startPortForwardToResource(ctx, namespace, fmt.Sprintf("pod/%s", podName), podName, localPort, remotePort)
}

// startPortForwardToResource is the internal implementation for port-forwarding
func (c *Client) startPortForwardToResource(ctx context.Context, namespace, resource, keyName string, localPort, remotePort int) error {
	c.pfMutex.Lock()
	defer c.pfMutex.Unlock()

	slog.Info("starting port-forward to resource",
		"namespace", namespace,
		"resource", resource,
		"key_name", keyName,
		"local_port", localPort,
		"remote_port", remotePort)

	// Include remote port in key to allow multiple port-forwards to same pod
	key := fmt.Sprintf("%s/%s:%d", namespace, keyName, remotePort)

	// Check if port-forward already exists
	if pf, exists := c.portForwards[key]; exists {
		// Check if the process is still running
		processAlive := false
		if pf.cmd != nil && pf.cmd.Process != nil {
			// Check if process is still running by sending signal 0
			if err := pf.cmd.Process.Signal(syscall.Signal(0)); err == nil {
				processAlive = true
			}
		}

		if processAlive && pf.LocalPort == localPort && pf.RemotePort == remotePort {
			// Already running with same ports and process is alive
			slog.Info("port-forward already exists and is alive",
				"key", key,
				"local_port", localPort,
				"remote_port", remotePort,
				"pid", pf.cmd.Process.Pid)
			return nil
		}

		// Process is dead or ports are different, stop existing and create new
		slog.Info("stopping stale or mismatched port-forward",
			"key", key,
			"process_alive", processAlive,
			"old_local_port", pf.LocalPort,
			"new_local_port", localPort)
		c.stopPortForwardLocked(key)
	}

	// Create detached context for port-forward
	// Important: Use Background() not the parent ctx, otherwise the port-forward
	// will be cancelled when the HTTP request context times out
	pfCtx, cancel := context.WithCancel(context.Background())

	// Build kubectl port-forward command
	cmd := exec.CommandContext(pfCtx, "kubectl", "port-forward",
		"-n", namespace,
		resource,
		fmt.Sprintf("%d:%d", localPort, remotePort),
	)

	// Set KUBECONFIG environment variable
	kubeconfig, err := getBundledKubeconfigPath()
	if err == nil {
		cmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
	}

	// Capture stderr for debugging
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	cmdStr := fmt.Sprintf("kubectl port-forward -n %s %s %d:%d", namespace, resource, localPort, remotePort)
	slog.Info("executing kubectl port-forward command",
		"command", cmdStr,
		"key", key)

	// Start the command
	if err := cmd.Start(); err != nil {
		cancel()
		stderr := stderrBuf.String()
		slog.Error("failed to start port-forward process",
			"key", key,
			"command", cmdStr,
			"error", err,
			"stderr", stderr)
		return fmt.Errorf("failed to start port-forward: %w", err)
	}

	slog.Info("port-forward process started successfully",
		"key", key,
		"pid", cmd.Process.Pid,
		"local_port", localPort,
		"remote_port", remotePort)

	// Store port-forward info
	c.portForwards[key] = &PortForward{
		Service:    keyName,
		Namespace:  namespace,
		LocalPort:  localPort,
		RemotePort: remotePort,
		cmd:        cmd,
		cancel:     cancel,
	}

	slog.Info("port-forward tracked successfully",
		"key", key,
		"total_active_port_forwards", len(c.portForwards))

	return nil
}

// StopPortForward stops all kubectl port-forwards for a given service/pod
func (c *Client) StopPortForward(namespace, service string) error {
	c.pfMutex.Lock()
	defer c.pfMutex.Unlock()

	slog.Info("stopping port-forwards for service",
		"namespace", namespace,
		"service", service)

	// Stop all port-forwards matching this namespace/service prefix
	// Since keys now include remote port (namespace/service:port), we need to find all matches
	prefix := fmt.Sprintf("%s/%s:", namespace, service)

	keysToDelete := []string{}
	for key := range c.portForwards {
		if key == fmt.Sprintf("%s/%s", namespace, service) || // Old format (for backward compatibility)
		   len(key) >= len(prefix) && key[:len(prefix)] == prefix { // New format with port
			keysToDelete = append(keysToDelete, key)
		}
	}

	slog.Info("found port-forwards to stop",
		"namespace", namespace,
		"service", service,
		"count", len(keysToDelete),
		"keys", keysToDelete)

	for _, key := range keysToDelete {
		c.stopPortForwardLocked(key)
	}

	slog.Info("stopped all port-forwards for service",
		"namespace", namespace,
		"service", service,
		"count", len(keysToDelete))

	return nil
}

// stopPortForwardLocked stops a port-forward (must be called with lock held)
func (c *Client) stopPortForwardLocked(key string) error {
	pf, exists := c.portForwards[key]
	if !exists {
		slog.Debug("port-forward already stopped or does not exist",
			"key", key)
		return nil // Already stopped
	}

	slog.Info("stopping port-forward",
		"key", key,
		"service", pf.Service,
		"namespace", pf.Namespace,
		"local_port", pf.LocalPort,
		"remote_port", pf.RemotePort)

	// Cancel context to stop kubectl
	if pf.cancel != nil {
		pf.cancel()
	}

	// Wait for process to exit
	if pf.cmd != nil && pf.cmd.Process != nil {
		pid := pf.cmd.Process.Pid
		_ = pf.cmd.Wait() // Ignore error, process may already be dead
		slog.Info("port-forward process terminated",
			"key", key,
			"pid", pid)
	}

	delete(c.portForwards, key)
	slog.Info("port-forward stopped and removed from tracking",
		"key", key)
	return nil
}

// StopAllPortForwards stops all active port-forwards
func (c *Client) StopAllPortForwards() {
	c.pfMutex.Lock()
	defer c.pfMutex.Unlock()

	for key := range c.portForwards {
		_ = c.stopPortForwardLocked(key)
	}
}
