import { useKubernetesStatus } from '../../hooks/useKubernetesStatus';
import { InstallationInstructions } from './InstallationInstructions';
import { ProgressSidebar } from './ProgressSidebar';
import './KubernetesSetup.css';

interface KubernetesSetupProps {
  onComplete?: () => void;
}

export function KubernetesSetup({ onComplete }: KubernetesSetupProps) {
  const { status, isLoading, refetch } = useKubernetesStatus();

  if (isLoading) {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
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
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Infrastructure Ready</h1>
            <p className="brand-subtitle">Kubernetes cluster detected successfully</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="25"></div>
            </div>
          </header>
          <div className="setup-success">
            <div className="success-icon">✓</div>
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
            {onComplete && (
              <div className="setup-actions">
                <button onClick={onComplete} className="btn-primary">
                  Continue
                </button>
              </div>
            )}
          </div>
        </main>
      </div>
    );
  }

  return (
    <div className="setup-container">
      <ProgressSidebar currentStep={2} />
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
          {/* Temporary skip button for testing */}
          {onComplete && (
            <button onClick={onComplete} className="btn-primary" style={{ marginLeft: '1rem' }}>
              Skip (Testing)
            </button>
          )}
        </div>
      </div>
      </main>
    </div>
  );
}
