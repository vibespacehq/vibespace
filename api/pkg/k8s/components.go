package k8s

import (
	"context"
	"fmt"

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
	BuildKit ComponentStatus `json:"buildkit"`
}

// CheckComponents checks the status of all required cluster components
func (c *Client) CheckComponents(ctx context.Context) (*ClusterComponents, error) {
	components := &ClusterComponents{}

	// Check Knative
	components.Knative = c.checkKnative(ctx)

	// Check Traefik
	components.Traefik = c.checkTraefik(ctx)

	// Check Registry
	components.Registry = c.checkRegistry(ctx)

	// Check BuildKit
	components.BuildKit = c.checkBuildKit(ctx)

	return components, nil
}

// checkKnative checks if Knative Serving is installed and healthy
func (c *Client) checkKnative(ctx context.Context) ComponentStatus {
	status := ComponentStatus{Installed: false, Healthy: false}

	// Check if knative-serving namespace exists
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, "knative-serving", metav1.GetOptions{})
	if err != nil {
		status.Error = "knative-serving namespace not found"
		return status
	}

	status.Installed = true

	// Check if controller deployment is running
	deployment, err := c.clientset.AppsV1().Deployments("knative-serving").Get(ctx, "controller", metav1.GetOptions{})
	if err != nil {
		status.Error = "controller deployment not found"
		return status
	}

	// Check if deployment is ready
	if deployment.Status.ReadyReplicas > 0 {
		status.Healthy = true
	} else {
		status.Error = "controller deployment not ready"
	}

	// Try to get version from labels
	if version, ok := deployment.Labels["serving.knative.dev/release"]; ok {
		status.Version = version
	}

	return status
}

// checkTraefik checks if Traefik is installed and healthy
func (c *Client) checkTraefik(ctx context.Context) ComponentStatus {
	status := ComponentStatus{Installed: false, Healthy: false}

	// Check if traefik namespace exists
	_, err := c.clientset.CoreV1().Namespaces().Get(ctx, "traefik", metav1.GetOptions{})
	if err != nil {
		status.Error = "traefik namespace not found"
		return status
	}

	status.Installed = true

	// Check if traefik deployment is running
	deployment, err := c.clientset.AppsV1().Deployments("traefik").Get(ctx, "traefik", metav1.GetOptions{})
	if err != nil {
		status.Error = "traefik deployment not found"
		return status
	}

	// Check if deployment is ready
	if deployment.Status.ReadyReplicas > 0 {
		status.Healthy = true
	} else {
		status.Error = "traefik deployment not ready"
	}

	// Get version from image tag
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image := deployment.Spec.Template.Spec.Containers[0].Image
		status.Version = image
	}

	return status
}

// checkRegistry checks if the local registry is installed and healthy
func (c *Client) checkRegistry(ctx context.Context) ComponentStatus {
	status := ComponentStatus{Installed: false, Healthy: false}

	// Check if registry deployment exists in default namespace
	deployment, err := c.clientset.AppsV1().Deployments("default").Get(ctx, "registry", metav1.GetOptions{})
	if err != nil {
		status.Error = "registry deployment not found"
		return status
	}

	status.Installed = true

	// Check if deployment is ready
	if deployment.Status.ReadyReplicas > 0 {
		status.Healthy = true
	} else {
		status.Error = "registry deployment not ready"
	}

	// Get version from image tag
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image := deployment.Spec.Template.Spec.Containers[0].Image
		status.Version = image
	}

	return status
}

// checkBuildKit checks if BuildKit is installed and healthy
func (c *Client) checkBuildKit(ctx context.Context) ComponentStatus {
	status := ComponentStatus{Installed: false, Healthy: false}

	// Check if buildkitd deployment exists in default namespace
	deployment, err := c.clientset.AppsV1().Deployments("default").Get(ctx, "buildkitd", metav1.GetOptions{})
	if err != nil {
		status.Error = "buildkitd deployment not found"
		return status
	}

	status.Installed = true

	// Check if deployment is ready
	if deployment.Status.ReadyReplicas > 0 {
		status.Healthy = true
	} else {
		status.Error = "buildkitd deployment not ready"
	}

	// Get version from image tag
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		image := deployment.Spec.Template.Spec.Containers[0].Image
		status.Version = image
	}

	return status
}

// AllComponentsReady checks if all components are installed and healthy
func (c *ClusterComponents) AllComponentsReady() bool {
	return c.Knative.Installed && c.Knative.Healthy &&
		c.Traefik.Installed && c.Traefik.Healthy &&
		c.Registry.Installed && c.Registry.Healthy &&
		c.BuildKit.Installed && c.BuildKit.Healthy
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
	if !c.BuildKit.Installed || !c.BuildKit.Healthy {
		missing = append(missing, "buildkit")
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
