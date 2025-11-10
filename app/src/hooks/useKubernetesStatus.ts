import { useState, useEffect, useCallback } from 'react';
import { invoke } from '@tauri-apps/api/core';
import { listen } from '@tauri-apps/api/event';
import type { KubernetesStatus, InstallProgress } from '../lib/types';

// Check if running in Tauri or browser
const isTauri = '__TAURI__' in window;

/**
 * Hook to monitor bundled Kubernetes status (installed, running, version).
 *
 * DEPLOYMENT MODE: This hook is for LOCAL MODE only, where bundled Kubernetes
 * (Colima/k3s) runs on the user's machine.
 *
 * With ADR 0006, vibespace bundles Kubernetes runtime for Local Mode:
 * - macOS: Colima (lightweight VM) + k3s
 * - Linux: Native k3s installation
 *
 * This hook provides status checking and lifecycle management for the bundled cluster.
 *
 * For REMOTE MODE (planned Post-MVP), a different hook will query the remote API
 * for cluster status instead of checking local binaries.
 *
 * @returns Object containing cluster status, loading state, error state, and refetch function
 *
 * @example
 * ```tsx
 * function SetupPage() {
 *   const { status, isLoading, error, refetch } = useKubernetesStatus();
 *
 *   if (isLoading) return <Spinner />;
 *   if (!status?.installed) return <InstallKubernetesButton />;
 *   if (!status?.running) return <StartKubernetesButton />;
 *
 *   return <ClusterInfo version={status.version} />;
 * }
 * ```
 *
 * @see {@link KubernetesStatus} for status object structure
 * @see ADR 0006 for bundled Kubernetes architecture
 * @public
 */
export function useKubernetesStatus() {
  const [status, setStatus] = useState<KubernetesStatus | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const getStatus = useCallback(async () => {
    setIsLoading(true);
    setError(null);

    try {
      let result: KubernetesStatus;

      if (isTauri) {
        // Native app mode - use Tauri invoke
        result = await invoke<KubernetesStatus>('get_kubernetes_status');
      } else {
        // Browser mode - mock as installed and running for development
        result = {
          installed: true,
          running: true,
          version: 'v1.27.16+k3s1 (development)',
          is_external: false,
          suggested_action: undefined,
        };
      }

      setStatus(result);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Unknown error occurred';
      setError(errorMessage);
      setStatus({
        installed: false,
        running: false,
        is_external: false,
        error: errorMessage,
        suggested_action: 'install',
      });
    } finally {
      setIsLoading(false);
    }
  }, []);

  // Initial detection on mount
  useEffect(() => {
    getStatus();
  }, [getStatus]);

  return {
    status,
    isLoading,
    error,
    refetch: getStatus,
  };
}

/**
 * Hook to install and monitor bundled Kubernetes.
 *
 * Handles the installation flow for bundled Kubernetes (Colima + k3s on macOS,
 * native k3s on Linux). Provides progress tracking via event listener.
 *
 * @returns Object with install function, installation state, and progress updates
 *
 * @example
 * ```tsx
 * function InstallButton() {
 *   const { install, isInstalling, progress } = useKubernetesInstall();
 *
 *   return (
 *     <>
 *       <button onClick={install} disabled={isInstalling}>
 *         {isInstalling ? 'Installing...' : 'Install Kubernetes'}
 *       </button>
 *       {isInstalling && <Progress value={progress?.progress} message={progress?.message} />}
 *     </>
 *   );
 * }
 * ```
 *
 * @public
 */
export function useKubernetesInstall() {
  const [isInstalling, setIsInstalling] = useState(false);
  const [progress, setProgress] = useState<InstallProgress | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [installComplete, setInstallComplete] = useState(false);

  useEffect(() => {
    if (!isTauri) return;

    // Listen for install progress events
    const unlisten = listen<InstallProgress>('install-progress', (event) => {
      const progress = event.payload;
      setProgress(progress);

      // Handle completion or error
      if (progress.stage === 'complete') {
        setIsInstalling(false);
        setInstallComplete(true);
      } else if (progress.stage === 'error') {
        setError(progress.message);
        setIsInstalling(false);
      }
    });

    return () => {
      unlisten.then((fn) => fn());
    };
  }, []);

  const install = useCallback(async () => {
    setIsInstalling(true);
    setError(null);
    setProgress(null);

    if (isTauri) {
      // Installation runs in background, progress tracked via events
      // Command returns immediately
      await invoke('install_kubernetes');
    } else {
      // Browser mode - simulate installation
      await new Promise((resolve) => setTimeout(resolve, 2000));
      setIsInstalling(false);
    }
  }, []);

  return {
    install,
    isInstalling,
    progress,
    error,
    installComplete,
  };
}

/**
 * Hook to start/stop bundled Kubernetes cluster.
 *
 * Provides lifecycle management functions for the bundled cluster.
 * Start/stop operations are idempotent (safe to call multiple times).
 *
 * @returns Object with start and stop functions, loading states
 *
 * @example
 * ```tsx
 * function ControlPanel() {
 *   const { start, stop, isStarting, isStopping } = useKubernetesControl();
 *
 *   return (
 *     <>
 *       <button onClick={start} disabled={isStarting}>Start</button>
 *       <button onClick={stop} disabled={isStopping}>Stop</button>
 *     </>
 *   );
 * }
 * ```
 *
 * @public
 */
export function useKubernetesControl() {
  const [isStarting, setIsStarting] = useState(false);
  const [isStopping, setIsStopping] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const start = useCallback(async () => {
    setIsStarting(true);
    setError(null);

    try {
      if (isTauri) {
        await invoke('start_kubernetes');
      } else {
        // Browser mode - simulate start
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to start';
      setError(errorMessage);
      throw err;
    } finally {
      setIsStarting(false);
    }
  }, []);

  const stop = useCallback(async () => {
    setIsStopping(true);
    setError(null);

    try {
      if (isTauri) {
        await invoke('stop_kubernetes');
      } else {
        // Browser mode - simulate stop
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to stop';
      setError(errorMessage);
      throw err;
    } finally {
      setIsStopping(false);
    }
  }, []);

  const uninstall = useCallback(async () => {
    try {
      if (isTauri) {
        await invoke('uninstall_kubernetes');
      } else {
        // Browser mode - simulate uninstall
        await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : 'Failed to uninstall';
      setError(errorMessage);
      throw err;
    }
  }, []);

  return {
    start,
    stop,
    uninstall,
    isStarting,
    isStopping,
    error,
  };
}

/**
 * Get the current operating system type.
 * Used to show platform-specific installation instructions or warnings.
 *
 * @public
 * @returns Promise resolving to 'macos', 'linux', or 'windows'
 * @example
 * ```ts
 * const os = await getOSType();
 * if (os === 'windows') {
 *   console.log('Windows not supported, use WSL2');
 * }
 * ```
 */
export async function getOSType(): Promise<string> {
  if (!isTauri) return 'macos'; // Mock in browser mode
  return await invoke<string>('get_os_type');
}
