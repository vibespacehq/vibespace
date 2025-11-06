import { useState, useEffect, useCallback } from 'react';
import { API_ENDPOINTS, apiFetch } from '../lib/api-config';
import type { Vibespace, CreateVibespaceRequest } from '../lib/types';

interface UseVibespacesReturn {
  vibespaces: Vibespace[];
  isLoading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
  createVibespace: (request: CreateVibespaceRequest) => Promise<Vibespace>;
  deleteVibespace: (id: string) => Promise<void>;
  startVibespace: (id: string) => Promise<void>;
  stopVibespace: (id: string) => Promise<void>;
  accessVibespace: (id: string) => Promise<Record<string, string>>;
}

/**
 * React hook for managing vibespace CRUD operations.
 * Provides methods to list, create, delete, start, and stop vibespaces.
 *
 * @returns Object containing vibespace data, loading state, error state, and operation methods
 *
 * @example
 * ```tsx
 * function WorkspaceList() {
 *   const { vibespaces, isLoading, error, createVibespace, deleteVibespace } = useWorkspaces();
 *
 *   const handleCreate = async () => {
 *     await createWorkspace({
 *       name: 'my-vibespace',
 *       template: 'nextjs',
 *       persistent: true
 *     });
 *   };
 *
 *   return (
 *     <div>
 *       {vibespaces.map(ws => <VibespaceCard key={ws.id} vibespace={ws} />)}
 *     </div>
 *   );
 * }
 * ```
 */
