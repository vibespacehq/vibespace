import { useState, useRef, useEffect } from 'react';
import { ExternalLink, Play, Square, Trash2, MoreVertical } from 'lucide-react';
import type { Vibespace } from '../../../lib/types';
import { DeleteConfirmationModal } from './DeleteConfirmationModal';
import '../styles/VibespaceCard.css';

interface VibespaceCardProps {
  vibespace: Vibespace;
  onOpen: (id: string, urlType?: string) => void;
  onStart: (id: string) => Promise<void>;
  onStop: (id: string) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
}

/**
 * Vibespace card component displaying vibespace information and actions.
 * Shows status, template, resources, and provides controls for open/start/stop/delete.
 */
export function VibespaceCard({
  vibespace,
  onOpen,
  onStart,
  onStop,
  onDelete,
}: VibespaceCardProps) {
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
      await onStart(vibespace.id);
    } finally {
      setIsOperating(false);
    }
  };

  const handleStop = async () => {
    if (isOperating) return;
    setIsOperating(true);
    setIsActionsOpen(false);
    try {
      await onStop(vibespace.id);
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
      await onDelete(vibespace.id);
      // Success - vibespace will be removed from list after API call
    } catch (error) {
      // Error handling - show user-friendly message
      const message = error instanceof Error ? error.message : 'Unknown error occurred';
      alert(`Failed to delete vibespace: ${message}`);
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

  const getStatusColor = (status: Vibespace['status']) => {
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

  const canStart = vibespace.status === 'stopped';
  const canStop = vibespace.status === 'running';

  // Knative DNS URLs: code/preview/prod subdomains
  const hasUrls = vibespace.urls && Object.keys(vibespace.urls).length > 0;
  const canOpenCode = vibespace.status === 'running' && (vibespace.urls?.code || vibespace.urls?.['code-server']);
  const canOpenPreview = vibespace.status === 'running' && vibespace.urls?.preview;
  const canOpenProd = vibespace.status === 'running' && vibespace.urls?.prod;

  return (
    <div className="vibespace-card">
      <div className="vibespace-card-header">
        <div className="vibespace-name-section">
          <h3>{vibespace.name}</h3>
          <span className={`vibespace-status ${getStatusColor(vibespace.status)}`}>
            {vibespace.status}
          </span>
        </div>
        <div className="vibespace-menu-container">
          <button
            ref={buttonRef}
            className="vibespace-menu-btn"
            onClick={() => setIsActionsOpen(!isActionsOpen)}
            aria-label="Vibespace actions"
            aria-expanded={isActionsOpen}
            disabled={isOperating}
          >
            <MoreVertical size={18} />
          </button>

          {isActionsOpen && (
            <div ref={menuRef} className="vibespace-actions-menu">
              {canStart && (
                <button
                  onClick={handleStart}
                  disabled={isOperating}
                  className="menu-action"
                  aria-label="Start vibespace"
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
                  aria-label="Stop vibespace"
                >
                  <Square size={16} />
                  Stop
                </button>
              )}
              <button
                onClick={handleDelete}
                disabled={isOperating}
                className="menu-action menu-action-danger"
                aria-label="Delete vibespace"
              >
                <Trash2 size={16} />
                Delete
              </button>
            </div>
          )}
        </div>
      </div>

      <div className="vibespace-card-body">
        <div className="vibespace-meta">
          <span className="meta-label">Template</span>
          <span className="meta-value">{vibespace.template}</span>
        </div>
        <div className="vibespace-meta">
          <span className="meta-label">CPU</span>
          <span className="meta-value">{vibespace.resources.cpu}</span>
        </div>
        <div className="vibespace-meta">
          <span className="meta-label">Memory</span>
          <span className="meta-value">{vibespace.resources.memory}</span>
        </div>
        <div className="vibespace-meta">
          <span className="meta-label">Created</span>
          <span className="meta-value">{formatDate(vibespace.created_at)}</span>
        </div>
        {vibespace.persistent && (
          <div className="vibespace-badge-persistent">
            Persistent
          </div>
        )}
      </div>

      <div className="vibespace-card-footer">
        {canOpenCode && (
          <button
            className="btn-open-vibespace"
            onClick={() => onOpen(vibespace.id, 'code')}
            disabled={isOperating}
            aria-label="Open code-server in browser"
          >
            <ExternalLink size={16} />
            Code
          </button>
        )}
        {canOpenPreview && (
          <button
            className="btn-open-preview"
            onClick={() => onOpen(vibespace.id, 'preview')}
            disabled={isOperating}
            aria-label="Open preview server in browser"
          >
            <ExternalLink size={16} />
            Preview
          </button>
        )}
        {canOpenProd && (
          <button
            className="btn-open-preview"
            onClick={() => onOpen(vibespace.id, 'prod')}
            disabled={isOperating}
            aria-label="Open production server in browser"
          >
            <ExternalLink size={16} />
            Production
          </button>
        )}
      </div>

      {showDeleteModal && (
        <DeleteConfirmationModal
          vibespaceName={vibespace.name}
          onConfirm={confirmDelete}
          onCancel={cancelDelete}
        />
      )}
    </div>
  );
}
