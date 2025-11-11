import { Plus } from 'lucide-react';
import { useVibespaces } from '../../../hooks/useVibespaces';
import { VibespaceCard } from './VibespaceCard';
import { VibespaceEmptyState } from './VibespaceEmptyState';
import { open } from '@tauri-apps/plugin-shell';
import '../styles/vibespace.css';

interface VibespaceListProps {
  onCreateNew: () => void;
}

/**
 * Main vibespace list view displaying all user vibespaces.
 * Fetches vibespace data from API and provides management controls.
 */
export function VibespaceList({ onCreateNew }: VibespaceListProps) {
  const {
    vibespaces,
    isLoading,
    error,
    refetch,
    startVibespace,
    stopVibespace,
    deleteVibespace,
    accessVibespace,
  } = useVibespaces();

  const handleOpen = async (id: string, urlType: string = 'code') => {
    try {
      // Find vibespace in current list to check if DNS URLs are available
      const vibespace = vibespaces.find((vs) => vs.id === id);
      let url: string | undefined;

      // Optimization: Use DNS URLs directly if available (Knative mode)
      if (vibespace?.urls && Object.keys(vibespace.urls).length > 0) {
        url = vibespace.urls[urlType];
      } else {
        // Fallback: Call /access endpoint for port-forward URLs (legacy Pod mode)
        const urls = await accessVibespace(id);
        url = urls[urlType];
      }

      if (!url) {
        throw new Error(`${urlType} URL not available for this vibespace`);
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
      console.error('Failed to open vibespace:', err);
      const errorMessage = err instanceof Error ? err.message : 'Unknown error';
      alert(`Failed to open vibespace: ${errorMessage}\n\nPlease ensure the vibespace is running and try again.`);
    }
  };

  if (isLoading) {
    return (
      <div className="vibespace-list-container">
        <header className="vibespace-header">
          <div className="header-content">
            <div className="header-title">
              <img src="/icon-transparent.png" alt="vibespace" className="header-icon" />
              <h1>vibespace</h1>
            </div>
            <p>Loading vibespaces...</p>
          </div>
        </header>
        <div className="vibespace-loading">
          <div className="spinner" />
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="vibespace-list-container">
        <header className="vibespace-header">
          <div className="header-content">
            <div className="header-title">
              <img src="/icon-transparent.png" alt="vibespace" className="header-icon" />
              <h1>vibespace</h1>
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

  if (vibespaces.length === 0) {
    return (
      <div className="vibespace-list-container">
        <header className="vibespace-header">
          <div className="header-content">
            <div className="header-title">
              <img src="/icon-transparent.png" alt="vibespace" className="header-icon" />
              <h1>vibespace</h1>
            </div>
            <p>Containerized development environments with AI coding agents</p>
          </div>
          <button className="btn-new-vibespace" onClick={onCreateNew}>
            <Plus size={18} />
            New vibespace
          </button>
        </header>

        <VibespaceEmptyState onCreateNew={onCreateNew} />
      </div>
    );
  }

  return (
    <div className="vibespace-list-container">
      <header className="vibespace-header">
        <div className="header-content">
          <div className="header-title">
            <img src="/icon-transparent.png" alt="vibespace" className="header-icon" />
            <h1>vibespace</h1>
          </div>
          <p>
            {vibespaces.length} {vibespaces.length === 1 ? 'vibespace' : 'vibespaces'}
          </p>
        </div>
        <button className="btn-new-vibespace" onClick={onCreateNew}>
          <Plus size={18} />
          New vibespace
        </button>
      </header>

      <div className="vibespace-grid">
        {vibespaces.map((vibespace) => (
          <VibespaceCard
            key={vibespace.id}
            vibespace={vibespace}
            onOpen={handleOpen}
            onStart={startVibespace}
            onStop={stopVibespace}
            onDelete={deleteVibespace}
          />
        ))}
      </div>
    </div>
  );
}
