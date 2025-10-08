package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	WorkspaceNamespace = "workspace"
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
