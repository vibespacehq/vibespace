import { useKubernetesStatus } from '../../hooks/useKubernetesStatus';
import { InstallationInstructions } from './InstallationInstructions';
import './KubernetesSetup.css';

export function KubernetesSetup() {
  const { status, isLoading, refetch } = useKubernetesStatus();

  if (isLoading) {
    return (
      <div className="setup-container">
        <aside className="setup-sidebar">
          <div className="sidebar-logo">
            <h1>workspaces</h1>
            <p>setup</p>
          </div>
          <div className="progress-steps">
            <div className="progress-step completed">
              <div className="step-indicator">1</div>
              <div className="step-content">
                <h3>Authentication</h3>
                <p>Sign in complete</p>
              </div>
            </div>
            <div className="progress-step active">
              <div className="step-indicator">2</div>
              <div className="step-content">
                <h3>Infrastructure</h3>
                <p>Checking environment</p>
              </div>
            </div>
            <div className="progress-step">
              <div className="step-indicator">3</div>
              <div className="step-content">
                <h3>Configuration</h3>
                <p>workspace settings</p>
              </div>
            </div>
            <div className="progress-step">
              <div className="step-indicator">4</div>
              <div className="step-content">
                <h3>Ready</h3>
                <p>Launch workspace</p>
              </div>
            </div>
          </div>
        </aside>
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Infrastructure Detection</h1>
            <p className="brand-subtitle">Scanning your system for container runtime</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="25"></div>
            </div>
          </header>
          <div className="setup-loading">
            <div className="spinner" />
            <p>Detecting infrastructure...</p>
          </div>
        </main>
      </div>
    );
  }

  if (status?.available) {
    return (
      <div className="setup-container">
        <aside className="setup-sidebar">
          <div className="sidebar-logo">
            <h1>workspaces</h1>
            <p>setup</p>
          </div>
          <div className="progress-steps">
            <div className="progress-step completed">
              <div className="step-indicator">1</div>
              <div className="step-content">
                <h3>Authentication</h3>
                <p>Sign in complete</p>
              </div>
            </div>
            <div className="progress-step completed">
              <div className="step-indicator">2</div>
              <div className="step-content">
                <h3>Infrastructure</h3>
                <p>Environment ready</p>
              </div>
            </div>
            <div className="progress-step active">
              <div className="step-indicator">3</div>
              <div className="step-content">
                <h3>Configuration</h3>
                <p>workspace settings</p>
              </div>
            </div>
            <div className="progress-step">
              <div className="step-indicator">4</div>
              <div className="step-content">
                <h3>Ready</h3>
                <p>Launch workspace</p>
              </div>
            </div>
          </div>
        </aside>
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">3</span>
              <span>Step 3 of 4</span>
            </div>
            <h1 className="brand-title">Configuration</h1>
            <p className="brand-subtitle">Set up your workspace preferences</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="50"></div>
            </div>
          </header>
          <div className="setup-success">
            <div className="success-icon">OK</div>
            <h2>Infrastructure ready</h2>
            <div className="cluster-info">
              {status.installType && (
                <p>
                  <strong>Type:</strong> {status.installType}
                </p>
              )}
              {status.version && (
                <p>
                  <strong>Version:</strong> {status.version}
                </p>
              )}
              {status.kubeconfigPath && (
                <p className="kubeconfig-path">
                  <strong>Config:</strong> {status.kubeconfigPath}
                </p>
              )}
            </div>
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="setup-container">
      <aside className="setup-sidebar">
        <div className="sidebar-logo">
          <h1>workspaces</h1>
          <p>setup</p>
        </div>
        <div className="progress-steps">
          <div className="progress-step completed">
            <div className="step-indicator">1</div>
            <div className="step-content">
              <h3>Authentication</h3>
              <p>Sign in complete</p>
            </div>
          </div>
          <div className="progress-step active">
            <div className="step-indicator">2</div>
            <div className="step-content">
              <h3>Infrastructure</h3>
              <p>Setting up environment</p>
            </div>
          </div>
          <div className="progress-step">
            <div className="step-indicator">3</div>
            <div className="step-content">
              <h3>Configuration</h3>
              <p>workspace settings</p>
            </div>
          </div>
          <div className="progress-step">
            <div className="step-indicator">4</div>
            <div className="step-content">
              <h3>Ready</h3>
              <p>Launch workspace</p>
            </div>
          </div>
        </div>
      </aside>
      <main className="setup-main">
        <header className="setup-header">
          <div className="step-badge">
            <span className="step-badge-number">2</span>
            <span>Step 2 of 4</span>
          </div>
          <h1 className="brand-title">Infrastructure Setup</h1>
          <p className="brand-subtitle">Install the container orchestration layer</p>
          <div className="progress-bar-container">
            <div className="progress-bar-fill" data-progress="25"></div>
          </div>
        </header>
        <div className="setup-required">
          <h2>Container orchestration required</h2>
          {status?.error && (
            <div className="error-message">
              <span className="error-icon">!</span>
              {status.error}
            </div>
          )}

        <InstallationInstructions suggestedAction={status?.suggestedAction} />

        <div className="setup-actions">
          <button onClick={refetch} className="btn-primary">
            Verify installation
          </button>
        </div>
      </div>
      </main>
    </div>
  );
}
