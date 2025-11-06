import { useState, useEffect, useRef } from 'react';
import { AlertCircle } from 'lucide-react';
import { useKubernetesStatus } from '../../../hooks/useKubernetesStatus';
import { InstallationInstructions } from './InstallationInstructions';
import { ProgressSidebar } from './ProgressSidebar';
import { ClusterStatus, SetupProgress, ClusterContext } from '../../../lib/types';
import { API_ENDPOINTS, apiFetch } from '../../../lib/api-config';
import '../styles/setup.css';
import '../styles/KubernetesSetup.css';

interface KubernetesSetupProps {
  onComplete?: () => void;
}

type SetupState = 'detecting' | 'not-found' | 'selecting-cluster' | 'found' | 'installing' | 'ready' | 'error';

/**
 * Infrastructure setup component for Kubernetes cluster detection and component installation.
 *
 * Handles the complete cluster setup flow:
 * 1. Detects Kubernetes installation (k3s, Rancher Desktop, k3d, etc.)
 * 2. Allows user to select cluster context
 * 3. Checks for required components (Knative, Traefik, Registry, BuildKit)
 * 4. Installs missing components with real-time progress via Server-Sent Events
 * 5. Builds all vibespace images (12 images total)
 *
 * @param props - Component props
 * @param props.onComplete - Callback invoked when setup completes successfully
 *
 * @example
 * ```tsx
 * <KubernetesSetup onComplete={() => navigate('/dashboard')} />
 * ```
 *
 * @public
 */
