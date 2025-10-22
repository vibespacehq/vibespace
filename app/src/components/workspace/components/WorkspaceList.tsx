import { Plus } from 'lucide-react';
import { useWorkspaces } from '../../../hooks/useWorkspaces';
import { WorkspaceCard } from './WorkspaceCard';
import { WorkspaceEmptyState } from './WorkspaceEmptyState';
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

  const handleOpen = async (id: string) => {
    try {
      // Call access endpoint to get port-forward URL
      const url = await accessWorkspace(id);
      window.open(url, '_blank');
    } catch (err) {
      console.error('Failed to open workspace:', err);
      // TODO: Show error toast/notification to user
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
