import { useState, useEffect, useRef } from 'react';
import { AlertCircle } from 'lucide-react';
import { invoke } from '@tauri-apps/api/core';
import { listen } from '@tauri-apps/api/event';
import {
  useKubernetesStatus,
  useKubernetesInstall,
  useKubernetesControl,
} from '../../../hooks/useKubernetesStatus';
import { ProgressSidebar } from './ProgressSidebar';
import { ClusterStatus, SetupProgress } from '../../../lib/types';
import { API_ENDPOINTS, apiFetch } from '../../../lib/api-config';
import '../styles/setup.css';
import '../styles/KubernetesSetup.css';

interface KubernetesSetupProps {
  onComplete?: () => void;
}

type SetupState =
  | 'checking'
  | 'not-installed'
  | 'installing-k8s'
  | 'starting-k8s'
  | 'installing-components'
  | 'installing-dns'
  | 'installing-tls'
  | 'installing-portforward'
  | 'ready'
  | 'error';

/**
 * Infrastructure setup component for bundled Kubernetes installation and configuration.
 *
 * DEPLOYMENT MODE: This component is for LOCAL MODE only, where all components
 * (Tauri app, API server, k8s cluster) run on the user's machine.
 *
 * With ADR 0006, vibespace bundles Kubernetes runtime for Local Mode:
 * - macOS: Colima (lightweight VM) + k3s
 * - Linux: Native k3s installation
 *
 * Handles the complete setup flow:
 * 1. Checks if bundled Kubernetes is installed
 * 2. Installs Colima/k3s if missing (one-click installation)
 * 3. Starts Kubernetes cluster
 * 4. Installs required components (Knative, Traefik, Registry)
 * 5. Configures GHCR image pull secrets
 *
 * For REMOTE MODE (Tauri app on user's machine, API/k8s on VPS), a different
 * setup flow will be implemented in Post-MVP phase. Remote Mode does not install
 * bundled k8s - it configures connection to remote API endpoint.
 *
 * @param props - Component props
 * @param props.onComplete - Callback invoked when setup completes successfully
 *
 * @example
 * ```tsx
 * <KubernetesSetup onComplete={() => navigate('/dashboard')} />
 * ```
 *
 * @see ADR 0006 - Bundled Kubernetes Runtime
 * @public
 */
