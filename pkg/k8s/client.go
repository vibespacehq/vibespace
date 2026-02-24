package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/vibespacehq/vibespace/pkg/remote"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	VibespaceNamespace = "vibespace"
)

// Client wraps the Kubernetes client
type Client struct {
	clientset   kubernetes.Interface
	config      *rest.Config
	metricsset  metricsv.Interface
	metricsOnce sync.Once
	metricsErr  error
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
		clientset: clientset,
		config:    config,
	}, nil
}

// getK8sConfig returns the Kubernetes config based on connection mode.
// In remote mode, connects to a VPS cluster via WireGuard tunnel.
// In local mode, connects to bundled Kubernetes (Colima on macOS, k3s on Linux).
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

// getBundledKubeconfigPath returns the kubeconfig path based on connection mode.
// In remote mode, uses ~/.vibespace/remote_kubeconfig.
// In local mode, uses ~/.vibespace/kubeconfig.
func getBundledKubeconfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Check remote mode first
	if remote.IsConnected() {
		remotePath, err := remote.GetRemoteKubeconfigPath()
		if err != nil {
			return "", fmt.Errorf("failed to get remote kubeconfig path: %w", err)
		}
		if _, err := os.Stat(remotePath); err == nil {
			slog.Debug("using remote kubeconfig", "path", remotePath)
			return remotePath, nil
		}
		// Fall through to local if remote kubeconfig doesn't exist
		slog.Warn("remote mode active but kubeconfig missing, falling back to local")
	}

	// Use isolated kubeconfig to avoid conflicts with user's other clusters
	return filepath.Join(home, ".vibespace", "kubeconfig"), nil
}

// Clientset returns the underlying Kubernetes clientset
func (c *Client) Clientset() kubernetes.Interface {
	return c.clientset
}

// NewClientFromClientset creates a Client from an existing clientset (for testing).
func NewClientFromClientset(cs kubernetes.Interface) *Client {
	return &Client{clientset: cs}
}

// Config returns the underlying Kubernetes REST config
func (c *Client) Config() *rest.Config {
	return c.config
}

// MetricsClientset returns the metrics API clientset, creating it on first call.
func (c *Client) MetricsClientset() (metricsv.Interface, error) {
	c.metricsOnce.Do(func() {
		mc, err := metricsv.NewForConfig(c.config)
		if err != nil {
			c.metricsErr = fmt.Errorf("failed to create metrics clientset: %w", err)
			return
		}
		c.metricsset = mc
	})
	return c.metricsset, c.metricsErr
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
