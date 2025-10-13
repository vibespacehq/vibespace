interface Workspace {
  id: string;
  name: string;
  template: string;
  status: 'running' | 'stopped' | 'creating' | 'error';
  createdAt: string;
}

interface WorkspaceListProps {
  workspaces: Workspace[];
  onCreateNew: () => void;
}

export function WorkspaceList({ workspaces, onCreateNew }: WorkspaceListProps) {
  if (workspaces.length === 0) {
    return (
      <div className="workspace-list-container">
        <header className="workspace-header">
          <div className="header-content">
            <h1>workspaces</h1>
            <p>Containerized development environments with AI coding agents</p>
          </div>
          <button className="btn-new-workspace" onClick={onCreateNew}>
            + New workspace
          </button>
        </header>

        <div className="empty-state">
          <div className="empty-icon">◇</div>
          <h2>No workspaces yet</h2>
          <p>Create your first workspace to get started</p>
          <button className="btn-create-first" onClick={onCreateNew}>
            Create workspace
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="workspace-list-container">
      <header className="workspace-header">
        <div className="header-content">
          <h1>workspaces</h1>
          <p>{workspaces.length} {workspaces.length === 1 ? 'workspace' : 'workspaces'}</p>
        </div>
        <button className="btn-new-workspace" onClick={onCreateNew}>
          + New workspace
        </button>
      </header>

      <div className="workspace-grid">
        {workspaces.map((workspace) => (
          <div key={workspace.id} className="workspace-card">
            <div className="workspace-card-header">
              <h3>{workspace.name}</h3>
              <span className={`status-badge status-${workspace.status}`}>
                {workspace.status}
              </span>
            </div>
            <div className="workspace-card-body">
              <div className="workspace-meta">
                <span className="meta-label">Template</span>
                <span className="meta-value">{workspace.template}</span>
              </div>
              <div className="workspace-meta">
                <span className="meta-label">Created</span>
                <span className="meta-value">{workspace.createdAt}</span>
              </div>
            </div>
            <div className="workspace-card-actions">
              <button className="btn-action btn-open">Open</button>
              <button className="btn-action btn-more" aria-label="More actions">
                <span aria-hidden="true">⋯</span>
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
