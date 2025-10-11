import { getCurrentWindow } from '@tauri-apps/api/window';
import './TitleBar.css';

export function TitleBar() {
  console.log('[TitleBar] Component mounted');

  const handleMinimize = async (e: React.MouseEvent) => {
    console.log('[TitleBar] Minimize button clicked', e);
    e.preventDefault();
    e.stopPropagation();
    try {
      const appWindow = getCurrentWindow();
      console.log('[TitleBar] Got window instance:', appWindow);
      console.log('[TitleBar] Calling minimize()...');
      await appWindow.minimize();
      console.log('[TitleBar] Minimize successful');
    } catch (error) {
      console.error('[TitleBar] Minimize error:', error);
    }
  };

  const handleMaximize = async (e: React.MouseEvent) => {
    console.log('[TitleBar] Maximize button clicked', e);
    e.preventDefault();
    e.stopPropagation();
    try {
      const appWindow = getCurrentWindow();
      console.log('[TitleBar] Got window instance:', appWindow);
      console.log('[TitleBar] Calling toggleMaximize()...');
      await appWindow.toggleMaximize();
      console.log('[TitleBar] Maximize successful');
    } catch (error) {
      console.error('[TitleBar] Maximize error:', error);
    }
  };

  const handleClose = async (e: React.MouseEvent) => {
    console.log('[TitleBar] Close button clicked', e);
    e.preventDefault();
    e.stopPropagation();
    try {
      const appWindow = getCurrentWindow();
      console.log('[TitleBar] Got window instance:', appWindow);
      console.log('[TitleBar] Calling close()...');
      await appWindow.close();
      console.log('[TitleBar] Close successful');
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
          src="/src-tauri/icons/icon.png"
          alt="workspaces"
          className="titlebar-icon"
        />
        <div className="titlebar-divider"></div>
        <span className="titlebar-title">workspaces</span>
      </div>

      <div className="titlebar-right">
        {/* Reserved for future controls */}
      </div>
    </div>
  );
}
