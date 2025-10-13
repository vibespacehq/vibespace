import { useState, useEffect } from 'react';
import { ProgressSidebar } from './ProgressSidebar';
import { ClusterStatus, SetupProgress } from '../../../lib/types';
import '../styles/setup.css';
import '../styles/ReadySetup.css';

interface ReadySetupProps {
  onLaunch: () => void;
}

type SetupState = 'checking' | 'ready' | 'configuring' | 'error';

export function ReadySetup({ onLaunch }: ReadySetupProps) {
  const [setupState, setSetupState] = useState<SetupState>('checking');
  const [clusterStatus, setClusterStatus] = useState<ClusterStatus | null>(null);
  const [setupProgress, setSetupProgress] = useState<Record<string, SetupProgress>>({});
  const [error, setError] = useState<string | null>(null);

  // Check cluster status on mount
  useEffect(() => {
    checkClusterStatus();
  }, []);

  const checkClusterStatus = async () => {
    try {
      const response = await fetch('http://localhost:8090/api/v1/cluster/status');
      const status: ClusterStatus = await response.json();

      setClusterStatus(status);

      if (status.healthy) {
        setSetupState('ready');
      } else {
        // Initialize progress for missing components
        const progress: Record<string, SetupProgress> = {};
        if (!status.components.knative.installed || !status.components.knative.healthy) {
          progress['knative'] = { component: 'knative', status: 'pending' };
        }
        if (!status.components.traefik.installed || !status.components.traefik.healthy) {
          progress['traefik'] = { component: 'traefik', status: 'pending' };
        }
        if (!status.components.registry.installed || !status.components.registry.healthy) {
          progress['registry'] = { component: 'registry', status: 'pending' };
        }
        if (!status.components.buildkit.installed || !status.components.buildkit.healthy) {
          progress['buildkit'] = { component: 'buildkit', status: 'pending' };
        }
        setSetupProgress(progress);
      }
    } catch (err) {
      console.error('Failed to check cluster status:', err);
      setError('Failed to connect to API server');
      setSetupState('error');
    }
  };

  const configureCluster = async () => {
    setSetupState('configuring');
    setError(null);

    try {
      const eventSource = new EventSource('http://localhost:8090/api/v1/cluster/setup');

      eventSource.addEventListener('progress', (event) => {
        const progress: SetupProgress = JSON.parse(event.data);
        setSetupProgress(prev => ({
          ...prev,
          [progress.component]: progress
        }));
      });

      eventSource.addEventListener('complete', (event) => {
        const data = JSON.parse(event.data);
        console.log('Setup complete:', data);
        eventSource.close();
        setSetupState('ready');
      });

      eventSource.addEventListener('error', (event) => {
        const data = JSON.parse(event.data);
        console.error('Setup error:', data);
        setError(data.error || 'Setup failed');
        eventSource.close();
        setSetupState('error');
      });

      eventSource.onerror = (err) => {
        console.error('EventSource error:', err);
        setError('Connection to API server lost');
        eventSource.close();
        setSetupState('error');
      };
    } catch (err) {
      console.error('Failed to start setup:', err);
      setError('Failed to start cluster setup');
      setSetupState('error');
    }
  };

  const getProgressPercentage = () => {
    const components = Object.values(setupProgress);
    if (components.length === 0) return 100;

    const done = components.filter(c => c.status === 'done').length;
    return Math.round((done / components.length) * 100);
  };

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'done':
        return '✓';
      case 'installing':
        return '⚙️';
      case 'error':
        return '❌';
      default:
        return '⏳';
    }
  };

  const getComponentName = (component: string) => {
    switch (component) {
      case 'knative':
        return 'Knative Serving';
      case 'traefik':
        return 'Traefik Ingress';
      case 'registry':
        return 'Local Registry';
      case 'buildkit':
        return 'BuildKit';
      default:
        return component;
    }
  };

  // Show configuration screen if cluster not ready
  if (setupState === 'checking') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={4} />
        <main className="setup-main">
          <header className="setup-header">
            <h1 className="brand-title">Checking cluster status...</h1>
            <div className="spinner"></div>
          </header>
        </main>
      </div>
    );
  }

  if (setupState === 'configuring' || (setupState !== 'ready' && setupState !== 'error' && clusterStatus && !clusterStatus.healthy)) {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={4} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">4</span>
              <span>Step 4 of 4</span>
            </div>
            <h1 className="brand-title">Configuring cluster...</h1>
            <p className="brand-subtitle">Installing required components</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress={getProgressPercentage()}></div>
            </div>
          </header>

          <div className="setup-configure">
            <div className="configure-components">
              {Object.entries(setupProgress).map(([key, progress]) => (
                <div key={key} className={`component-item status-${progress.status}`}>
                  <span className="component-icon">{getStatusIcon(progress.status)}</span>
                  <div className="component-content">
                    <h4>{getComponentName(progress.component)}</h4>
                    <p>{progress.message || progress.status}</p>
                    {progress.error && <p className="error-text">{progress.error}</p>}
                  </div>
                </div>
              ))}
            </div>

            {setupState !== 'configuring' && (
              <div className="setup-actions">
                <button onClick={configureCluster} className="btn-primary">
                  Configure Cluster
                </button>
              </div>
            )}
          </div>
        </main>
      </div>
    );
  }

  if (setupState === 'error') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={4} />
        <main className="setup-main">
          <header className="setup-header">
            <h1 className="brand-title">Setup Error</h1>
            <p className="error-text">{error}</p>
          </header>
          <div className="setup-actions">
            <button onClick={checkClusterStatus} className="btn-secondary">
              Retry
            </button>
          </div>
        </main>
      </div>
    );
  }

  // Show ready screen when cluster is healthy
  return (
    <div className="setup-container">
      <ProgressSidebar currentStep={4} />
      <main className="setup-main">
        <header className="setup-header">
          <div className="step-badge">
            <span className="step-badge-number">4</span>
            <span>Step 4 of 4</span>
          </div>
          <h1 className="brand-title">You're all set!</h1>
          <p className="brand-subtitle">Ready to launch your first workspace</p>
          <div className="progress-bar-container">
            <div className="progress-bar-fill" data-progress="100"></div>
          </div>
        </header>

        <div className="setup-success">
          <div className="success-icon">✓</div>
          <h2>Setup complete</h2>

          <div className="ready-checklist">
            <div className="checklist-item">
              <span className="checklist-icon">✓</span>
              <div className="checklist-content">
                <h4>Authentication</h4>
                <p>Signed in successfully</p>
              </div>
            </div>

            <div className="checklist-item">
              <span className="checklist-icon">✓</span>
              <div className="checklist-content">
                <h4>Infrastructure</h4>
                <p>Kubernetes cluster ready</p>
              </div>
            </div>

            <div className="checklist-item">
              <span className="checklist-icon">✓</span>
              <div className="checklist-content">
                <h4>Components</h4>
                <p>Knative, Traefik, Registry, BuildKit installed</p>
              </div>
            </div>

            <div className="checklist-item">
              <span className="checklist-icon">✓</span>
              <div className="checklist-content">
                <h4>Configuration</h4>
                <p>Workspace preferences saved</p>
              </div>
            </div>
          </div>

          <div className="ready-info">
            <h3>What's next?</h3>
            <ul className="ready-list">
              <li>Create and manage containerized workspaces</li>
              <li>Choose from pre-built templates (Next.js, Vue, Jupyter)</li>
              <li>Integrate AI coding agents (Claude Code, OpenAI Codex)</li>
              <li>Scale workspaces up or down as needed</li>
            </ul>
          </div>

          <div className="setup-actions">
            <button onClick={onLaunch} className="btn-primary btn-launch">
              Launch workspaces
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
