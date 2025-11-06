import { useState, useEffect, useCallback } from 'react';
import { invoke } from '@tauri-apps/api/core';
import type { KubernetesStatus } from '../lib/types';

// Check if running in Tauri or browser
const isTauri = '__TAURI__' in window;

/**
 * Hook to detect and monitor Kubernetes cluster availability.
 *
 * Detects if Kubernetes (kubectl, k3s, Rancher Desktop, etc.) is available
 * on the system. In Tauri mode, uses native OS calls. In browser mode,
 * mocks availability for development.
 *
 * @returns Object containing cluster status, loading state, error state, and refetch function
 *
 * @example
 * ```tsx
 * function SetupPage() {
 *   const { status, isLoading, error, refetch } = useKubernetesStatus();
 *
 *   if (isLoading) return <Spinner />;
 *   if (!status?.available) return <InstallInstructions />;
 *
 *   return <ClusterInfo version={status.version} type={status.installType} />;
 * }
 * ```
 *
 * @see {@link KubernetesStatus} for status object structure
 * @public
 */
export function useKubernetesStatus() {
  const [status, setStatus] = useState<KubernetesStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const detectKubernetes = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      let result: KubernetesStatus;

      if (isTauri) {
        // Native app mode - use Tauri invoke
        result = await invoke<KubernetesStatus>('detect_kubernetes');
      } else {
        // Browser mode - mock as available for development
        result = {
          available: true,
          installType: 'k3d',
          version: 'development',
          suggestedAction: undefined,
        };
      }

      setStatus(result);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Unknown error occurred';
      setError(errorMessage);
      setStatus({
        available: false,
        error: errorMessage,
        suggestedAction: 'check_installation',
      });
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Initial detection on mount
  useEffect(() => {
    detectKubernetes();
  }, [detectKubernetes]);

  return {
    status,
    isLoading,
    error,
    refetch: detectKubernetes,
  };
}

// Individual detection functions (for advanced use cases)
// These are exported for future features that need granular control
// TODO(Phase 2): Will be used by vibespace creation wizard and health monitoring

/**
 * Checks if kubectl is available in the system PATH.
 *
 * @public
 * @returns Promise resolving to true if kubectl is found, false otherwise
 * @example
 * ```ts
 * const hasKubectl = await checkKubectl();
 * if (!hasKubectl) {
 *   console.log('kubectl not found, please install Kubernetes');
 * }
 * ```
 */
export async function checkKubectl(): Promise<boolean> {
  if (!isTauri) return true; // Mock in browser mode
  return await invoke<boolean>('check_kubectl');
}

/**
 * Attempts to locate a valid kubeconfig file in standard locations.
 * Checks ~/.kube/config, /etc/rancher/k3s/k3s.yaml, and $KUBECONFIG env var.
 *
 * @public
 * @returns Promise resolving to the kubeconfig file path, or null if not found
 * @example
 * ```ts
 * const configPath = await findKubeconfig();
 * if (configPath) {
 *   console.log(`Found kubeconfig at: ${configPath}`);
 * }
 * ```
 */
export async function findKubeconfig(): Promise<string | null> {
  if (!isTauri) return '~/.kube/config'; // Mock in browser mode
  return await invoke<string | null>('find_kubeconfig');
}

/**
 * Verifies that the Kubernetes cluster is reachable and healthy.
 * Performs connectivity check and basic health validation.
 *
 * @public
 * @param kubeconfigPath - Optional path to kubeconfig file. Uses default if not provided.
 * @returns Promise resolving to true if cluster is healthy and reachable
 * @example
 * ```ts
 * const isHealthy = await checkClusterHealth();
 * if (!isHealthy) {
 *   console.log('Cluster is unreachable or not running');
 * }
 * ```
 */
export async function checkClusterHealth(kubeconfigPath?: string): Promise<boolean> {
  if (!isTauri) return true; // Mock in browser mode
  return await invoke<boolean>('check_cluster_health', { kubeconfigPath });
}

/**
 * Detects the type of Kubernetes installation present on the system.
 * Can identify k3s, Rancher Desktop, k3d, minikube, Docker Desktop, or unknown.
 *
 * @public
 * @param kubeconfigPath - Optional path to kubeconfig file. Uses default if not provided.
 * @returns Promise resolving to the installation type as a string
 * @example
 * ```ts
 * const installType = await detectInstallType();
 * console.log(`Detected: ${installType}`); // e.g., "rancher-desktop"
 * ```
 */
export async function detectInstallType(kubeconfigPath?: string): Promise<string> {
  if (!isTauri) return 'k3d'; // Mock in browser mode
  return await invoke<string>('detect_install_type', { kubeconfigPath });
}

/**
 * Retrieves the version of the running Kubernetes cluster.
 *
 * @public
 * @param kubeconfigPath - Optional path to kubeconfig file. Uses default if not provided.
 * @returns Promise resolving to the cluster version string, or null if unable to determine
 * @example
 * ```ts
 * const version = await getClusterVersion();
 * console.log(`Cluster version: ${version}`); // e.g., "v1.27.3+k3s1"
 * ```
 */
export async function getClusterVersion(kubeconfigPath?: string): Promise<string | null> {
  if (!isTauri) return 'v1.27.3+k3s1'; // Mock in browser mode
  return await invoke<string | null>('get_cluster_version', { kubeconfigPath });
}
