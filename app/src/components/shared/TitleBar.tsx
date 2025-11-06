import { getCurrentWindow } from '@tauri-apps/api/window';
import './TitleBar.css';

export function TitleBar() {
  const handleMinimize = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    try {
      const appWindow = getCurrentWindow();
      await appWindow.minimize();
    } catch (error) {
      console.error('[TitleBar] Minimize error:', error);
    }
  };

  const handleMaximize = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    try {
      const appWindow = getCurrentWindow();
      await appWindow.toggleMaximize();
    } catch (error) {
      console.error('[TitleBar] Maximize error:', error);
    }
  };

  const handleClose = async (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    try {
      const appWindow = getCurrentWindow();
      await appWindow.close();
    } catch (error) {
      console.error('[TitleBar] Close error:', error);
    }
  };

  return (
    <div className="custom-titlebar">
      <div className="titlebar-drag-region" data-tauri-drag-region></div>

      <div className="titlebar-left">
        <div className="window-controls">
          <button
            className="titlebar-button close"
            onClick={handleClose}
            aria-label="Close"
          >
            <svg width="10" height="10" viewBox="0 0 10 10">
              <path d="M 0,0 L 10,10 M 10,0 L 0,10" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </button>
          <button
            className="titlebar-button minimize"
            onClick={handleMinimize}
            aria-label="Minimize"
          >
            <svg width="10" height="10" viewBox="0 0 10 10">
              <path d="M 0,5 L 10,5" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </button>
          <button
            className="titlebar-button maximize"
            onClick={handleMaximize}
            aria-label="Maximize"
          >
            <svg width="10" height="10" viewBox="0 0 10 10">
              <rect x="0" y="0" width="10" height="10" fill="none" stroke="currentColor" strokeWidth="1.5" />
            </svg>
          </button>
        </div>
      </div>

      <div className="titlebar-center">
        <img
          src="/icon.png"
          alt="vibespace"
          className="titlebar-icon"
        />
        <div className="titlebar-divider"></div>
        <span className="titlebar-title">vibespace</span>
      </div>

      <div className="titlebar-right">
        {/* Reserved for future controls */}
      </div>
    </div>
  );
}
