import { Plus } from 'lucide-react';
import '../styles/WorkspaceEmptyState.css';

interface WorkspaceEmptyStateProps {
  onCreateNew: () => void;
}

/**
 * Empty state component displayed when no workspaces exist.
 * Encourages users to create their first workspace.
 */
export function WorkspaceEmptyState({ onCreateNew }: WorkspaceEmptyStateProps) {
  return (
    <div className="workspace-empty-state">
      <div className="empty-icon-container">
        <svg
          className="empty-icon"
          width="120"
          height="120"
          viewBox="0 0 120 120"
          fill="none"
          xmlns="http://www.w3.org/2000/svg"
        >
          <rect
            x="20"
            y="30"
            width="80"
            height="60"
            rx="4"
            stroke="currentColor"
            strokeWidth="2"
            fill="none"
          />
          <line
            x1="20"
            y1="45"
            x2="100"
            y2="45"
            stroke="currentColor"
            strokeWidth="2"
          />
          <circle cx="30" cy="37.5" r="2" fill="currentColor" />
          <circle cx="38" cy="37.5" r="2" fill="currentColor" />
          <circle cx="46" cy="37.5" r="2" fill="currentColor" />
          <line
            x1="30"
            y1="60"
            x2="70"
            y2="60"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
          />
          <line
            x1="30"
            y1="70"
            x2="60"
            y2="70"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
          />
        </svg>
      </div>

      <div className="empty-content">
        <h2>No workspaces yet</h2>
        <p>
          Create your first containerized development environment with code-server
          and AI coding agents.
        </p>
      </div>

      <button className="btn-create-first" onClick={onCreateNew}>
        <Plus size={18} />
        Create workspace
      </button>
    </div>
  );
}