export function useVibespaces(): UseVibespacesReturn {
  const [vibespaces, setVibespaces] = useState<Vibespace[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isInitialLoad, setIsInitialLoad] = useState(true);

  /**
   * Fetches the list of all vibespaces from the API.
   * @param showLoading - Whether to show loading state (true for initial load, false for polling)
   * @throws {Error} If API request fails
   */
  const fetchWorkspaces = useCallback(async (showLoading = false) => {
    try {
      if (showLoading) {
        setIsLoading(true);
      }
      setError(null);

      const data = await apiFetch<{ vibespaces: Vibespace[] }>(
        `${API_ENDPOINTS.vibespaces}`
      );

      setVibespaces(data.vibespaces || []);

      if (isInitialLoad) {
        setIsInitialLoad(false);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      console.error('Failed to fetch vibespaces:', err);
      setError(`Failed to load vibespaces: ${message}`);
    } finally {
      if (showLoading) {
        setIsLoading(false);
      }
    }
  }, [isInitialLoad]);

  /**
   * Creates a new vibespace with the specified configuration.
   * @param request - Workspace creation parameters
   * @returns The created vibespace object
   * @throws {Error} If vibespace creation fails
   */
  const createVibespace = useCallback(
    async (request: CreateVibespaceRequest): Promise<Vibespace> => {
      try {
        const vibespace = await apiFetch<Vibespace>(
          `${API_ENDPOINTS.vibespaces}`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(request),
          }
        );

        // Refresh vibespace list after creation
        await fetchWorkspaces();

        return vibespace;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to create vibespace:', err);
        throw new Error(`Failed to create vibespace: ${message}`);
      }
    },
    [fetchWorkspaces]
  );

  /**
   * Updates a vibespace's status optimistically (before API response).
   * @param id - Workspace ID
   * @param status - New status to set
   */
  const updateWorkspaceStatus = useCallback((id: string, status: Vibespace['status']) => {
    console.log(`[useWorkspaces] Optimistically updating vibespace ${id} to status: ${status}`);
    setVibespaces((prev) => {
      const updated = prev.map((ws) => (ws.id === id ? { ...ws, status } : ws));
      console.log('[useWorkspaces] Updated vibespaces:', updated);
      return updated;
    });
  }, []);

  /**
   * Deletes a vibespace by ID.
   * @param id - Workspace ID to delete
   * @throws {Error} If deletion fails
   */
  const deleteVibespace = useCallback(
    async (id: string): Promise<void> => {
      try {
        // Optimistically update status to 'deleting'
        updateWorkspaceStatus(id, 'deleting');

        await apiFetch(`${API_ENDPOINTS.vibespaces}/${id}`, {
          method: 'DELETE',
        });

        // Wait briefly to let user see the "deleting" status
        await new Promise(resolve => setTimeout(resolve, 500));

        // Remove vibespace from list after successful deletion
        setVibespaces((prev) => prev.filter((ws) => ws.id !== id));
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to delete vibespace:', err);
        // Revert status on error
        await fetchWorkspaces();
        throw new Error(`Failed to delete vibespace: ${message}`);
      }
    },
    [updateWorkspaceStatus, fetchWorkspaces]
  );

  /**
   * Starts a stopped vibespace.
   * @param id - Workspace ID to start
   * @throws {Error} If start operation fails
   */
  const startVibespace = useCallback(
    async (id: string): Promise<void> => {
      try {
        // Optimistically update status
        updateWorkspaceStatus(id, 'starting');

        await apiFetch(`${API_ENDPOINTS.vibespaces}/${id}/start`, {
          method: 'POST',
        });

        // Wait a bit to let user see the "starting" status, then refresh
        await new Promise(resolve => setTimeout(resolve, 500));
        await fetchWorkspaces();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to start vibespace:', err);
        // Revert status on error
        await fetchWorkspaces();
        throw new Error(`Failed to start vibespace: ${message}`);
      }
    },
    [fetchWorkspaces, updateWorkspaceStatus]
  );

  /**
   * Stops a running vibespace.
   * @param id - Workspace ID to stop
   * @throws {Error} If stop operation fails
   */
  const stopVibespace = useCallback(
    async (id: string): Promise<void> => {
      try {
        // Optimistically update status
        updateWorkspaceStatus(id, 'stopping');

        await apiFetch(`${API_ENDPOINTS.vibespaces}/${id}/stop`, {
          method: 'POST',
        });

        // Wait a bit to let user see the "stopping" status, then refresh
        await new Promise(resolve => setTimeout(resolve, 500));
        await fetchWorkspaces();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to stop vibespace:', err);
        // Revert status on error
        await fetchWorkspaces();
        throw new Error(`Failed to stop vibespace: ${message}`);
      }
    },
    [fetchWorkspaces, updateWorkspaceStatus]
  );

  /**
   * Gets accessible URLs for a vibespace by starting port-forwards.
   * Must be called before opening a vibespace in the browser.
   *
   * @param id - Workspace ID to access
   * @returns A map of URLs where the vibespace can be accessed (e.g., { "code-server": "http://127.0.0.1:8081", "preview": "http://127.0.0.1:8181" })
   * @throws {Error} If vibespace is not running or port-forward fails
   */
  const accessVibespace = useCallback(
    async (id: string): Promise<Record<string, string>> => {
      try {
        const response = await apiFetch<{ urls: Record<string, string> }>(
          `${API_ENDPOINTS.vibespaces}/${id}/access`,
          {
            method: 'GET',
          }
        );

        return response.urls;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to access vibespace:', err);
        throw new Error(`Failed to access vibespace: ${message}`);
      }
    },
    []
  );

  // Fetch vibespaces on mount (with loading state)
  useEffect(() => {
    fetchWorkspaces(true);
  }, [fetchWorkspaces]);

  // Poll for status updates every 3 seconds (without loading state)
  useEffect(() => {
    const interval = setInterval(() => {
      fetchWorkspaces(false); // Background refresh, no loading state
    }, 3000);

    return () => clearInterval(interval);
  }, [fetchWorkspaces]);

  return {
    vibespaces,
    isLoading,
    error,
    refetch: fetchWorkspaces,
    createVibespace,
    deleteVibespace,
    startVibespace,
    stopVibespace,
    accessVibespace,
  };
}
