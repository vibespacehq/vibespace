package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterConfig represents configuration settings for the cluster
type ClusterConfig struct {
	KnativeDomain     string // Domain suffix for Knative services (e.g., "local")
	RegistryURL       string // Local registry URL (e.g., "localhost:5000")
	TraefikNodePort   int    // Traefik HTTP NodePort (default: 30080)
	EnableScaleToZero bool   // Enable Knative scale-to-zero (Phase 2)
	DefaultMinScale   int    // Default minimum scale for workspaces (1 for Phase 1, 0 for Phase 2)
}

// DefaultClusterConfig returns the default configuration for local mode
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		KnativeDomain:     "local",
		RegistryURL:       "localhost:5000",
		TraefikNodePort:   30080,
		EnableScaleToZero: false, // Disabled for Phase 1
		DefaultMinScale:   1,     // Always running in Phase 1
	}
}

// ApplyConfiguration applies configuration to the cluster
func (c *Client) ApplyConfiguration(ctx context.Context, config *ClusterConfig) error {
	// Configure Knative domain
	if err := c.ConfigureKnativeDomain(ctx, config.KnativeDomain); err != nil {
		return fmt.Errorf("failed to configure Knative domain: %w", err)
	}

	// Configure Knative autoscaling defaults
	if err := c.ConfigureKnativeAutoscaling(ctx, config); err != nil {
		return fmt.Errorf("failed to configure Knative autoscaling: %w", err)
	}

	return nil
}

// ConfigureKnativeDomain configures the default domain for Knative services
func (c *Client) ConfigureKnativeDomain(ctx context.Context, domain string) error {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-domain",
			Namespace: "knative-serving",
		},
		Data: map[string]string{
			domain: "", // Empty value makes it the default domain
		},
	}

	// Try to create the ConfigMap
	_, err := c.clientset.CoreV1().ConfigMaps("knative-serving").Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			// Update existing ConfigMap
			_, err = c.clientset.CoreV1().ConfigMaps("knative-serving").Update(ctx, configMap, metav1.UpdateOptions{})
			if err != nil {
				return fmt.Errorf("failed to update config-domain ConfigMap: %w", err)
			}
		} else {
			return fmt.Errorf("failed to create config-domain ConfigMap: %w", err)
		}
	}

	return nil
}

// ConfigureKnativeAutoscaling configures Knative autoscaling settings
func (c *Client) ConfigureKnativeAutoscaling(ctx context.Context, config *ClusterConfig) error {
	// Configure autoscaling defaults
	autoscalingConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config-autoscaler",
			Namespace: "knative-serving",
		},
		Data: map[string]string{
			"enable-scale-to-zero": fmt.Sprintf("%t", config.EnableScaleToZero),
			"scale-to-zero-grace-period": "30s",
			"stable-window":              "60s",
			"panic-window-percentage":    "10.0",
			"panic-threshold-percentage": "200.0",
			"max-scale-up-rate":          "10.0",
			"max-scale-down-rate":        "2.0",
			"target-burst-capacity":      "200",
		},
	}

	// Try to get existing ConfigMap
	existing, err := c.clientset.CoreV1().ConfigMaps("knative-serving").Get(ctx, "config-autoscaler", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new ConfigMap
			_, err = c.clientset.CoreV1().ConfigMaps("knative-serving").Create(ctx, autoscalingConfig, metav1.CreateOptions{})
			if err != nil {
				return fmt.Errorf("failed to create config-autoscaler ConfigMap: %w", err)
			}
		} else {
			return fmt.Errorf("failed to get config-autoscaler ConfigMap: %w", err)
		}
	} else {
		// Merge with existing config (preserve other settings)
		if existing.Data == nil {
			existing.Data = make(map[string]string)
		}
		for k, v := range autoscalingConfig.Data {
			existing.Data[k] = v
		}

		_, err = c.clientset.CoreV1().ConfigMaps("knative-serving").Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update config-autoscaler ConfigMap: %w", err)
		}
	}

	return nil
}

// GetKnativeDomain returns the configured Knative domain
func (c *Client) GetKnativeDomain(ctx context.Context) (string, error) {
	configMap, err := c.clientset.CoreV1().ConfigMaps("knative-serving").Get(ctx, "config-domain", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", nil // No domain configured
		}
		return "", fmt.Errorf("failed to get config-domain: %w", err)
	}

	// Return the first (default) domain
	for domain := range configMap.Data {
		return domain, nil
	}

	return "", nil
}
