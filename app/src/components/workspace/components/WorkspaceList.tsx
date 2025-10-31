import { Plus } from 'lucide-react';
import { useWorkspaces } from '../../../hooks/useWorkspaces';
import { WorkspaceCard } from './WorkspaceCard';
import { WorkspaceEmptyState } from './WorkspaceEmptyState';
import { open } from '@tauri-apps/plugin-shell';
import '../styles/workspace.css';

interface WorkspaceListProps {
  onCreateNew: () => void;
}

/**
 * Main workspace list view displaying all user workspaces.
 * Fetches workspace data from API and provides management controls.
 */
export function WorkspaceList({ onCreateNew }: WorkspaceListProps) {
  const {
    workspaces,
    isLoading,
    error,
    refetch,
    startWorkspace,
    stopWorkspace,
    deleteWorkspace,
    accessWorkspace,
  } = useWorkspaces();

  const handleOpen = async (id: string, urlType: string = 'code-server') => {
    try {
      // Call access endpoint to get port-forward URLs
      const urls = await accessWorkspace(id);
      const url = urls[urlType];

      if (!url) {
        throw new Error(`${urlType} URL not available for this workspace`);
      }

      // Check if running in Tauri context
      if (window.__TAURI__) {
        // Use Tauri shell plugin to open URL in default browser
        await open(url);
      } else {
        // Fallback for browser/dev mode
        console.log('Opening URL (Tauri not available):', url);
        window.open(url, '_blank');
      }
    } catch (err) {
      console.error('Failed to open workspace:', err);
      const errorMessage = err instanceof Error ? err.message : 'Unknown error';
      alert(`Failed to open workspace: ${errorMessage}\n\nPlease ensure the workspace is running and try again.`);
    }
  };

  if (isLoading) {
    return (
      <div className="workspace-list-container">
        <header className="workspace-header">
          <div className="header-content">
            <div className="header-title">
              <img src="/icon-transparent.png" alt="workspaces" className="header-icon" />
              <h1>workspaces</h1>
            </div>
            <p>Loading workspaces...</p>
          </div>
        </header>
        <div className="workspace-loading">
          <div className="spinner" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="workspace-list-container">
        <header className="workspace-header">
          <div className="header-content">
            <div className="header-title">
              <img src="/icon-transparent.png" alt="workspaces" className="header-icon" />
              <h1>workspaces</h1>
            </div>
            <p className="error-text">{error}</p>
          </div>
          <button className="btn-retry" onClick={refetch}>
            Retry
          </button>
        </header>
      </div>
    );
  }

  if (workspaces.length === 0) {
    return (
      <div className="workspace-list-container">
        <header className="workspace-header">
          <div className="header-content">
            <div className="header-title">
              <img src="/icon-transparent.png" alt="workspaces" className="header-icon" />
              <h1>workspaces</h1>
            </div>
            <p>Containerized development environments with AI coding agents</p>
          </div>
          <button className="btn-new-workspace" onClick={onCreateNew}>
            <Plus size={18} />
            New workspace
          </button>
        </header>

        <WorkspaceEmptyState onCreateNew={onCreateNew} />
      </div>
    );
  }

  return (
    <div className="workspace-list-container">
      <header className="workspace-header">
        <div className="header-content">
          <div className="header-title">
            <img src="/icon-transparent.png" alt="workspaces" className="header-icon" />
            <h1>workspaces</h1>
          </div>
          <p>
            {workspaces.length} {workspaces.length === 1 ? 'workspace' : 'workspaces'}
          </p>
        </div>
        <button className="btn-new-workspace" onClick={onCreateNew}>
          <Plus size={18} />
          New workspace
        </button>
      </header>

      <div className="workspace-grid">
        {workspaces.map((workspace) => (
          <WorkspaceCard
            key={workspace.id}
            workspace={workspace}
            onOpen={handleOpen}
            onStart={startWorkspace}
            onStop={stopWorkspace}
            onDelete={deleteWorkspace}
          />
        ))}
      </div>
    </div>
  );
}
