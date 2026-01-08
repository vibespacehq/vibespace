package k8s

import (
	"context"
	"fmt"
	"log/slog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentStatus represents the status of a cluster component
type ComponentStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Healthy   bool   `json:"healthy"`
	Error     string `json:"error,omitempty"`
}

// ClusterComponents represents all required components
type ClusterComponents struct {
	Knative  ComponentStatus `json:"knative"`
	Traefik  ComponentStatus `json:"traefik"`
	Registry ComponentStatus `json:"registry"`
}

// CheckComponents checks the status of all required cluster components
func (c *Client) CheckComponents(ctx context.Context) (*ClusterComponents, error) {
	slog.Info("checking cluster components")

	// Check if client is nil (k8s not available yet)
	if c.clientset == nil {
		slog.Warn("k8s clientset not initialized, attempting to reinitialize")
		// Try to reinitialize the client
		newClient, err := NewClient()
		if err != nil {
			return nil, fmt.Errorf("kubernetes not available: %w", err)
		}
		// Update the clientset
		c.clientset = newClient.clientset
		c.config = newClient.config
	}

	components := &ClusterComponents{}

	// Check Knative
	components.Knative = c.checkKnative(ctx)

	// Check Traefik
	components.Traefik = c.checkTraefik(ctx)

	// Check Registry
	components.Registry = c.checkRegistry(ctx)

	slog.Info("cluster component check completed",
		"all_ready", components.AllComponentsReady(),
		"knative_healthy", components.Knative.Healthy,
		"traefik_healthy", components.Traefik.Healthy,
		"registry_healthy", components.Registry.Healthy)

	return components, nil
}

// checkKnative checks if Knative Serving is installed and healthy
func (c *Client) checkKnative(ctx context.Context) ComponentStatus {
	slog.Debug("checking knative component")

	status := ComponentStatus{Installed: false, Healthy: false}

	// Check if knative-serving namespace exists
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, "knative-serving", metav1.GetOptions{})
	if err != nil {
		status.Error = "knative-serving namespace not found"
		slog.Debug("knative check failed", "reason", status.Error)
		return status
	}

	status.Installed = true

	// Check if controller deployment is running
	deployment, err := c.clientset.AppsV1().Deployments("knative-serving").Get(ctx, "controller", metav1.GetOptions{})
	if err != nil {
		status.Error = "controller deployment not found"
		slog.Debug("knative check failed", "reason", status.Error)
		return status
	}

	// Check if controller deployment is ready
	if deployment.Status.ReadyReplicas == 0 {
		status.Error = "controller deployment not ready"
		slog.Debug("knative check failed", "reason", status.Error)
		return status
	}

	// Check if webhook deployment is running and ready
	webhookDeployment, err := c.clientset.AppsV1().Deployments("knative-serving").Get(ctx, "webhook", metav1.GetOptions{})
	if err != nil {
		status.Error = "webhook deployment not found"
		return status
	}

	if webhookDeployment.Status.ReadyReplicas == 0 {
		status.Error = "webhook deployment not ready"
		return status
	}

	// Check if webhook certificates secret exists with required keys
	secret, err := c.clientset.CoreV1().Secrets("knative-serving").Get(ctx, "webhook-certs", metav1.GetOptions{})
	if err != nil {
		status.Error = "webhook certificates not found"
		return status
	}

	// Verify required certificate keys exist
	requiredKeys := []string{"ca-cert.pem", "server-cert.pem", "server-key.pem"}
	for _, key := range requiredKeys {
		if _, exists := secret.Data[key]; !exists {
			status.Error = fmt.Sprintf("webhook certificate missing %s", key)
			return status
		}
	}

	// Check if webhook configurations exist (indicates webhooks are registered)
	_, err = c.clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, "validation.webhook.serving.knative.dev", metav1.GetOptions{})
	if err != nil {
		status.Error = "validating webhook not registered"
		return status
	}

	_, err = c.clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, "webhook.serving.knative.dev", metav1.GetOptions{})
	if err != nil {
		status.Error = "mutating webhook not registered"
		return status
	}

	// All checks passed
	status.Healthy = true

	// Try to get version from labels
	if version, ok := deployment.Labels["serving.knative.dev/release"]; ok {
		status.Version = version
	}

	slog.Debug("knative component healthy", "version", status.Version)

	return status
}

// checkTraefik checks if Traefik is installed and healthy
func (c *Client) checkTraefik(ctx context.Context) ComponentStatus {
	slog.Debug("checking traefik component")

	status := ComponentStatus{Installed: false, Healthy: false}

	// Check if traefik namespace exists
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, "traefik", metav1.GetOptions{})
	if err != nil {
		status.Error = "traefik namespace not found"
		slog.Debug("traefik check failed", "reason", status.Error)
		return status
	}

	status.Installed = true

	// Check if traefik deployment is running
	deployment, err := c.clientset.AppsV1().Deployments("traefik").Get(ctx, "traefik", metav1.GetOptions{})
	if err != nil {
		status.Error = "traefik deployment not found"
		slog.Debug("traefik check failed", "reason", status.Error)
		return status
	}

	// Check if deployment is ready
	if deployment.Status.ReadyReplicas > 0 {
		status.Healthy = true
	} else {
		status.Error = "traefik deployment not ready"
		slog.Debug("traefik check failed", "reason", status.Error)
	}

	// Get version from image tag
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image := deployment.Spec.Template.Spec.Containers[0].Image
		status.Version = image
	}

	if status.Healthy {
		slog.Debug("traefik component healthy", "version", status.Version)
	}

	return status
}

// checkRegistry checks if the local registry is installed and healthy
func (c *Client) checkRegistry(ctx context.Context) ComponentStatus {
	slog.Debug("checking registry component")

	status := ComponentStatus{Installed: false, Healthy: false}

	// Check if registry deployment exists in default namespace
	deployment, err := c.clientset.AppsV1().Deployments("default").Get(ctx, "registry", metav1.GetOptions{})
	if err != nil {
		status.Error = "registry deployment not found"
		slog.Debug("registry check failed", "reason", status.Error)
		return status
	}

	status.Installed = true

	// Check if deployment is ready
	if deployment.Status.ReadyReplicas > 0 {
		status.Healthy = true
	} else {
		status.Error = "registry deployment not ready"
		slog.Debug("registry check failed", "reason", status.Error)
	}

	// Get version from image tag
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image := deployment.Spec.Template.Spec.Containers[0].Image
		status.Version = image
	}

	if status.Healthy {
		slog.Debug("registry component healthy", "version", status.Version)
	}

	return status
}

// AllComponentsReady checks if all components are installed and healthy
func (c *ClusterComponents) AllComponentsReady() bool {
	return c.Knative.Installed && c.Knative.Healthy &&
		c.Traefik.Installed && c.Traefik.Healthy &&
		c.Registry.Installed && c.Registry.Healthy
}

// GetMissingComponents returns a list of component names that are not ready
func (c *ClusterComponents) GetMissingComponents() []string {
	missing := []string{}

	if !c.Knative.Installed || !c.Knative.Healthy {
		missing = append(missing, "knative")
	}
	if !c.Traefik.Installed || !c.Traefik.Healthy {
		missing = append(missing, "traefik")
	}
	if !c.Registry.Installed || !c.Registry.Healthy {
		missing = append(missing, "registry")
	}

	return missing
}

// GetStatusSummary returns a human-readable status summary
func (c *ClusterComponents) GetStatusSummary() string {
	if c.AllComponentsReady() {
		return "All components ready"
	}

	missing := c.GetMissingComponents()
	return fmt.Sprintf("Missing or unhealthy components: %v", missing)
}
