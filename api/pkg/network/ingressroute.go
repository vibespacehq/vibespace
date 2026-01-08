package network

import (
	"context"
	"fmt"
	"log/slog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// IngressRouteManager handles Traefik IngressRoute creation for vibespaces
type IngressRouteManager struct {
	dynamicClient dynamic.Interface
	baseDomain    string
}

// NewIngressRouteManager creates a new IngressRouteManager
func NewIngressRouteManager(dynamicClient dynamic.Interface, baseDomain string) *IngressRouteManager {
	if baseDomain == "" {
		baseDomain = "vibe.space"
	}
	return &IngressRouteManager{
		dynamicClient: dynamicClient,
		baseDomain:    baseDomain,
	}
}

// IngressRouteGVR returns the GroupVersionResource for Traefik IngressRoute
func IngressRouteGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "ingressroutes",
	}
}

// CreateVibespaceRoutes creates the IngressRoutes for a vibespace:
// 1. Main route: {projectname}.vibe.space -> Knative service (port 8080)
// 2. Wildcard route: *.{projectname}.vibe.space -> Knative service (port 8080)
//
// Both routes go to port 8080 where Caddy runs. Caddy handles:
// - Main route: serves the primary UI/application
// - Wildcard route: parses subdomain to extract port, forwards to localhost:{port}
func (m *IngressRouteManager) CreateVibespaceRoutes(ctx context.Context, projectName, serviceName, namespace string) error {
	slog.Info("creating IngressRoutes for vibespace",
		"project_name", projectName,
		"service_name", serviceName,
		"namespace", namespace,
		"base_domain", m.baseDomain)

	// Create main route: {projectname}.{basedomain}
	mainHost := fmt.Sprintf("%s.%s", projectName, m.baseDomain)
	if err := m.createIngressRoute(ctx, projectName, mainHost, serviceName, namespace, false); err != nil {
		return fmt.Errorf("failed to create main IngressRoute: %w", err)
	}

	// Create wildcard route: *.{projectname}.{basedomain}
	wildcardHost := fmt.Sprintf("*.%s.%s", projectName, m.baseDomain)
	if err := m.createIngressRoute(ctx, projectName+"-wildcard", wildcardHost, serviceName, namespace, true); err != nil {
		return fmt.Errorf("failed to create wildcard IngressRoute: %w", err)
	}

	slog.Info("IngressRoutes created successfully",
		"project_name", projectName,
		"main_host", mainHost,
		"wildcard_host", wildcardHost)

	return nil
}

// createIngressRoute creates a single Traefik IngressRoute
func (m *IngressRouteManager) createIngressRoute(ctx context.Context, name, host, serviceName, namespace string, isWildcard bool) error {
	routeName := fmt.Sprintf("vibespace-%s", name)

	slog.Debug("creating IngressRoute",
		"route_name", routeName,
		"host", host,
		"service_name", serviceName,
		"is_wildcard", isWildcard)

	// Build the IngressRoute spec
	ingressRoute := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "IngressRoute",
			"metadata": map[string]interface{}{
				"name":      routeName,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"vibespace.dev/managed": "true",
					"vibespace.dev/project": name,
				},
			},
			"spec": map[string]interface{}{
				"entryPoints": []interface{}{"websecure"},
				"routes": []interface{}{
					map[string]interface{}{
						"match": fmt.Sprintf("Host(`%s`)", host),
						"kind":  "Rule",
						"services": []interface{}{
							map[string]interface{}{
								// Knative creates a service with format: {ksvc-name}
								// The private service is: {ksvc-name}-private
								// We route to the main service which load balances
								"name": serviceName,
								"port": 80, // Knative service port
							},
						},
					},
				},
				"tls": map[string]interface{}{
					"certResolver": "le", // Let's Encrypt resolver configured in Traefik
				},
			},
		},
	}

	// Create or update the IngressRoute
	client := m.dynamicClient.Resource(IngressRouteGVR()).Namespace(namespace)

	// Check if exists
	existing, err := client.Get(ctx, routeName, metav1.GetOptions{})
	if err == nil {
		// Update existing
		ingressRoute.SetResourceVersion(existing.GetResourceVersion())
		_, err = client.Update(ctx, ingressRoute, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update IngressRoute %s: %w", routeName, err)
		}
		slog.Debug("IngressRoute updated", "route_name", routeName)
	} else {
		// Create new
		_, err = client.Create(ctx, ingressRoute, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create IngressRoute %s: %w", routeName, err)
		}
		slog.Debug("IngressRoute created", "route_name", routeName)
	}

	return nil
}

// DeleteVibespaceRoutes deletes all IngressRoutes for a vibespace
func (m *IngressRouteManager) DeleteVibespaceRoutes(ctx context.Context, projectName, namespace string) error {
	slog.Info("deleting IngressRoutes for vibespace",
		"project_name", projectName,
		"namespace", namespace)

	client := m.dynamicClient.Resource(IngressRouteGVR()).Namespace(namespace)

	// Delete main route
	mainRouteName := fmt.Sprintf("vibespace-%s", projectName)
	if err := client.Delete(ctx, mainRouteName, metav1.DeleteOptions{}); err != nil {
		slog.Warn("failed to delete main IngressRoute (may not exist)",
			"route_name", mainRouteName,
			"error", err)
	}

	// Delete wildcard route
	wildcardRouteName := fmt.Sprintf("vibespace-%s-wildcard", projectName)
	if err := client.Delete(ctx, wildcardRouteName, metav1.DeleteOptions{}); err != nil {
		slog.Warn("failed to delete wildcard IngressRoute (may not exist)",
			"route_name", wildcardRouteName,
			"error", err)
	}

	slog.Info("IngressRoutes deleted successfully",
		"project_name", projectName)

	return nil
}

// GetVibespaceURLs returns the URLs for a vibespace
func (m *IngressRouteManager) GetVibespaceURLs(projectName string) map[string]string {
	return map[string]string{
		"main": fmt.Sprintf("https://%s.%s", projectName, m.baseDomain),
	}
}

// GetServiceURL returns the URL for a specific port on a vibespace
func (m *IngressRouteManager) GetServiceURL(projectName string, port int) string {
	return fmt.Sprintf("https://%d.%s.%s", port, projectName, m.baseDomain)
}
