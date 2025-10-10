import { useKubernetesStatus } from '../../hooks/useKubernetesStatus';
import { InstallationInstructions } from './InstallationInstructions';
import './KubernetesSetup.css';

export function KubernetesSetup() {
  const { status, isLoading, refetch } = useKubernetesStatus();

  if (isLoading) {
    return (
      <div className="setup-container">
        <div className="setup-loading">
          <div className="spinner" />
          <p>Detecting Kubernetes installation...</p>
        </div>
      </div>
    );
  }

  if (status?.available) {
    return (
      <div className="setup-container">
        <div className="setup-success">
          <div className="success-icon">✓</div>
          <h2>Kubernetes Ready</h2>
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
      </div>
    );
  }

  return (
    <div className="setup-container">
      <div className="setup-required">
        <h2>Kubernetes Required</h2>
        {status?.error && (
          <div className="error-message">
            <span className="error-icon">⚠</span>
            {status.error}
          </div>
        )}

        <InstallationInstructions suggestedAction={status?.suggestedAction} />

        <div className="setup-actions">
          <button onClick={refetch} className="btn-primary">
            Verify Installation
          </button>
        </div>
      </div>
    </div>
  );
}
