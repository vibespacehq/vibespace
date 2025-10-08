import { useState, useEffect, useCallback } from 'react';
import { invoke } from '@tauri-apps/api/core';
import type { KubernetesStatus } from '../lib/types';

export function useKubernetesStatus() {
  const [status, setStatus] = useState<KubernetesStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const detectKubernetes = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      const result = await invoke<KubernetesStatus>('detect_kubernetes');
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
// TODO(Phase 2): Will be used by workspace creation wizard and health monitoring

export async function checkKubectl(): Promise<boolean> {
  return await invoke<boolean>('check_kubectl');
}

export async function findKubeconfig(): Promise<string | null> {
  return await invoke<string | null>('find_kubeconfig');
}

export async function checkClusterHealth(kubeconfigPath?: string): Promise<boolean> {
  return await invoke<boolean>('check_cluster_health', { kubeconfigPath });
}

export async function detectInstallType(kubeconfigPath?: string): Promise<string> {
  return await invoke<string>('detect_install_type', { kubeconfigPath });
}

export async function getClusterVersion(kubeconfigPath?: string): Promise<string | null> {
  return await invoke<string | null>('get_cluster_version', { kubeconfigPath });
}
