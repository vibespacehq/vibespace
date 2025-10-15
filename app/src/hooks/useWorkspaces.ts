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

  /**
   * Fetches the list of all workspaces from the API.
   * @throws {Error} If API request fails
   */
  const fetchWorkspaces = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);

      const data = await apiFetch<{ workspaces: Workspace[] }>(
        `${API_ENDPOINTS.workspaces}`
      );

      setWorkspaces(data.workspaces || []);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      console.error('Failed to fetch workspaces:', err);
      setError(`Failed to load workspaces: ${message}`);
    } finally {
      setIsLoading(false);
    }
  }, []);

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
   * Deletes a workspace by ID.
   * @param id - Workspace ID to delete
   * @throws {Error} If deletion fails
   */
  const deleteWorkspace = useCallback(
    async (id: string): Promise<void> => {
      try {
        await apiFetch(`${API_ENDPOINTS.workspaces}/${id}`, {
          method: 'DELETE',
        });

        // Refresh workspace list after deletion
        await fetchWorkspaces();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to delete workspace:', err);
        throw new Error(`Failed to delete workspace: ${message}`);
      }
    },
    [fetchWorkspaces]
  );

  /**
   * Starts a stopped workspace.
   * @param id - Workspace ID to start
   * @throws {Error} If start operation fails
   */
  const startWorkspace = useCallback(
    async (id: string): Promise<void> => {
      try {
        await apiFetch(`${API_ENDPOINTS.workspaces}/${id}/start`, {
          method: 'POST',
        });

        // Refresh workspace list to get updated status
        await fetchWorkspaces();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to start workspace:', err);
        throw new Error(`Failed to start workspace: ${message}`);
      }
    },
    [fetchWorkspaces]
  );

  /**
   * Stops a running workspace.
   * @param id - Workspace ID to stop
   * @throws {Error} If stop operation fails
   */
  const stopWorkspace = useCallback(
    async (id: string): Promise<void> => {
      try {
        await apiFetch(`${API_ENDPOINTS.workspaces}/${id}/stop`, {
          method: 'POST',
        });

        // Refresh workspace list to get updated status
        await fetchWorkspaces();
      } catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown error';
        console.error('Failed to stop workspace:', err);
        throw new Error(`Failed to stop workspace: ${message}`);
      }
    },
    [fetchWorkspaces]
  );

  // Fetch workspaces on mount
  useEffect(() => {
    fetchWorkspaces();
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
  };
}
