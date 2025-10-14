package k8s

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// ClusterContext represents a Kubernetes context
type ClusterContext struct {
	Name       string `json:"name"`
	Cluster    string `json:"cluster"`
	User       string `json:"user"`
	IsCurrent  bool   `json:"is_current"`
	IsLocal    bool   `json:"is_local"`
}

// ListContexts returns all available Kubernetes contexts
func ListContexts() ([]ClusterContext, error) {
	kubeconfig := getKubeconfigPath()

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	var contexts []ClusterContext
	for name, context := range config.Contexts {
		ctx := ClusterContext{
			Name:      name,
			Cluster:   context.Cluster,
			User:      context.AuthInfo,
			IsCurrent: name == config.CurrentContext,
			IsLocal:   isLocalCluster(name),
		}
		contexts = append(contexts, ctx)
	}

	return contexts, nil
}

// GetCurrentContext returns the current context name
func GetCurrentContext() (string, error) {
	kubeconfig := getKubeconfigPath()

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	return config.CurrentContext, nil
}

// SwitchContext switches to a different Kubernetes context
func SwitchContext(contextName string) error {
	kubeconfig := getKubeconfigPath()

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Verify context exists
	if _, exists := config.Contexts[contextName]; !exists {
		return fmt.Errorf("context %s does not exist", contextName)
	}

	// Update current context
	config.CurrentContext = contextName

	// Save config
	err = clientcmd.WriteToFile(*config, kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	return nil
}

// NewClientWithContext creates a new Kubernetes client for a specific context
func NewClientWithContext(contextName string) (*Client, error) {
	kubeconfig := getKubeconfigPath()

	// Build config for specific context
	configLoadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{
		CurrentContext: contextName,
	}

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides)
	restConfig, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build config for context %s: %w", contextName, err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		config:    restConfig,
	}, nil
}

// getKubeconfigPath returns the path to the kubeconfig file
func getKubeconfigPath() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	return kubeconfig
}

// isLocalCluster determines if a context is for a local development cluster
func isLocalCluster(contextName string) bool {
	localPrefixes := []string{
		"k3d-",
		"minikube",
		"docker-desktop",
		"rancher-desktop",
		"kind-",
		"colima",
	}

	for _, prefix := range localPrefixes {
		if len(contextName) >= len(prefix) && contextName[:len(prefix)] == prefix {
			return true
		}
	}

	return false
}
