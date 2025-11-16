package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"vibespace/pkg/template"

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
	slog.Info("starting cluster component setup")

	// Check what's already installed
	components, err := c.CheckComponents(ctx)
	if err != nil {
		slog.Error("failed to check cluster components",
			"error", err)
		return fmt.Errorf("failed to check components: %w", err)
	}

	slog.Info("cluster component status checked",
		"knative_healthy", components.Knative.Healthy,
		"traefik_healthy", components.Traefik.Healthy,
		"registry_healthy", components.Registry.Healthy,
		"buildkit_healthy", components.BuildKit.Healthy)

	// Install missing components (skip if already installed)
	if !components.Knative.Installed || !components.Knative.Healthy {
		slog.Info("knative not ready, starting installation",
			"installed", components.Knative.Installed,
			"healthy", components.Knative.Healthy)
		if progressFn != nil {
			progressFn(SetupProgress{Component: "knative", Status: "installing", Message: "Installing Knative Serving..."})
		}
		if err := c.InstallKnative(ctx); err != nil {
			slog.Error("failed to install knative",
				"error", err)
			if progressFn != nil {
				progressFn(SetupProgress{Component: "knative", Status: "error", Error: err.Error()})
			}
			return fmt.Errorf("failed to install Knative: %w", err)
		}
		slog.Info("knative installed successfully")
		if progressFn != nil {
			progressFn(SetupProgress{Component: "knative", Status: "done", Message: "Knative Serving installed"})
		}
	} else {
		slog.Info("knative already installed and healthy, skipping")
	}

	if !components.Traefik.Installed || !components.Traefik.Healthy {
		slog.Info("traefik not ready, starting installation",
			"installed", components.Traefik.Installed,
			"healthy", components.Traefik.Healthy)
		if progressFn != nil {
			progressFn(SetupProgress{Component: "traefik", Status: "installing", Message: "Installing Traefik..."})
		}
		if err := c.InstallTraefik(ctx); err != nil {
			slog.Error("failed to install traefik",
				"error", err)
			if progressFn != nil {
				progressFn(SetupProgress{Component: "traefik", Status: "error", Error: err.Error()})
			}
			return fmt.Errorf("failed to install Traefik: %w", err)
		}
		slog.Info("traefik installed successfully")
		if progressFn != nil {
			progressFn(SetupProgress{Component: "traefik", Status: "done", Message: "Traefik installed"})
		}
	} else {
		slog.Info("traefik already installed and healthy, skipping")
	}

	if !components.Registry.Installed || !components.Registry.Healthy {
		slog.Info("registry not ready, starting installation",
			"installed", components.Registry.Installed,
			"healthy", components.Registry.Healthy)
		if progressFn != nil {
			progressFn(SetupProgress{Component: "registry", Status: "installing", Message: "Installing Local Registry..."})
		}
		if err := c.InstallRegistry(ctx); err != nil {
			slog.Error("failed to install registry",
				"error", err)
			if progressFn != nil {
				progressFn(SetupProgress{Component: "registry", Status: "error", Error: err.Error()})
			}
			return fmt.Errorf("failed to install Registry: %w", err)
		}
		slog.Info("registry installed successfully")
		if progressFn != nil {
			progressFn(SetupProgress{Component: "registry", Status: "done", Message: "Local Registry installed"})
		}
	} else {
		slog.Info("registry already installed and healthy, skipping")
	}

	if !components.BuildKit.Installed || !components.BuildKit.Healthy {
		slog.Info("buildkit not ready, starting installation",
			"installed", components.BuildKit.Installed,
			"healthy", components.BuildKit.Healthy)
		if progressFn != nil {
			progressFn(SetupProgress{Component: "buildkit", Status: "installing", Message: "Installing BuildKit..."})
		}
		if err := c.InstallBuildKit(ctx); err != nil {
			slog.Error("failed to install buildkit",
				"error", err)
			if progressFn != nil {
				progressFn(SetupProgress{Component: "buildkit", Status: "error", Error: err.Error()})
			}
			return fmt.Errorf("failed to install BuildKit: %w", err)
		}
		slog.Info("buildkit installed successfully")
		if progressFn != nil {
			progressFn(SetupProgress{Component: "buildkit", Status: "done", Message: "BuildKit installed"})
		}
	} else {
		slog.Info("buildkit already installed and healthy, skipping")
	}

	// Build template images after BuildKit is ready
	// BuildTemplateImages sends its own progress updates for each image
	slog.Info("starting template image builds")
	if err := c.BuildTemplateImages(ctx, progressFn); err != nil {
		slog.Error("failed to build template images",
			"error", err)
		return fmt.Errorf("failed to build template images: %w", err)
	}
	slog.Info("all template images built successfully")

	// Ensure vibespace namespace exists
	slog.Info("ensuring vibespace namespace exists")
	if err := c.EnsureNamespace(ctx); err != nil {
		slog.Error("failed to ensure vibespace namespace",
			"error", err)
		return fmt.Errorf("failed to ensure vibespace namespace: %w", err)
	}
	slog.Info("vibespace namespace ready")

	slog.Info("cluster component setup completed successfully")
	return nil
}

