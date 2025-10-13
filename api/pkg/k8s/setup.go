package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// SetupProgress represents the progress of component installation
type SetupProgress struct {
	Component string `json:"component"`
	Status    string `json:"status"` // pending, installing, done, error
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

// SetupProgressFunc is a callback function for reporting progress
type SetupProgressFunc func(progress SetupProgress)

// EnsureClusterComponents ensures all required components are installed
// It's idempotent and safe to call multiple times
func (c *Client) EnsureClusterComponents(ctx context.Context, progressFn SetupProgressFunc) error {
	// Check what's already installed
	components, err := c.CheckComponents(ctx)
	if err != nil {
		return fmt.Errorf("failed to check components: %w", err)
	}

	// If everything is ready, we're done
	if components.AllComponentsReady() {
		return nil
	}

	// Install missing components
	if !components.Knative.Installed || !components.Knative.Healthy {
		if progressFn != nil {
			progressFn(SetupProgress{Component: "knative", Status: "installing", Message: "Installing Knative Serving..."})
		}
		if err := c.InstallKnative(ctx); err != nil {
			if progressFn != nil {
				progressFn(SetupProgress{Component: "knative", Status: "error", Error: err.Error()})
			}
			return fmt.Errorf("failed to install Knative: %w", err)
		}
		if progressFn != nil {
			progressFn(SetupProgress{Component: "knative", Status: "done", Message: "Knative Serving installed"})
		}
	}

	if !components.Traefik.Installed || !components.Traefik.Healthy {
		if progressFn != nil {
			progressFn(SetupProgress{Component: "traefik", Status: "installing", Message: "Installing Traefik..."})
		}
		if err := c.InstallTraefik(ctx); err != nil {
			if progressFn != nil {
				progressFn(SetupProgress{Component: "traefik", Status: "error", Error: err.Error()})
			}
			return fmt.Errorf("failed to install Traefik: %w", err)
		}
		if progressFn != nil {
			progressFn(SetupProgress{Component: "traefik", Status: "done", Message: "Traefik installed"})
		}
	}

	if !components.Registry.Installed || !components.Registry.Healthy {
		if progressFn != nil {
			progressFn(SetupProgress{Component: "registry", Status: "installing", Message: "Installing Local Registry..."})
		}
		if err := c.InstallRegistry(ctx); err != nil {
			if progressFn != nil {
				progressFn(SetupProgress{Component: "registry", Status: "error", Error: err.Error()})
			}
			return fmt.Errorf("failed to install Registry: %w", err)
		}
		if progressFn != nil {
			progressFn(SetupProgress{Component: "registry", Status: "done", Message: "Local Registry installed"})
		}
	}

	if !components.BuildKit.Installed || !components.BuildKit.Healthy {
		if progressFn != nil {
			progressFn(SetupProgress{Component: "buildkit", Status: "installing", Message: "Installing BuildKit..."})
		}
		if err := c.InstallBuildKit(ctx); err != nil {
			if progressFn != nil {
				progressFn(SetupProgress{Component: "buildkit", Status: "error", Error: err.Error()})
			}
			return fmt.Errorf("failed to install BuildKit: %w", err)
		}
		if progressFn != nil {
			progressFn(SetupProgress{Component: "buildkit", Status: "done", Message: "BuildKit installed"})
		}
	}

	// Ensure workspace namespace exists
	if err := c.EnsureNamespace(ctx); err != nil {
		return fmt.Errorf("failed to ensure workspace namespace: %w", err)
	}

	return nil
}

// InstallKnative installs Knative Serving
func (c *Client) InstallKnative(ctx context.Context) error {
	// Apply CRDs first
	if err := c.ApplyManifest(ctx, KnativeServingCRDs); err != nil {
		return fmt.Errorf("failed to apply Knative CRDs: %w", err)
	}

	// Wait a bit for CRDs to be registered
	time.Sleep(2 * time.Second)

	// Apply core components
	if err := c.ApplyManifest(ctx, KnativeServingCore); err != nil {
		return fmt.Errorf("failed to apply Knative core: %w", err)
	}

	// Wait for controller to be ready
	if err := c.waitForDeployment(ctx, "knative-serving", "controller", 5*time.Minute); err != nil {
		return fmt.Errorf("Knative controller not ready: %w", err)
	}

	return nil
}

// InstallTraefik installs Traefik Ingress Controller
func (c *Client) InstallTraefik(ctx context.Context) error {
	if err := c.ApplyManifest(ctx, TraefikManifest); err != nil {
		return fmt.Errorf("failed to apply Traefik manifest: %w", err)
	}

	// Wait for Traefik to be ready
	if err := c.waitForDeployment(ctx, "traefik", "traefik", 3*time.Minute); err != nil {
		return fmt.Errorf("Traefik not ready: %w", err)
	}

	return nil
}

// InstallRegistry installs the local Docker registry
func (c *Client) InstallRegistry(ctx context.Context) error {
	if err := c.ApplyManifest(ctx, RegistryManifest); err != nil {
		return fmt.Errorf("failed to apply Registry manifest: %w", err)
	}

	// Wait for Registry to be ready
	if err := c.waitForDeployment(ctx, "default", "registry", 3*time.Minute); err != nil {
		return fmt.Errorf("Registry not ready: %w", err)
	}

	return nil
}

// InstallBuildKit installs BuildKit
func (c *Client) InstallBuildKit(ctx context.Context) error {
	if err := c.ApplyManifest(ctx, BuildKitManifest); err != nil {
		return fmt.Errorf("failed to apply BuildKit manifest: %w", err)
	}

	// Wait for BuildKit to be ready
	if err := c.waitForDeployment(ctx, "default", "buildkitd", 3*time.Minute); err != nil {
		return fmt.Errorf("BuildKit not ready: %w", err)
	}

	return nil
}

// ApplyManifest applies a Kubernetes manifest (YAML)
func (c *Client) ApplyManifest(ctx context.Context, manifestData []byte) error {
	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(c.config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Split YAML into individual documents
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(manifestData), 4096)

	for {
		var obj unstructured.Unstructured
		if err := decoder.Decode(&obj); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode YAML: %w", err)
		}

		// Skip empty objects
		if obj.Object == nil || len(obj.Object) == 0 {
			continue
		}

		// Get GVR for the object
		gvk := obj.GroupVersionKind()
		mapper, err := c.restMapper()
		if err != nil {
			return fmt.Errorf("failed to create REST mapper: %w", err)
		}
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("failed to get REST mapping for %s: %w", gvk.String(), err)
		}

		// Get the resource interface
		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// Namespaced resource
			namespace := obj.GetNamespace()
			if namespace == "" {
				namespace = "default"
			}
			dr = dynamicClient.Resource(mapping.Resource).Namespace(namespace)
		} else {
			// Cluster-scoped resource
			dr = dynamicClient.Resource(mapping.Resource)
		}

		// Try to create the resource
		_, err = dr.Create(ctx, &obj, metav1.CreateOptions{})
		if err != nil {
			if errors.IsAlreadyExists(err) {
				// Resource already exists, get current version and update
				existing, err := dr.Get(ctx, obj.GetName(), metav1.GetOptions{})
				if err != nil {
					return fmt.Errorf("failed to get existing %s %s: %w", gvk.Kind, obj.GetName(), err)
				}

				// Preserve resourceVersion and other metadata
				obj.SetResourceVersion(existing.GetResourceVersion())
				obj.SetUID(existing.GetUID())
				obj.SetGeneration(existing.GetGeneration())
				obj.SetCreationTimestamp(existing.GetCreationTimestamp())

				// Update the resource
				_, err = dr.Update(ctx, &obj, metav1.UpdateOptions{})
				if err != nil {
					return fmt.Errorf("failed to update %s %s: %w", gvk.Kind, obj.GetName(), err)
				}
			} else {
				return fmt.Errorf("failed to create %s %s: %w", gvk.Kind, obj.GetName(), err)
			}
		}
	}

	return nil
}

// waitForDeployment waits for a deployment to be ready
func (c *Client) waitForDeployment(ctx context.Context, namespace, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment %s/%s", namespace, name)
		case <-ticker.C:
			deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// Deployment not created yet, continue waiting
					continue
				}
				return fmt.Errorf("failed to get deployment: %w", err)
			}

			if deployment.Status.ReadyReplicas > 0 {
				return nil
			}
		}
	}
}

// restMapper returns a RESTMapper for discovering GVR from GVK
func (c *Client) restMapper() (meta.RESTMapper, error) {
	// Get discovery client
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(c.config)

	// Get API group resources
	apiGroupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get API group resources: %w", err)
	}

	// Create REST mapper
	return restmapper.NewDiscoveryRESTMapper(apiGroupResources), nil
}