export function KubernetesSetup({ onComplete }: KubernetesSetupProps) {
  const { status, isLoading, refetch } = useKubernetesStatus();
  const [setupState, setSetupState] = useState<SetupState>('detecting');
  const [clusterStatus, setClusterStatus] = useState<ClusterStatus | null>(null);
  const [setupProgress, setSetupProgress] = useState<Record<string, SetupProgress>>({});
  const [error, setError] = useState<string | null>(null);
  const [contexts, setContexts] = useState<ClusterContext[]>([]);
  const [selectedContext, setSelectedContext] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [isSwitchingContext, setIsSwitchingContext] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);

  // Fetch available contexts when Kubernetes is detected
  useEffect(() => {
    if (status?.available && setupState === 'detecting') {
      fetchContexts();
    } else if (!isLoading && !status?.available) {
      setSetupState('not-found');
    }
  }, [status, isLoading]);

  // Cleanup EventSource on component unmount
  useEffect(() => {
    return () => {
      if (eventSourceRef.current) {
        console.log('Cleaning up EventSource');
        eventSourceRef.current.close();
        eventSourceRef.current = null;
      }
    };
  }, []);

  /**
   * Fetches available Kubernetes contexts from the API.
   * Displays cluster selection screen once contexts are loaded.
   */
  const fetchContexts = async () => {
    try {
      const data = await apiFetch<{ contexts: ClusterContext[] }>(API_ENDPOINTS.clusterContexts);

      setContexts(data.contexts || []);

      // Don't pre-select any context - force user to choose
      setSelectedContext(null);

      // Always show cluster selection
      setSetupState('selecting-cluster');
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      console.error('Failed to fetch contexts:', err);
      setError(`Failed to load cluster contexts: ${message}`);
      setSetupState('error');
    }
  };

  /**
   * Switches the active Kubernetes context.
   * @param contextName - Name of the context to switch to
   */
  const switchContext = async (contextName: string) => {
    try {
      setIsSwitchingContext(true);
      await apiFetch(API_ENDPOINTS.clusterContextSwitch(contextName), {
        method: 'POST',
      });

      setSelectedContext(contextName);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      console.error('Failed to switch context:', err);
      setError(`Failed to switch cluster context: ${message}`);
      setSetupState('error');
    } finally {
      setIsSwitchingContext(false);
    }
  };

  const proceedWithSelectedCluster = async () => {
    if (!selectedContext) {
      return;
    }

    // Switch to selected context if it's not current
    const current = contexts.find(ctx => ctx.is_current);
    if (current?.name !== selectedContext) {
      await switchContext(selectedContext);
    }

    // Proceed to component check
    checkClusterComponents();
  };

  /**
   * Checks cluster health and installed components.
   * Auto-initiates installation if components are missing.
   * @throws {Error} If unable to connect to API server
   */
  const checkClusterComponents = async () => {
    try {
      const clusterStat = await apiFetch<ClusterStatus>(API_ENDPOINTS.clusterStatus);

      setClusterStatus(clusterStat);

      if (clusterStat.healthy) {
        setSetupState('ready');
      } else {
        // Components missing, start installation
        setSetupState('found');
        const progress: Record<string, SetupProgress> = {};
        if (!clusterStat.components.knative.installed || !clusterStat.components.knative.healthy) {
          progress['knative'] = { component: 'knative', status: 'pending' };
        }
        if (!clusterStat.components.traefik.installed || !clusterStat.components.traefik.healthy) {
          progress['traefik'] = { component: 'traefik', status: 'pending' };
        }
        if (!clusterStat.components.registry.installed || !clusterStat.components.registry.healthy) {
          progress['registry'] = { component: 'registry', status: 'pending' };
        }
        if (!clusterStat.components.buildkit.installed || !clusterStat.components.buildkit.healthy) {
          progress['buildkit'] = { component: 'buildkit', status: 'pending' };
        }
        setSetupProgress(progress);

        // Auto-start installation
        installComponents();
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      console.error('Failed to check cluster status:', err);
      setError(`Failed to connect to API server: ${message}`);
      setSetupState('error');
    }
  };

  /**
   * Installs missing cluster components using Server-Sent Events for progress tracking.
   * Monitors installation progress and handles completion or errors.
   */
  const installComponents = async () => {
    setSetupState('installing');
    setError(null);

    try {
      console.log('Creating EventSource:', API_ENDPOINTS.clusterSetup);
      const eventSource = new EventSource(API_ENDPOINTS.clusterSetup);
      eventSourceRef.current = eventSource;

      console.log('EventSource created, readyState:', eventSource.readyState);

      eventSource.onopen = () => {
        console.log('EventSource connection opened');
      };

      eventSource.addEventListener('progress', (event) => {
        console.log('Progress event received:', event.data);
        const progress: SetupProgress = JSON.parse(event.data);
        console.log('Parsed progress:', progress);
        setSetupProgress(prev => {
          const updated = {
            ...prev,
            [progress.component]: progress
          };
          console.log('Updated setupProgress:', updated);
          return updated;
        });
      });

      eventSource.addEventListener('complete', (event) => {
        console.log('Complete event received:', event.data);
        console.log('Setup complete:', JSON.parse(event.data));
        eventSource.close();
        eventSourceRef.current = null;
        setSetupState('ready');
      });

      eventSource.addEventListener('error', (event) => {
        console.log('Error event received:', event);
        const messageEvent = event as MessageEvent;
        const data = JSON.parse(messageEvent.data);
        console.error('Setup error:', data);
        setError(data.error || 'Setup failed');
        eventSource.close();
        eventSourceRef.current = null;
        setSetupState('error');
      });

      eventSource.onerror = async (err) => {
        console.error('EventSource onerror triggered:', err);
        console.error('EventSource readyState:', eventSource.readyState);
        eventSource.close();
        eventSourceRef.current = null;

        // Check cluster status to determine if setup completed successfully
        try {
          console.log('Checking cluster status after error...');
          const clusterStat = await apiFetch<ClusterStatus>(API_ENDPOINTS.clusterStatus);
          console.log('Cluster status:', clusterStat);

          if (clusterStat.healthy) {
            console.log('Cluster is healthy, marking as ready');
            setSetupState('ready');
          } else {
            console.log('Cluster is not healthy');
            setError('Installation incomplete. Please retry.');
            setSetupState('error');
          }
        } catch (fetchErr) {
          const message = fetchErr instanceof Error ? fetchErr.message : 'Unknown error';
          console.error('Failed to check cluster status:', fetchErr);
          setError(`Connection to API server lost: ${message}`);
          setSetupState('error');
        }
      };
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      console.error('Failed to start setup:', err);
      setError(`Failed to start cluster setup: ${message}`);
      setSetupState('error');
    }
  };

  const getProgressPercentage = () => {
    const components = Object.values(setupProgress);
    if (components.length === 0) return 0;

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

  // Detecting Kubernetes
  if (setupState === 'detecting' || isLoading) {
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
              <div className="progress-bar-fill" data-progress="10"></div>
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

  // Installing components
  if (setupState === 'installing' || setupState === 'found') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Setting up infrastructure</h1>
            <p className="brand-subtitle">Installing required components</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress={10 + getProgressPercentage() * 0.9}></div>
            </div>
          </header>

          <div className="setup-configure">
            <div className="configure-components">
              {Object.entries(setupProgress).map(([key, progress]) => (
                <div key={key} className={`component-item status-${progress.status}`}>
                  <div className="component-item-header">
                    <div className="component-name">
                      <span>{getStatusIcon(progress.status)}</span>
                      <span>{getComponentName(progress.component)}</span>
                    </div>
                    <span className="component-status-text">
                      {progress.status === 'installing' ? 'Installing' : progress.status === 'done' ? 'Complete' : 'Pending'}
                    </span>
                  </div>
                  <div className="component-progress-bar">
                    <div className="component-progress-fill"></div>
                  </div>
                  {progress.message && progress.status === 'installing' && (
                    <div className="component-message">{progress.message}</div>
                  )}
                  {progress.error && (
                    <div className="component-error">
                      <span>❌</span>
                      <span>{progress.error}</span>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Installation complete - components ready
  if (setupState === 'ready') {
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
            <p className="brand-subtitle">All components installed successfully</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="100"></div>
            </div>
          </header>
          <div className="setup-success">
            <div className="success-icon">✓</div>
            <h2>Infrastructure ready</h2>
            <div className="cluster-info">
              {status?.installType && (
                <p>
                  <strong>Type:</strong>
                  <span>{status.installType}</span>
                </p>
              )}
              {status?.version && (
                <p>
                  <strong>Version:</strong>
                  <span>{status.version}</span>
                </p>
              )}
              {clusterStatus && (
                <div className="component-list">
                  <div className="component-badges">
                    <div className="component-badge">
                      <span className="component-badge-icon">✓</span>
                      <span>Knative Serving</span>
                    </div>
                    <div className="component-badge">
                      <span className="component-badge-icon">✓</span>
                      <span>Traefik Ingress</span>
                    </div>
                    <div className="component-badge">
                      <span className="component-badge-icon">✓</span>
                      <span>Local Registry</span>
                    </div>
                    <div className="component-badge">
                      <span className="component-badge-icon">✓</span>
                      <span>BuildKit</span>
                    </div>
                  </div>
                </div>
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

  // Cluster selection
  if (setupState === 'selecting-cluster') {
    const selectedCtx = contexts.find(ctx => ctx.name === selectedContext);
    const showWarning = selectedCtx && !selectedCtx.is_local;

    // Filter contexts by search query
    const filteredContexts = contexts.filter(ctx =>
      ctx.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      ctx.cluster.toLowerCase().includes(searchQuery.toLowerCase()) ||
      ctx.user.toLowerCase().includes(searchQuery.toLowerCase())
    );

    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Select Cluster</h1>
            <p className="brand-subtitle">Choose which Kubernetes cluster to use</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="15"></div>
            </div>
          </header>

          <div className="setup-configure">
            <div className="cluster-selection-grid">
              {contexts.length > 3 && (
                <div className="cluster-search">
                  <input
                    type="text"
                    placeholder="Search clusters..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    aria-label="Search Kubernetes clusters"
                  />
                </div>
              )}

              <div className="cluster-grid">
                {filteredContexts.length > 0 ? (
                  filteredContexts.map((ctx) => (
                    <button
                      key={ctx.name}
                      className={`cluster-card ${selectedContext === ctx.name ? 'selected' : ''} ${!ctx.is_local ? 'remote' : ''}`}
                      onClick={() => setSelectedContext(ctx.name)}
                      aria-label={`Select ${ctx.name} cluster${!ctx.is_local ? ' (remote)' : ''}`}
                      aria-pressed={selectedContext === ctx.name}
                    >
                      <div className="cluster-card-header">
                        <span className="cluster-card-name">{ctx.name}</span>
                        <div className="cluster-card-badges">
                          {ctx.is_current && <span className="cluster-badge current">Current</span>}
                          {ctx.is_local && <span className="cluster-badge local">Local</span>}
                          {!ctx.is_local && <span className="cluster-badge remote">Remote</span>}
                        </div>
                      </div>
                      <div className="cluster-card-details">
                        <div className="cluster-card-info">
                          <span className="cluster-card-label">Cluster:</span>
                          <span>{ctx.cluster}</span>
                        </div>
                        <div className="cluster-card-info">
                          <span className="cluster-card-label">User:</span>
                          <span>{ctx.user}</span>
                        </div>
                      </div>
                      {selectedContext === ctx.name && (
                        <div className="cluster-card-selected-indicator">✓</div>
                      )}
                    </button>
                  ))
                ) : (
                  <div className="cluster-empty">No clusters found</div>
                )}
              </div>

              {showWarning && (
                <div className="cluster-warning">
                  <span>⚠️</span>
                  <span>This appears to be a remote cluster. Installing components may affect production workloads.</span>
                </div>
              )}

              <div className="setup-actions">
                <button
                  onClick={proceedWithSelectedCluster}
                  className="btn-primary"
                  disabled={!selectedContext || isSwitchingContext}
                  aria-busy={isSwitchingContext}
                >
                  {isSwitchingContext ? 'Switching...' : 'Continue'}
                </button>
              </div>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Error state
  if (setupState === 'error') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Setup Error</h1>
            <p className="brand-subtitle">{error}</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="0"></div>
            </div>
          </header>
          <div className="setup-loading">
            <div className="error-state-content">
              <div className="error-icon-container">
                <AlertCircle size={48} strokeWidth={2} />
              </div>
              <h3 className="error-state-title">Something went wrong</h3>
              <p className="error-state-message">
                Please check your connection and try again.
              </p>
              <button onClick={checkClusterComponents} className="btn-primary">
                Retry
              </button>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Kubernetes not found
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
            <div className="progress-bar-fill" data-progress="0"></div>
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