// InstallKnative installs Knative Serving
func (c *Client) InstallKnative(ctx context.Context) error {
	slog.Info("installing knative serving")

	// Apply CRDs first
	slog.Info("applying knative CRDs")
	if err := c.ApplyManifest(ctx, KnativeServingCRDs); err != nil {
		slog.Error("failed to apply knative CRDs",
			"error", err)
		return fmt.Errorf("failed to apply Knative CRDs: %w", err)
	}
	slog.Info("knative CRDs applied successfully")

	// Wait a bit for CRDs to be registered
	slog.Info("waiting for CRDs to be registered")
	time.Sleep(2 * time.Second)

	// Apply core components
	slog.Info("applying knative core components")
	if err := c.ApplyManifest(ctx, KnativeServingCore); err != nil {
		slog.Error("failed to apply knative core",
			"error", err)
		return fmt.Errorf("failed to apply Knative core: %w", err)
	}
	slog.Info("knative core components applied successfully")

	// Wait for controller to be ready
	slog.Info("waiting for knative controller to be ready",
		"timeout", "5m")
	if err := c.waitForDeployment(ctx, "knative-serving", "controller", 5*time.Minute); err != nil {
		slog.Error("knative controller not ready",
			"error", err)
		return fmt.Errorf("knative controller not ready: %w", err)
	}
	slog.Info("knative controller is ready")

	// Wait for webhook to be ready (required for ConfigMap validation)
	slog.Info("waiting for knative webhook to be ready",
		"timeout", "5m")
	if err := c.waitForDeployment(ctx, "knative-serving", "webhook", 5*time.Minute); err != nil {
		slog.Error("knative webhook not ready",
			"error", err)
		return fmt.Errorf("knative webhook not ready: %w", err)
	}
	slog.Info("knative webhook is ready")

	// Configure Knative features for vibespace requirements
	slog.Info("configuring knative features")
	if err := c.ConfigureKnativeFeatures(ctx); err != nil {
		slog.Error("failed to configure knative features",
			"error", err)
		return fmt.Errorf("failed to configure Knative features: %w", err)
	}
	slog.Info("knative features configured successfully")

	// Configure Knative defaults (revision timeout, etc.)
	slog.Info("configuring knative defaults")
	if err := c.ConfigureKnativeDefaults(ctx); err != nil {
		slog.Error("failed to configure knative defaults",
			"error", err)
		return fmt.Errorf("failed to configure Knative defaults: %w", err)
	}
	slog.Info("knative defaults configured successfully")

	return nil
}

