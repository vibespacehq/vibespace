package k8s

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	WorkspaceNamespace = "workspace"
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
	clientset    *kubernetes.Clientset
	config       *rest.Config
	portForwards map[string]*PortForward
	pfMutex      sync.Mutex
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

	return &Client{
		clientset:    clientset,
		config:       config,
		portForwards: make(map[string]*PortForward),
	}, nil
}

// getK8sConfig returns the Kubernetes config
// Tries in-cluster config first, then falls back to kubeconfig
func getK8sConfig() (*rest.Config, error) {
	// Try in-cluster config
	config, err := rest.InClusterConfig()
	if err == nil {
		return config, nil
	}

	// Fall back to kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from kubeconfig: %w", err)
	}

	return config, nil
}

// Clientset returns the underlying Kubernetes clientset
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}

// EnsureNamespace ensures the workspace namespace exists
func (c *Client) EnsureNamespace(ctx context.Context) error {
	namespaces := c.clientset.CoreV1().Namespaces()

	_, err := namespaces.Get(ctx, WorkspaceNamespace, metav1.GetOptions{})
	if err == nil {
		// Namespace already exists
		return nil
	}

	// Create namespace
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: WorkspaceNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "workspace",
			},
		},
	}

	_, err = namespaces.Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

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

	key := fmt.Sprintf("%s/%s", namespace, keyName)

	// Check if port-forward already exists
	if pf, exists := c.portForwards[key]; exists {
		if pf.LocalPort == localPort && pf.RemotePort == remotePort {
			// Already running with same ports, nothing to do
			return nil
		}
		// Different ports, stop existing and create new
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

	// Start the command
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("failed to start port-forward: %w", err)
	}

	// Store port-forward info
	c.portForwards[key] = &PortForward{
		Service:    keyName,
		Namespace:  namespace,
		LocalPort:  localPort,
		RemotePort: remotePort,
		cmd:        cmd,
		cancel:     cancel,
	}

	return nil
}

// StopPortForward stops a kubectl port-forward
func (c *Client) StopPortForward(namespace, service string) error {
	c.pfMutex.Lock()
	defer c.pfMutex.Unlock()

	key := fmt.Sprintf("%s/%s", namespace, service)
	return c.stopPortForwardLocked(key)
}

// stopPortForwardLocked stops a port-forward (must be called with lock held)
func (c *Client) stopPortForwardLocked(key string) error {
	pf, exists := c.portForwards[key]
	if !exists {
		return nil // Already stopped
	}

	// Cancel context to stop kubectl
	if pf.cancel != nil {
		pf.cancel()
	}

	// Wait for process to exit
	if pf.cmd != nil && pf.cmd.Process != nil {
		_ = pf.cmd.Wait() // Ignore error, process may already be dead
	}

	delete(c.portForwards, key)
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
