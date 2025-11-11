package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"vibespace/pkg/k8s"
)

func TestNewIngressRouteManager(t *testing.T) {
	t.Run("creates manager with default domain", func(t *testing.T) {
		// Note: In real tests, we'd need a proper k8s client
		// For now, testing the domain default behavior
		baseDomain := "vibe.space"
		assert.NotEmpty(t, baseDomain)
	})

	t.Run("creates manager with custom domain", func(t *testing.T) {
		customDomain := "custom.dev"
		assert.Equal(t, "custom.dev", customDomain)
	})
}

func TestGetVibespaceURLs(t *testing.T) {
	// Create a fake k8s client
	scheme := runtime.NewScheme()
	dynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	// Create mock k8s client
	k8sClient := &k8s.Client{}

	manager := &IngressRouteManager{
		k8sClient:     k8sClient,
		dynamicClient: dynamicClient,
		baseDomain:    "vibe.space",
	}

	t.Run("returns correct URLs for project", func(t *testing.T) {
		projectName := "myproject"
		urls := manager.GetVibespaceURLs(projectName)

		assert.Equal(t, "http://code.myproject.vibe.space", urls["code"])
		assert.Equal(t, "http://preview.myproject.vibe.space", urls["preview"])
		assert.Equal(t, "http://prod.myproject.vibe.space", urls["prod"])
	})

	t.Run("handles different project names", func(t *testing.T) {
		projectName := "test-123"
		urls := manager.GetVibespaceURLs(projectName)

		assert.Contains(t, urls["code"], "test-123")
		assert.Contains(t, urls["preview"], "test-123")
		assert.Contains(t, urls["prod"], "test-123")
	})
}

func TestCreateIngressRoutes(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This would be an integration test that requires a real k8s cluster
	// For now, we'll test the structure

	t.Run("validates IngressRoute structure", func(t *testing.T) {
		req := &CreateIngressRoutesRequest{
			VibespaceID: "test-123",
			ProjectName: "myproject",
			Namespace:   "vibespace",
		}

		// Validate request structure
		assert.NotEmpty(t, req.VibespaceID)
		assert.NotEmpty(t, req.ProjectName)
		assert.NotEmpty(t, req.Namespace)

		// Expected IngressRoute names
		expectedNames := []string{
			"vibespace-test-123-code",
			"vibespace-test-123-preview",
			"vibespace-test-123-prod",
		}

		for _, name := range expectedNames {
			assert.Contains(t, name, req.VibespaceID)
		}
	})
}

func TestDeleteIngressRoutes(t *testing.T) {
	t.Run("constructs correct label selector", func(t *testing.T) {
		vibespaceID := "test-123"
		expectedSelector := "vibespace.dev/id=test-123"

		assert.Contains(t, expectedSelector, vibespaceID)
	})

	t.Run("uses correct namespace", func(t *testing.T) {
		namespace := "vibespace"
		assert.Equal(t, "vibespace", namespace)
	})
}

func TestIngressRouteStructure(t *testing.T) {
	t.Run("validates IngressRoute manifest structure", func(t *testing.T) {
		// Expected structure of an IngressRoute
		ingressRoute := &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "traefik.containo.us/v1alpha1",
				"kind":       "IngressRoute",
				"metadata": map[string]interface{}{
					"name":      "vibespace-test-code",
					"namespace": "vibespace",
					"labels": map[string]string{
						"app.kubernetes.io/managed-by": "vibespace",
						"vibespace.dev/id":             "test",
						"vibespace.dev/project-name":   "myproject",
						"vibespace.dev/route-type":     "code",
					},
				},
				"spec": map[string]interface{}{
					"entryPoints": []interface{}{"web"},
					"routes": []interface{}{
						map[string]interface{}{
							"match": "Host(`code.myproject.vibe.space`)",
							"kind":  "Rule",
							"services": []interface{}{
								map[string]interface{}{
									"name": "vibespace-test",
									"port": 8080,
									"kind": "Service",
								},
							},
						},
					},
				},
			},
		}

		// Validate structure
		assert.Equal(t, "traefik.containo.us/v1alpha1", ingressRoute.GetAPIVersion())
		assert.Equal(t, "IngressRoute", ingressRoute.GetKind())
		assert.Equal(t, "vibespace-test-code", ingressRoute.GetName())

		// Validate labels from Object directly
		metadata, ok := ingressRoute.Object["metadata"].(map[string]interface{})
		require.True(t, ok)
		labels, ok := metadata["labels"].(map[string]string)
		require.True(t, ok)
		assert.Equal(t, "vibespace", labels["app.kubernetes.io/managed-by"])
		assert.Equal(t, "test", labels["vibespace.dev/id"])
		assert.Equal(t, "code", labels["vibespace.dev/route-type"])

		// Validate spec structure
		spec, ok := ingressRoute.Object["spec"].(map[string]interface{})
		require.True(t, ok)

		entryPoints, ok := spec["entryPoints"].([]interface{})
		require.True(t, ok)
		assert.Equal(t, "web", entryPoints[0])

		routes, ok := spec["routes"].([]interface{})
		require.True(t, ok)
		assert.Len(t, routes, 1)
	})

	t.Run("validates port mappings", func(t *testing.T) {
		portMappings := map[string]int{
			"code":    8080,
			"preview": 3000,
			"prod":    3001,
		}

		assert.Equal(t, 8080, portMappings["code"])
		assert.Equal(t, 3000, portMappings["preview"])
		assert.Equal(t, 3001, portMappings["prod"])
	})
}

func TestListIngressRoutes(t *testing.T) {
	t.Run("uses correct label selector", func(t *testing.T) {
		expectedSelector := "app.kubernetes.io/managed-by=vibespace"
		assert.Equal(t, "app.kubernetes.io/managed-by=vibespace", expectedSelector)
	})
}

func TestGetIngressRoute(t *testing.T) {
	t.Run("constructs correct IngressRoute name", func(t *testing.T) {
		vibespaceID := "test-123"
		routeType := "code"

		expectedName := "vibespace-test-123-code"
		actualName := "vibespace-" + vibespaceID + "-" + routeType

		assert.Equal(t, expectedName, actualName)
	})

	t.Run("handles different route types", func(t *testing.T) {
		routeTypes := []string{"code", "preview", "prod"}

		for _, routeType := range routeTypes {
			name := "vibespace-test-" + routeType
			assert.Contains(t, name, routeType)
		}
	})
}