// ConfigureKnativeFeatures enables required Knative features for vibespace
func (c *Client) ConfigureKnativeFeatures(ctx context.Context) error {
	// Patch config-features ConfigMap to enable:
	// - kubernetes.podspec-persistent-volume-claim: for PVC support
	// - kubernetes.podspec-persistent-volume-write: for writable PVCs
	// - kubernetes.podspec-init-containers: for init containers (git-clone, fix-permissions)
	// - multi-container: for multiple ports per vibespace
	slog.Info("enabling knative features for vibespace")

	configMapClient := c.clientset.CoreV1().ConfigMaps("knative-serving")

	// Get current config-features ConfigMap
	cm, err := configMapClient.Get(ctx, "config-features", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get config-features ConfigMap: %w", err)
	}

	// Update data fields
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	cm.Data["kubernetes.podspec-persistent-volume-claim"] = "enabled"
	cm.Data["kubernetes.podspec-persistent-volume-write"] = "enabled"
	cm.Data["kubernetes.podspec-init-containers"] = "enabled"
	cm.Data["multi-container"] = "enabled"

	// Update ConfigMap
	_, err = configMapClient.Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update config-features ConfigMap: %w", err)
	}

	slog.Info("knative features enabled",
		"pvc_support", "enabled",
		"pvc_write", "enabled",
		"init_containers", "enabled",
		"multi_container", "enabled")

	// Restart Knative controller to pick up new configuration
	slog.Info("restarting knative controller to apply new features")
	deploymentClient := c.clientset.AppsV1().Deployments("knative-serving")

	// Get current controller deployment
	controller, err := deploymentClient.Get(ctx, "controller", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get controller deployment: %w", err)
	}

	// Trigger rolling restart by updating an annotation
	if controller.Spec.Template.Annotations == nil {
		controller.Spec.Template.Annotations = make(map[string]string)
	}
	controller.Spec.Template.Annotations["vibespace.dev/config-reloaded"] = time.Now().Format(time.RFC3339)

	_, err = deploymentClient.Update(ctx, controller, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to restart controller: %w", err)
	}

	// Wait for controller to be ready again after restart
	slog.Info("waiting for controller to restart with new config")
	time.Sleep(5 * time.Second) // Give it a moment to start rolling out
	if err := c.waitForDeployment(ctx, "knative-serving", "controller", 2*time.Minute); err != nil {
		return fmt.Errorf("controller not ready after config update: %w", err)
	}

	slog.Info("knative controller restarted with new features")
	return nil
}

// ConfigureKnativeDefaults configures Knative default values
// Sets revision timeout to 600 seconds (Knative max limit)
func (c *Client) ConfigureKnativeDefaults(ctx context.Context) error {
	slog.Info("configuring knative defaults")

	// Get config-defaults ConfigMap
	cm, err := c.clientset.CoreV1().ConfigMaps("knative-serving").Get(ctx, "config-defaults", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get config-defaults: %w", err)
	}

	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	// Set revision timeout to 600 seconds (10 minutes, Knative maximum)
	// Default is 300s, we increase to 600s for longer coding sessions
	cm.Data["revision-timeout-seconds"] = "600"

	// Update ConfigMap
	_, err = c.clientset.CoreV1().ConfigMaps("knative-serving").Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update config-defaults: %w", err)
	}

	slog.Info("knative defaults configured",
		"revision-timeout-seconds", "600")

	// Wait for controller to pick up new config (no restart needed for config-defaults)
	time.Sleep(5 * time.Second)

	return nil
}

// InstallTraefik installs Traefik Ingress Controller
func (c *Client) InstallTraefik(ctx context.Context) error {
	slog.Info("installing traefik")

	// Apply CRDs first (IngressRoute, IngressRouteTCP, IngressRouteUDP, etc.)
	slog.Info("applying traefik CRDs")
	if err := c.ApplyManifest(ctx, TraefikCRDs); err != nil {
		slog.Error("failed to apply traefik CRDs",
			"error", err)
		return fmt.Errorf("failed to apply Traefik CRDs: %w", err)
	}
	slog.Info("traefik CRDs applied successfully")

	// Wait a bit for CRDs to be registered
	slog.Info("waiting for CRDs to be registered")
	time.Sleep(2 * time.Second)

	// Apply core components
	slog.Info("applying traefik manifest")
	if err := c.ApplyManifest(ctx, TraefikManifest); err != nil {
		slog.Error("failed to apply traefik manifest",
			"error", err)
		return fmt.Errorf("failed to apply Traefik manifest: %w", err)
	}
	slog.Info("traefik manifest applied successfully")

	// Wait for Traefik to be ready
	slog.Info("waiting for traefik to be ready",
		"timeout", "3m")
	if err := c.waitForDeployment(ctx, "traefik", "traefik", 3*time.Minute); err != nil {
		slog.Error("traefik not ready",
			"error", err)
		return fmt.Errorf("traefik not ready: %w", err)
	}
	slog.Info("traefik is ready")

	return nil
}

