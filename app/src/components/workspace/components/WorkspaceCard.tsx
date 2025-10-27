import { useState, useRef, useEffect } from 'react';
import { ExternalLink, Play, Square, Trash2, MoreVertical } from 'lucide-react';
import type { Workspace } from '../../../lib/types';
import { DeleteConfirmationModal } from './DeleteConfirmationModal';
import '../styles/WorkspaceCard.css';

interface WorkspaceCardProps {
  workspace: Workspace;
  onOpen: (id: string) => void;
  onStart: (id: string) => Promise<void>;
  onStop: (id: string) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
}

/**
 * Workspace card component displaying workspace information and actions.
 * Shows status, template, resources, and provides controls for open/start/stop/delete.
 */
export function WorkspaceCard({
  workspace,
  onOpen,
  onStart,
  onStop,
  onDelete,
}: WorkspaceCardProps) {
  const [isActionsOpen, setIsActionsOpen] = useState(false);
  const [isOperating, setIsOperating] = useState(false);
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        menuRef.current &&
        buttonRef.current &&
        !menuRef.current.contains(event.target as Node) &&
        !buttonRef.current.contains(event.target as Node)
      ) {
        setIsActionsOpen(false);
      }
    };

    if (isActionsOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      return () => document.removeEventListener('mousedown', handleClickOutside);
    }
  }, [isActionsOpen]);

  const handleStart = async () => {
    if (isOperating) return;
    setIsOperating(true);
    setIsActionsOpen(false);
    try {
      await onStart(workspace.id);
    } finally {
      setIsOperating(false);
    }
  };

  const handleStop = async () => {
    if (isOperating) return;
    setIsOperating(true);
    setIsActionsOpen(false);
    try {
      await onStop(workspace.id);
    } finally {
      setIsOperating(false);
    }
  };

  const handleDelete = () => {
    if (isOperating) return;
    setIsActionsOpen(false);
    setShowDeleteModal(true);
  };

  const confirmDelete = async () => {
    setShowDeleteModal(false);
    setIsOperating(true);
    try {
      await onDelete(workspace.id);
      // Success - workspace will be removed from list after API call
    } catch (error) {
      // Error handling - show user-friendly message
      const message = error instanceof Error ? error.message : 'Unknown error occurred';
      alert(`Failed to delete workspace: ${message}`);
    } finally {
      setIsOperating(false);
    }
  };

  const cancelDelete = () => {
    setShowDeleteModal(false);
  };

  const formatDate = (dateString: string) => {
    try {
      const date = new Date(dateString);
      return new Intl.RelativeTimeFormat('en', { numeric: 'auto' }).format(
        Math.round((date.getTime() - Date.now()) / (1000 * 60 * 60 * 24)),
        'day'
      );
    } catch {
      return dateString;
    }
  };

  const getStatusColor = (status: Workspace['status']) => {
    switch (status) {
      case 'running':
        return 'status-running';
      case 'stopped':
        return 'status-stopped';
      case 'creating':
      case 'starting':
      case 'stopping':
      case 'deleting':
        return 'status-transitioning';
      case 'error':
        return 'status-error';
      default:
        return 'status-unknown';
    }
  };

  const canStart = workspace.status === 'stopped';
  const canStop = workspace.status === 'running';
  const canOpen = workspace.status === 'running' && workspace.urls?.['code-server'];

  return (
    <div className="workspace-card">
      <div className="workspace-card-header">
        <div className="workspace-name-section">
          <h3>{workspace.name}</h3>
          <span className={`workspace-status ${getStatusColor(workspace.status)}`}>
            {workspace.status}
          </span>
        </div>
        <div className="workspace-menu-container">
          <button
            ref={buttonRef}
            className="workspace-menu-btn"
            onClick={() => setIsActionsOpen(!isActionsOpen)}
            aria-label="Workspace actions"
            aria-expanded={isActionsOpen}
            disabled={isOperating}
          >
            <MoreVertical size={18} />
          </button>

          {isActionsOpen && (
            <div ref={menuRef} className="workspace-actions-menu">
              {canStart && (
                <button
                  onClick={handleStart}
                  disabled={isOperating}
                  className="menu-action"
                  aria-label="Start workspace"
                >
                  <Play size={16} />
                  Start
                </button>
              )}
              {canStop && (
                <button
                  onClick={handleStop}
                  disabled={isOperating}
                  className="menu-action"
                  aria-label="Stop workspace"
                >
                  <Square size={16} />
                  Stop
                </button>
              )}
              <button
                onClick={handleDelete}
                disabled={isOperating}
                className="menu-action menu-action-danger"
                aria-label="Delete workspace"
              >
                <Trash2 size={16} />
                Delete
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="workspace-card-body">
        <div className="workspace-meta">
          <span className="meta-label">Template</span>
          <span className="meta-value">{workspace.template}</span>
        </div>
        <div className="workspace-meta">
          <span className="meta-label">CPU</span>
          <span className="meta-value">{workspace.resources.cpu}</span>
        </div>
        <div className="workspace-meta">
          <span className="meta-label">Memory</span>
          <span className="meta-value">{workspace.resources.memory}</span>
        </div>
        <div className="workspace-meta">
          <span className="meta-label">Created</span>
          <span className="meta-value">{formatDate(workspace.created_at)}</span>
        </div>
        {workspace.persistent && (
          <div className="workspace-badge-persistent">
            Persistent
          </div>
        )}
      </div>

      <div className="workspace-card-footer">
        <button
          className="btn-open-workspace"
          onClick={() => onOpen(workspace.id)}
          disabled={!canOpen || isOperating}
          aria-label="Open workspace in browser"
        >
          <ExternalLink size={16} />
          Open
        </button>
      </div>

      {showDeleteModal && (
        <DeleteConfirmationModal
          workspaceName={workspace.name}
          onConfirm={confirmDelete}
          onCancel={cancelDelete}
        />
      )}
    </div>
  );
}
