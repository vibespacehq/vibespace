package k8s

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

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

	slog.Info("listing kubernetes contexts",
		"kubeconfig", kubeconfig)

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		slog.Error("failed to load kubeconfig",
			"kubeconfig", kubeconfig,
			"error", err)
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

	slog.Info("found kubernetes contexts",
		"count", len(contexts),
		"current", config.CurrentContext)

	return contexts, nil
}

// SwitchContext switches to a different Kubernetes context
func SwitchContext(contextName string) error {
	kubeconfig := getKubeconfigPath()

	slog.Info("switching kubernetes context",
		"context", contextName,
		"kubeconfig", kubeconfig)

	config, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		slog.Error("failed to load kubeconfig for context switch",
			"context", contextName,
			"error", err)
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	oldContext := config.CurrentContext

	// Verify context exists
	if _, exists := config.Contexts[contextName]; !exists {
		slog.Error("context does not exist",
			"context", contextName,
			"current", oldContext)
		return fmt.Errorf("context %s does not exist", contextName)
	}

	// Update current context
	config.CurrentContext = contextName

	// Save config
	err = clientcmd.WriteToFile(*config, kubeconfig)
	if err != nil {
		slog.Error("failed to write kubeconfig",
			"context", contextName,
			"error", err)
		return fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	slog.Info("kubernetes context switched successfully",
		"from", oldContext,
		"to", contextName,
		"is_remote", IsContextRemote(contextName))

	return nil
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

// IsContextRemote checks if a context is for a remote cluster
func IsContextRemote(contextName string) bool {
	return !isLocalCluster(contextName)
}