// InstallRegistry installs the local Docker registry
func (c *Client) InstallRegistry(ctx context.Context) error {
	slog.Info("installing local registry")

	slog.Info("applying registry manifest")
	if err := c.ApplyManifest(ctx, RegistryManifest); err != nil {
		slog.Error("failed to apply registry manifest",
			"error", err)
		return fmt.Errorf("failed to apply Registry manifest: %w", err)
	}
	slog.Info("registry manifest applied successfully")

	// Wait for Registry to be ready
	slog.Info("waiting for registry to be ready",
		"timeout", "3m")
	if err := c.waitForDeployment(ctx, "default", "registry", 3*time.Minute); err != nil {
		slog.Error("registry not ready",
			"error", err)
		return fmt.Errorf("registry not ready: %w", err)
	}
	slog.Info("registry is ready")

	return nil
}

// InstallBuildKit installs BuildKit
func (c *Client) InstallBuildKit(ctx context.Context) error {
	slog.Info("installing buildkit")

	slog.Info("applying buildkit manifest")
	if err := c.ApplyManifest(ctx, BuildKitManifest); err != nil {
		slog.Error("failed to apply buildkit manifest",
			"error", err)
		return fmt.Errorf("failed to apply BuildKit manifest: %w", err)
	}
	slog.Info("buildkit manifest applied successfully")

	// Wait for BuildKit to be ready
	slog.Info("waiting for buildkit to be ready",
		"timeout", "3m")
	if err := c.waitForDeployment(ctx, "default", "buildkitd", 3*time.Minute); err != nil {
		slog.Error("buildkit not ready",
			"error", err)
		return fmt.Errorf("buildKit not ready: %w", err)
	}
	slog.Info("buildkit is ready")

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
		if len(obj.Object) == 0 {
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
	slog.Debug("waiting for deployment",
		"namespace", namespace,
		"deployment", name,
		"timeout", timeout.String())

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Error("timeout waiting for deployment",
				"namespace", namespace,
				"deployment", name,
				"timeout", timeout.String())
			return fmt.Errorf("timeout waiting for deployment %s/%s", namespace, name)
		case <-ticker.C:
			deployment, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					// Deployment not created yet, continue waiting
					slog.Debug("deployment not found yet, waiting",
						"namespace", namespace,
						"deployment", name)
					continue
				}
				slog.Error("failed to get deployment status",
					"namespace", namespace,
					"deployment", name,
					"error", err)
				return fmt.Errorf("failed to get deployment: %w", err)
			}

			slog.Debug("checking deployment status",
				"namespace", namespace,
				"deployment", name,
				"ready_replicas", deployment.Status.ReadyReplicas,
				"desired_replicas", deployment.Status.Replicas)

			if deployment.Status.ReadyReplicas > 0 {
				slog.Info("deployment is ready",
					"namespace", namespace,
					"deployment", name,
					"ready_replicas", deployment.Status.ReadyReplicas)
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

// BuildTemplateImages builds all template images using BuildKit
// This is called after BuildKit is installed and ready
func (c *Client) BuildTemplateImages(ctx context.Context, progressFn SetupProgressFunc) error {
	// Create builder instance with k8s client for port-forwarding
	// BuildKit runs inside the cluster, so use cluster-internal registry address
	builder := template.NewBuilder("registry.default.svc.cluster.local:5000", c)

	// Start port-forwards to BuildKit and Registry
	if err := builder.EnsurePortForwards(ctx); err != nil {
		return fmt.Errorf("failed to setup port-forwards: %w", err)
	}
	defer builder.StopPortForwards()

	// Convert template.BuildProgress to SetupProgress
	buildProgressFn := func(progress template.BuildProgress) {
		if progressFn != nil {
			progressFn(SetupProgress{
				Component: progress.Template,
				Status:    progress.Status,
				Message:   progress.Message,
				Error:     progress.Error,
			})
		}
	}

	// Build all base images and template images
	// This will build 12 images total:
	// - 3 base images (claude, codex, gemini)
	// - 9 template images (nextjs×3, vue×3, jupyter×3)
	if err := builder.BuildAllTemplates(ctx, buildProgressFn); err != nil {
		return fmt.Errorf("failed to build template images: %w", err)
	}

	return nil
}
