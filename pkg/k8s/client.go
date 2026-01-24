package k8s

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	VibespaceNamespace = "vibespace"
)

// Client wraps the Kubernetes client
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
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
// Uses isolated kubeconfig at ~/.vibespace/kubeconfig to avoid touching user's ~/.kube/config.
//
// For REMOTE MODE (planned Post-MVP), this would likely use in-cluster config or
// a configurable kubeconfig path from environment variables.
func getBundledKubeconfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Use isolated kubeconfig to avoid conflicts with user's other clusters
	return filepath.Join(home, ".vibespace", "kubeconfig"), nil
}

// Clientset returns the underlying Kubernetes clientset
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
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