export function KubernetesSetup({ onComplete }: KubernetesSetupProps) {
  const { status, isLoading, refetch } = useKubernetesStatus();
  const { install: installK8s, isInstalling, progress: k8sProgress, error: k8sError, installComplete } = useKubernetesInstall();
  const { start: startK8s, isStarting } = useKubernetesControl();
  const [setupState, setSetupState] = useState<SetupState>('checking');
  const [setupProgress, setSetupProgress] = useState<Record<string, SetupProgress>>({});
  const [dnsProgress, setDnsProgress] = useState({ stage: '', progress: 0, message: '' });
  const [tlsProgress, setTlsProgress] = useState({ message: '', progress: 0 });
  const [portForwardProgress, setPortForwardProgress] = useState({ message: '', progress: 0 });
  const [error, setError] = useState<string | null>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const installInProgressRef = useRef(false);
  const dnsSetupInProgressRef = useRef(false);
  const dnsUnlistenRef = useRef<(() => void) | null>(null);

  // Update setup state based on Kubernetes status
  useEffect(() => {
    if (isLoading) {
      setSetupState('checking');
      return;
    }

    if (!status) return;

    // Skip external Kubernetes installations (not officially supported)
    if (status.is_external) {
      console.warn('External Kubernetes detected - not officially supported, use bundled k8s');
      return;
    }

    // Handle bundled Kubernetes states
    if (!status.installed) {
      // Don't reset state if we're currently installing
      if (!isInstalling && !installInProgressRef.current && setupState !== 'installing-k8s') {
        setSetupState('not-installed');
        installInProgressRef.current = false;
      }
    } else if (!status.running && !installInProgressRef.current && !isInstalling && !isStarting && setupState !== 'installing-k8s' && setupState !== 'starting-k8s') {
      // Auto-start if installed but not running
      // Skip if we're currently installing or starting (prevents race condition)
      setSetupState('starting-k8s');
      handleStartK8s();
    } else if (status.running) {
      // Kubernetes running, check components
      installInProgressRef.current = false;
      checkClusterComponents();
    }
  }, [status, isLoading, isInstalling, isStarting]);

  // Handle Kubernetes installation errors
  useEffect(() => {
    if (k8sError) {
      setError(k8sError);
      setSetupState('error');
    }
  }, [k8sError]);

  // Handle installation completion
  useEffect(() => {
    if (installComplete) {
      // Installation finished, clear flag and refetch status
      installInProgressRef.current = false;
      refetch();
    }
  }, [installComplete, refetch]);

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

  // Cleanup DNS event listener on component unmount
  useEffect(() => {
    return () => {
      if (dnsUnlistenRef.current) {
        console.log('Cleaning up DNS event listener');
        dnsUnlistenRef.current();
        dnsUnlistenRef.current = null;
      }
    };
  }, []);

  /**
   * Installs bundled Kubernetes (Colima on macOS, k3s on Linux).
   * Progress is tracked via k8sProgress state.
   */
  const handleInstallK8s = async () => {
    try {
      installInProgressRef.current = true;
      setSetupState('installing-k8s');
      setError(null);
      await installK8s();
      // Don't refetch immediately - installation runs in background
      // Status will be checked when installation completes via progress events
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Installation failed';
      console.error('Failed to install Kubernetes:', err);
      setError(message);
      setSetupState('error');
      installInProgressRef.current = false;
    }
  };

  /**
   * Starts bundled Kubernetes cluster.
   */
  const handleStartK8s = async () => {
    try {
      setSetupState('starting-k8s');
      setError(null);
      await startK8s();
      // After starting, refetch status to trigger component check
      await refetch();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Failed to start';
      console.error('Failed to start Kubernetes:', err);
      setError(message);
      setSetupState('error');
    }
  };

  /**
   * Checks cluster health and installed components.
   * Auto-initiates installation if components are missing.
   * @throws {Error} If unable to connect to API server
   */
  const checkClusterComponents = async () => {
    try {
      const clusterStat = await apiFetch<ClusterStatus>(API_ENDPOINTS.clusterStatus);

      if (clusterStat.healthy) {
        setSetupState('ready');
      } else {
        // Components missing, start installation
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
        setSetupProgress(progress);

        // Auto-start component installation
        installComponents();
      }
    } catch (err) {
      console.error('Failed to check cluster status:', err);
      // Don't fail immediately - Kubernetes might still be starting up
      // Retry after a short delay
      setTimeout(checkClusterComponents, 2000);
    }
  };

  /**
   * Installs missing cluster components using Server-Sent Events for progress tracking.
   * Monitors installation progress and handles completion or errors.
   */
  const installComponents = async () => {
    setSetupState('installing-components');
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
        setSetupProgress((prev) => {
          const updated = {
            ...prev,
            [progress.component]: progress,
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
        // After cluster components are installed, setup DNS
        setupDNS();
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

  /**
   * Setup DNS server for *.vibe.space domain resolution.
   * Installs DNS binary, configures OS resolver, and starts DNS server.
   * Requires sudo for OS-level DNS configuration (/etc/resolver on macOS, systemd-resolved on Linux).
   */
  const setupDNS = async () => {
    // Guard: Prevent multiple simultaneous DNS setup calls
    if (dnsSetupInProgressRef.current) {
      console.log('DNS setup already in progress, skipping');
      return;
    }

    dnsSetupInProgressRef.current = true;
    setSetupState('installing-dns');
    setError(null);
    setDnsProgress({ stage: 'starting', progress: 0, message: 'Initializing DNS setup...' });

    try {
      // Cleanup any existing listener before creating new one
      if (dnsUnlistenRef.current) {
        console.log('Cleaning up previous DNS listener');
        dnsUnlistenRef.current();
        dnsUnlistenRef.current = null;
      }

      // Listen for DNS setup progress events
      const unlisten = await listen<{ stage: string; progress: number; message: string }>(
        'dns-setup-progress',
        (event) => {
          console.log('DNS setup progress:', event.payload);
          setDnsProgress(event.payload);

          // Check if DNS setup completed
          if (event.payload.stage === 'complete') {
            console.log('DNS setup completed successfully');
            // Cleanup listener immediately on success
            if (dnsUnlistenRef.current) {
              dnsUnlistenRef.current();
              dnsUnlistenRef.current = null;
            }
            dnsSetupInProgressRef.current = false;
            // Continue to TLS setup
            setupTLS();
          } else if (event.payload.stage === 'error') {
            console.error('DNS setup failed:', event.payload.message);
            setError(`DNS setup failed: ${event.payload.message}`);
            setSetupState('error');
            // Cleanup listener immediately on error
            if (dnsUnlistenRef.current) {
              dnsUnlistenRef.current();
              dnsUnlistenRef.current = null;
            }
            dnsSetupInProgressRef.current = false;
          }
        }
      );

      // Store unlisten function in ref for cleanup on unmount
      dnsUnlistenRef.current = unlisten;

      // Start DNS setup (runs in background thread, emits progress events)
      console.log('Invoking setup_dns command...');
      await invoke('setup_dns');
      console.log('setup_dns command started');

      // Safety timeout: cleanup if no completion event received within 60 seconds
      setTimeout(() => {
        if (dnsUnlistenRef.current) {
          console.warn('DNS setup timeout reached, cleaning up listener');
          dnsUnlistenRef.current();
          dnsUnlistenRef.current = null;
          dnsSetupInProgressRef.current = false;
        }
      }, 60000);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Unknown error';
      console.error('Failed to setup DNS:', err);
      setError(`Failed to setup DNS: ${message}`);
      setSetupState('error');
      // Cleanup listener on exception
      if (dnsUnlistenRef.current) {
        dnsUnlistenRef.current();
        dnsUnlistenRef.current = null;
      }
      dnsSetupInProgressRef.current = false;
    }
  };

  /**
   * Setup TLS certificates using mkcert for locally-trusted HTTPS.
   * Generates wildcard certificate for *.vibe.space domain.
   */
  const setupTLS = async () => {
    setSetupState('installing-tls');
    setError(null);
    setTlsProgress({ message: 'Checking for mkcert...', progress: 10 });

    try {
      console.log('Setting up TLS certificates...');
      setTlsProgress({ message: 'Generating TLS certificates...', progress: 30 });

      // This will generate certs using mkcert (requires mkcert to be installed)
      await invoke('setup_tls_certificates');

      setTlsProgress({ message: 'TLS certificates generated', progress: 100 });
      console.log('TLS setup completed successfully');

      // Continue to port forwarding setup
      setupPortForward();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.warn('TLS setup failed (continuing without HTTPS):', message);
      // TLS is optional - continue to port forwarding even if it fails
      // User can still access via HTTP or with browser cert warnings
      setTlsProgress({ message: 'TLS setup skipped (mkcert not installed)', progress: 100 });
      setupPortForward();
    }
  };

  /**
   * Setup port forwarding from privileged ports to NodePorts.
   * Allows accessing vibespaces via https://project.vibe.space without port number.
   * Uses pfctl on macOS, iptables on Linux.
   */
  const setupPortForward = async () => {
    setSetupState('installing-portforward');
    setError(null);
    setPortForwardProgress({ message: 'Setting up port forwarding...', progress: 30 });

    try {
      console.log('Setting up port forwarding...');

      // This will set up pfctl rules on macOS (requires sudo via osascript)
      await invoke('setup_port_forwarding');

      setPortForwardProgress({ message: 'Port forwarding configured', progress: 100 });
      console.log('Port forwarding setup completed successfully');

      // All setup complete
      setSetupState('ready');
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      console.warn('Port forwarding setup failed (continuing without it):', message);
      // Port forwarding is optional - user can still access with :30443
      setPortForwardProgress({ message: 'Port forwarding skipped', progress: 100 });
      setSetupState('ready');
    }
  };

  const getProgressPercentage = () => {
    const components = Object.values(setupProgress);
    if (components.length === 0) return 0;

    const done = components.filter((c) => c.status === 'done').length;
    return Math.round((done / components.length) * 100);
  };

  const getComponentName = (component: string) => {
    switch (component) {
      case 'knative':
        return 'Knative Serving';
      case 'traefik':
        return 'Traefik Ingress';
      case 'registry':
        return 'Docker Registry';
      default:
        return component;
    }
  };

  const getK8sProgressMessage = () => {
    // During DNS installation, show DNS progress
    if (setupState === 'installing-dns') {
      return dnsProgress.message || 'Setting up DNS server...';
    }

    // During component installation, show component progress
    if (setupState === 'installing-components') {
      const components = Object.values(setupProgress);
      const installing = components.find((c) => c.status === 'installing');
      if (installing) {
        return installing.message || `Installing ${getComponentName(installing.component || '')}...`;
      }
      const pending = components.find((c) => c.status === 'pending');
      if (pending) {
        return `Installing ${getComponentName(pending.component || '')}...`;
      }
      return 'Installing cluster components...';
    }

    // During K8s installation
    if (!k8sProgress) return 'Initializing...';
    return k8sProgress.message;
  };

  const getK8sProgressPercentage = () => {
    // During DNS installation (90-100%)
    if (setupState === 'installing-dns') {
      // Map DNS progress (0-100) to overall progress (90-100)
      return 90 + Math.round(dnsProgress.progress * 0.1);
    }

    // During component installation (60-90%)
    if (setupState === 'installing-components') {
      const percentage = getProgressPercentage();
      // Map component progress (0-100) to overall progress (60-90)
      return 60 + Math.round(percentage * 0.3);
    }

    // During K8s installation (0-60%)
    if (!k8sProgress) return 0;
    // Cap K8s progress at 60% of total
    return Math.min(Math.round(k8sProgress.progress * 0.6), 60);
  };

  // Checking Kubernetes status
  if (setupState === 'checking') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Checking Installation</h1>
            <p className="brand-subtitle">Detecting runtime environment</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="10"></div>
            </div>
          </header>
          <div className="setup-loading">
            <p className="progress-status-text">Checking installation status...</p>
          </div>
        </main>
      </div>
    );
  }

  // Kubernetes not installed
  if (setupState === 'not-installed') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Install vibespace</h1>
            <p className="brand-subtitle">Set up the local runtime environment</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="0"></div>
            </div>
          </header>
          <div className="setup-required">
            <p className="setup-description">
              Create isolated development environments with AI coding assistants.
              Everything runs locally on your machine.
            </p>
            {status?.error && (
              <div className="error-message">
                <span className="error-icon">!</span>
                {status.error}
              </div>
            )}

            <div className="installation-info">
              <p className="installation-note">
                Takes about 2-3 minutes
              </p>
            </div>

            <div className="setup-actions">
              <button onClick={handleInstallK8s} className="btn-primary" disabled={isInstalling}>
                {isInstalling ? 'Installing...' : 'Install vibespace'}
              </button>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Installing Kubernetes
  if (setupState === 'installing-k8s') {
    const percentage = getK8sProgressPercentage();
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Installing vibespace</h1>
            <p className="brand-subtitle">Setting up local runtime environment</p>
            <div className="progress-bar-container">
              <div
                className="progress-bar-fill"
                style={{ width: `${percentage}%` }}
              ></div>
            </div>
          </header>
          <div className="setup-required">
            <div className="install-progress-container">
              <div className="install-progress-header">
                <span className="install-progress-message">{getK8sProgressMessage()}</span>
                <span className="install-progress-percentage">{percentage}%</span>
              </div>
              <div className="install-progress-bar">
                <div
                  className="install-progress-fill"
                  style={{ width: `${percentage}%` }}
                ></div>
              </div>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Starting Kubernetes
  if (setupState === 'starting-k8s') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Starting vibespace</h1>
            <p className="brand-subtitle">Initializing runtime environment</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="20"></div>
            </div>
          </header>
          <div className="setup-loading">
            <p className="progress-status-text">Starting runtime environment...</p>
          </div>
        </main>
      </div>
    );
  }

  // Installing components (continuation of K8s installation)
  if (setupState === 'installing-components') {
    const percentage = getK8sProgressPercentage();
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Installing vibespace</h1>
            <p className="brand-subtitle">Setting up cluster components</p>
            <div className="progress-bar-container">
              <div
                className="progress-bar-fill"
                style={{ width: `${percentage}%` }}
              ></div>
            </div>
          </header>
          <div className="setup-required">
            <div className="install-progress-container">
              <div className="install-progress-header">
                <span className="install-progress-message">{getK8sProgressMessage()}</span>
                <span className="install-progress-percentage">{percentage}%</span>
              </div>
              <div className="install-progress-bar">
                <div
                  className="install-progress-fill"
                  style={{ width: `${percentage}%` }}
                ></div>
              </div>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Installing DNS server
  if (setupState === 'installing-dns') {
    const percentage = getK8sProgressPercentage();
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Installing DNS Server</h1>
            <p className="brand-subtitle">Setting up local domain resolution</p>
            <div className="progress-bar-container">
              <div
                className="progress-bar-fill"
                style={{ width: `${percentage}%` }}
              ></div>
            </div>
          </header>
          <div className="setup-required">
            <div className="install-progress-container">
              <div className="install-progress-header">
                <span className="install-progress-message">{getK8sProgressMessage()}</span>
                <span className="install-progress-percentage">{percentage}%</span>
              </div>
              <div className="install-progress-bar">
                <div
                  className="install-progress-fill"
                  style={{ width: `${percentage}%` }}
                ></div>
              </div>
              <div className="installation-info">
                <p className="installation-note">
                  You may be prompted for administrator access to configure system DNS
                </p>
              </div>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Installing TLS certificates
  if (setupState === 'installing-tls') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Setting up HTTPS</h1>
            <p className="brand-subtitle">Generating TLS certificates</p>
            <div className="progress-bar-container">
              <div
                className="progress-bar-fill"
                style={{ width: '93%' }}
              ></div>
            </div>
          </header>
          <div className="setup-required">
            <div className="install-progress-container">
              <div className="install-progress-header">
                <span className="install-progress-message">{tlsProgress.message}</span>
                <span className="install-progress-percentage">93%</span>
              </div>
              <div className="install-progress-bar">
                <div
                  className="install-progress-fill"
                  style={{ width: '93%' }}
                ></div>
              </div>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Setting up port forwarding
  if (setupState === 'installing-portforward') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={2} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">2</span>
              <span>Step 2 of 4</span>
            </div>
            <h1 className="brand-title">Configuring Network</h1>
            <p className="brand-subtitle">Setting up port forwarding</p>
            <div className="progress-bar-container">
              <div
                className="progress-bar-fill"
                style={{ width: '97%' }}
              ></div>
            </div>
          </header>
          <div className="setup-required">
            <div className="install-progress-container">
              <div className="install-progress-header">
                <span className="install-progress-message">{portForwardProgress.message}</span>
                <span className="install-progress-percentage">97%</span>
              </div>
              <div className="install-progress-bar">
                <div
                  className="install-progress-fill"
                  style={{ width: '97%' }}
                ></div>
              </div>
              <div className="installation-info">
                <p className="installation-note">
                  You may be prompted for administrator access to configure network
                </p>
              </div>
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
            <h1 className="brand-title">vibespace Ready</h1>
            <p className="brand-subtitle">Installation complete</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="100"></div>
            </div>
          </header>
          <div className="setup-success">
            <div className="success-icon">✓</div>
            <h2>Ready to create workspaces</h2>
            <div className="cluster-info">
              <p>vibespace is ready to use. You can now create isolated development environments with AI coding assistants.</p>
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
              <p className="error-state-message">{error || 'Please check your connection and try again.'}</p>
              <button onClick={refetch} className="btn-primary">
                Retry
              </button>
            </div>
          </div>
        </main>
      </div>
    );
  }

  // Default fallback (should never reach here)
  return null;
}
