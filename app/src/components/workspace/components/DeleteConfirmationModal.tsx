import { useState, useEffect, useRef } from 'react';
import { Trash2 } from 'lucide-react';
import '../styles/DeleteConfirmationModal.css';

interface DeleteConfirmationModalProps {
  workspaceName: string;
  onConfirm: () => void;
  onCancel: () => void;
}

/**
 * Modal for confirming workspace deletion.
 * Requires user to type workspace name to prevent accidental deletion.
 */
export function DeleteConfirmationModal({
  workspaceName,
  onConfirm,
  onCancel,
}: DeleteConfirmationModalProps) {
  const [inputValue, setInputValue] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  // Focus input when modal opens
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Handle Enter key to confirm (if name matches)
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && inputValue === workspaceName) {
      onConfirm();
    } else if (e.key === 'Escape') {
      onCancel();
    }
  };

  const isValid = inputValue === workspaceName;

  return (
    <div className="modal-overlay" onClick={onCancel}>
      <div className="modal-content" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <div className="modal-icon-warning">
            <Trash2 size={24} />
          </div>
          <h2>Delete Workspace</h2>
        </div>

        <div className="modal-body">
          <p className="modal-warning">
            ⚠️ WARNING: This will permanently delete workspace "<strong>{workspaceName}</strong>" and all its data.
          </p>
          <p className="modal-instruction">
            Type the workspace name "<strong>{workspaceName}</strong>" to confirm deletion:
          </p>
          <input
            ref={inputRef}
            type="text"
            className="modal-input"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={workspaceName}
            autoComplete="off"
          />
        </div>

        <div className="modal-footer">
          <button
            className="modal-btn modal-btn-cancel"
            onClick={onCancel}
            type="button"
          >
            Cancel
          </button>
          <button
            className="modal-btn modal-btn-danger"
            onClick={onConfirm}
            disabled={!isValid}
            type="button"
          >
            Delete Workspace
          </button>
        </div>
      </div>
    </div>
  );
}
