import { useState, useEffect, useCallback } from 'react';
import { API_ENDPOINTS, apiFetch } from '../lib/api-config';
import type { Workspace, CreateWorkspaceRequest } from '../lib/types';

interface UseWorkspacesReturn {
  workspaces: Workspace[];
  isLoading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
  createWorkspace: (request: CreateWorkspaceRequest) => Promise<Workspace>;
  deleteWorkspace: (id: string) => Promise<void>;
  startWorkspace: (id: string) => Promise<void>;
  stopWorkspace: (id: string) => Promise<void>;
  accessWorkspace: (id: string) => Promise<Record<string, string>>;
}

/**
 * React hook for managing workspace CRUD operations.
 * Provides methods to list, create, delete, start, and stop workspaces.
 *
 * @returns Object containing workspace data, loading state, error state, and operation methods
 *
 * @example
 * ```tsx
 * function WorkspaceList() {
 *   const { workspaces, isLoading, error, createWorkspace, deleteWorkspace } = useWorkspaces();
 *
 *   const handleCreate = async () => {
 *     await createWorkspace({
 *       name: 'my-workspace',
 *       template: 'nextjs',
 *       persistent: true
 *     });
 *   };
 *
 *   return (
 *     <div>
 *       {workspaces.map(ws => <WorkspaceCard key={ws.id} workspace={ws} />)}
 *     </div>
 *   );
 * }
 * ```
 */
export function useWorkspaces(): UseWorkspacesReturn {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isInitialLoad, setIsInitialLoad] = useState(true);

  /**
   * Fetches the list of all workspaces from the API.
   * @param showLoading - Whether to show loading state (true for initial load, false for polling)
   * @throws {Error} If API request fails
   */
  const fetchWorkspaces = useCallback(async (showLoading = false) => {
    try {
      if (showLoading) {
        setIsLoading(true);
      }
      setError(null);

      const data = await apiFetch<{ workspaces: Workspace[] }>(
        `${API_ENDPOINTS.workspaces}`
      );

      setWorkspaces(data.workspaces || []);

      if (isInitialLoad) {
        setIsInitialLoad(false);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      console.error('Failed to fetch workspaces:', err);
      setError(`Failed to load workspaces: ${message}`);
    } finally {
      if (showLoading) {
        setIsLoading(false);
      }
    }
  }, [isInitialLoad]);

  /**
   * Creates a new workspace with the specified configuration.
   * @param request - Workspace creation parameters
   * @returns The created workspace object
   * @throws {Error} If workspace creation fails
   */
  const createWorkspace = useCallback(
    async (request: CreateWorkspaceRequest): Promise<Workspace> => {
      try {
        const workspace = await apiFetch<Workspace>(
          `${API_ENDPOINTS.workspaces}`,
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(request),
          }
        );

        // Refresh workspace list after creation
        await fetchWorkspaces();

        return workspace;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to create workspace:', err);
        throw new Error(`Failed to create workspace: ${message}`);
      }
    },
    [fetchWorkspaces]
  );

  /**
   * Updates a workspace's status optimistically (before API response).
   * @param id - Workspace ID
   * @param status - New status to set
   */
  const updateWorkspaceStatus = useCallback((id: string, status: Workspace['status']) => {
    console.log(`[useWorkspaces] Optimistically updating workspace ${id} to status: ${status}`);
    setWorkspaces((prev) => {
      const updated = prev.map((ws) => (ws.id === id ? { ...ws, status } : ws));
      console.log('[useWorkspaces] Updated workspaces:', updated);
      return updated;
    });
  }, []);

  /**
   * Deletes a workspace by ID.
   * @param id - Workspace ID to delete
   * @throws {Error} If deletion fails
   */
  const deleteWorkspace = useCallback(
    async (id: string): Promise<void> => {
      try {
        // Optimistically update status to 'deleting'
        updateWorkspaceStatus(id, 'deleting');

        await apiFetch(`${API_ENDPOINTS.workspaces}/${id}`, {
          method: 'DELETE',
        });

        // Wait briefly to let user see the "deleting" status
        await new Promise(resolve => setTimeout(resolve, 500));

        // Remove workspace from list after successful deletion
        setWorkspaces((prev) => prev.filter((ws) => ws.id !== id));
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to delete workspace:', err);
        // Revert status on error
        await fetchWorkspaces();
        throw new Error(`Failed to delete workspace: ${message}`);
      }
    },
    [updateWorkspaceStatus, fetchWorkspaces]
  );

  /**
   * Starts a stopped workspace.
   * @param id - Workspace ID to start
   * @throws {Error} If start operation fails
   */
  const startWorkspace = useCallback(
    async (id: string): Promise<void> => {
      try {
        // Optimistically update status
        updateWorkspaceStatus(id, 'starting');

        await apiFetch(`${API_ENDPOINTS.workspaces}/${id}/start`, {
          method: 'POST',
        });

        // Wait a bit to let user see the "starting" status, then refresh
        await new Promise(resolve => setTimeout(resolve, 500));
        await fetchWorkspaces();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to start workspace:', err);
        // Revert status on error
        await fetchWorkspaces();
        throw new Error(`Failed to start workspace: ${message}`);
      }
    },
    [fetchWorkspaces, updateWorkspaceStatus]
  );

  /**
   * Stops a running workspace.
   * @param id - Workspace ID to stop
   * @throws {Error} If stop operation fails
   */
  const stopWorkspace = useCallback(
    async (id: string): Promise<void> => {
      try {
        // Optimistically update status
        updateWorkspaceStatus(id, 'stopping');

        await apiFetch(`${API_ENDPOINTS.workspaces}/${id}/stop`, {
          method: 'POST',
        });

        // Wait a bit to let user see the "stopping" status, then refresh
        await new Promise(resolve => setTimeout(resolve, 500));
        await fetchWorkspaces();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to stop workspace:', err);
        // Revert status on error
        await fetchWorkspaces();
        throw new Error(`Failed to stop workspace: ${message}`);
      }
    },
    [fetchWorkspaces, updateWorkspaceStatus]
  );

  /**
   * Gets accessible URLs for a workspace by starting port-forwards.
   * Must be called before opening a workspace in the browser.
   *
   * @param id - Workspace ID to access
   * @returns A map of URLs where the workspace can be accessed (e.g., { "code-server": "http://127.0.0.1:8081", "preview": "http://127.0.0.1:8181" })
   * @throws {Error} If workspace is not running or port-forward fails
   */
  const accessWorkspace = useCallback(
    async (id: string): Promise<Record<string, string>> => {
      try {
        const response = await apiFetch<{ urls: Record<string, string> }>(
          `${API_ENDPOINTS.workspaces}/${id}/access`,
          {
            method: 'GET',
          }
        );

        return response.urls;
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to access workspace:', err);
        throw new Error(`Failed to access workspace: ${message}`);
      }
    },
    []
  );

  // Fetch workspaces on mount (with loading state)
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
    workspaces,
    isLoading,
    error,
    refetch: fetchWorkspaces,
    createWorkspace,
    deleteWorkspace,
    startWorkspace,
    stopWorkspace,
    accessWorkspace,
  };
}
