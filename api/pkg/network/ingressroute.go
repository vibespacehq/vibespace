package network

import (
	"context"
	"fmt"

	"vibespace/pkg/k8s"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// IngressRouteGVR is the GroupVersionResource for Traefik IngressRoutes
var IngressRouteGVR = schema.GroupVersionResource{
	Group:    "traefik.containo.us",
	Version:  "v1alpha1",
	Resource: "ingressroutes",
}

// IngressRouteManager manages Traefik IngressRoutes for vibespaces
type IngressRouteManager struct {
	k8sClient     *k8s.Client
	dynamicClient dynamic.Interface
	baseDomain    string // e.g., "vibe.space"
}

// NewIngressRouteManager creates a new IngressRoute manager
func NewIngressRouteManager(k8sClient *k8s.Client, baseDomain string) (*IngressRouteManager, error) {
	dynamicClient, err := dynamic.NewForConfig(k8sClient.Config())
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	if baseDomain == "" {
		baseDomain = "vibe.space" // Default domain
	}

	return &IngressRouteManager{
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
		baseDomain:    baseDomain,
	}, nil
}

// CreateIngressRoutesRequest contains parameters for creating IngressRoutes
type CreateIngressRoutesRequest struct {
	VibespaceID string
	ProjectName string // DNS-friendly project name
	Namespace   string
}

// CreateIngressRoutes creates 3 IngressRoutes for a vibespace:
// - code.{project}.vibe.space → vibespace-{id}:8080 (code-server)
// - preview.{project}.vibe.space → vibespace-{id}:3000 (dev server)
// - prod.{project}.vibe.space → vibespace-{id}:3001 (production server)
func (m *IngressRouteManager) CreateIngressRoutes(ctx context.Context, req *CreateIngressRoutesRequest) error {
	namespace := req.Namespace
	if namespace == "" {
		namespace = k8s.VibespaceNamespace
	}

	serviceName := fmt.Sprintf("vibespace-%s", req.VibespaceID)

	// Route configurations for the 3 subdomains
	routes := []struct {
		subdomain string
		port      int
		portName  string
	}{
		{"code", 8080, "code"},
		{"preview", 3000, "preview"},
		{"prod", 3001, "prod"},
	}

	// Create an IngressRoute for each subdomain
	for _, route := range routes {
		host := fmt.Sprintf("%s.%s.%s", route.subdomain, req.ProjectName, m.baseDomain)
		ingressRouteName := fmt.Sprintf("vibespace-%s-%s", req.VibespaceID, route.subdomain)

		ingressRoute := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "traefik.containo.us/v1alpha1",
				"kind":       "IngressRoute",
				"metadata": map[string]interface{}{
					"name":      ingressRouteName,
					"namespace": namespace,
					"labels": map[string]string{
						"app.kubernetes.io/managed-by": "vibespace",
						"vibespace.dev/id":             req.VibespaceID,
						"vibespace.dev/project-name":   req.ProjectName,
						"vibespace.dev/route-type":     route.subdomain,
					},
				},
				"spec": map[string]interface{}{
					"entryPoints": []interface{}{"web"}, // HTTP (port 80)
					"routes": []interface{}{
						map[string]interface{}{
							"match": fmt.Sprintf("Host(`%s`)", host),
							"kind":  "Rule",
							"services": []interface{}{
								map[string]interface{}{
									"name": serviceName,
									"port": route.port,
									// For Knative Services, we need to route to the actual Service
									// Knative creates a K8s Service for each revision
									"kind": "Service",
								},
							},
						},
					},
				},
			},
		}

		// Create the IngressRoute
		_, err := m.dynamicClient.Resource(IngressRouteGVR).Namespace(namespace).Create(
			ctx,
			ingressRoute,
			metav1.CreateOptions{},
		)
		if err != nil {
			return fmt.Errorf("failed to create IngressRoute for %s: %w", route.subdomain, err)
		}
	}

	return nil
}

// DeleteIngressRoutes deletes all IngressRoutes for a vibespace
func (m *IngressRouteManager) DeleteIngressRoutes(ctx context.Context, vibespaceID string, namespace string) error {
	if namespace == "" {
		namespace = k8s.VibespaceNamespace
	}

	// Delete all IngressRoutes with the vibespace ID label
	labelSelector := fmt.Sprintf("vibespace.dev/id=%s", vibespaceID)

	err := m.dynamicClient.Resource(IngressRouteGVR).Namespace(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{},
		metav1.ListOptions{
			LabelSelector: labelSelector,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to delete IngressRoutes: %w", err)
	}

	return nil
}

// ListIngressRoutes lists all IngressRoutes for vibespaces
func (m *IngressRouteManager) ListIngressRoutes(ctx context.Context, namespace string) (*unstructured.UnstructuredList, error) {
	if namespace == "" {
		namespace = k8s.VibespaceNamespace
	}

	routes, err := m.dynamicClient.Resource(IngressRouteGVR).Namespace(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/managed-by=vibespace",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list IngressRoutes: %w", err)
	}

	return routes, nil
}

// GetIngressRoute retrieves a specific IngressRoute
func (m *IngressRouteManager) GetIngressRoute(ctx context.Context, vibespaceID string, routeType string, namespace string) (*unstructured.Unstructured, error) {
	if namespace == "" {
		namespace = k8s.VibespaceNamespace
	}

	ingressRouteName := fmt.Sprintf("vibespace-%s-%s", vibespaceID, routeType)

	route, err := m.dynamicClient.Resource(IngressRouteGVR).Namespace(namespace).Get(
		ctx,
		ingressRouteName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get IngressRoute: %w", err)
	}

	return route, nil
}

// GetVibespaceURLs returns the 3 URLs for a vibespace
func (m *IngressRouteManager) GetVibespaceURLs(projectName string) map[string]string {
	return map[string]string{
		"code":    fmt.Sprintf("http://code.%s.%s", projectName, m.baseDomain),
		"preview": fmt.Sprintf("http://preview.%s.%s", projectName, m.baseDomain),
		"prod":    fmt.Sprintf("http://prod.%s.%s", projectName, m.baseDomain),
	}
}
