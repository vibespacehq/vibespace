import { useState } from 'react';

interface InstallationInstructionsProps {
  suggestedAction?: 'install_kubernetes' | 'start_kubernetes' | 'check_installation';
}

export function InstallationInstructions({ suggestedAction }: InstallationInstructionsProps) {
  const [platform, setPlatform] = useState<'macos' | 'linux' | 'windows'>('macos');

  if (suggestedAction === 'start_kubernetes') {
    return (
      <div className="instructions">
        <h3>Start Kubernetes</h3>
        <p>Your Kubernetes installation was detected but the cluster is not running.</p>

        <div className="code-block">
          <h4>Linux (k3s):</h4>
          <pre>
            <code>sudo systemctl start k3s</code>
          </pre>
        </div>

        <div className="code-block">
          <h4>Rancher Desktop:</h4>
          <ol>
            <li>Open Rancher Desktop</li>
            <li>Go to Preferences → Kubernetes</li>
            <li>Check "Enable Kubernetes"</li>
            <li>Wait for the cluster to start (green indicator)</li>
          </ol>
        </div>
      </div>
    );
  }

  return (
    <div className="instructions">
      <h3>Install Kubernetes</h3>
      <p className="instructions-intro">
        Choose your platform and follow the installation instructions:
      </p>

      <div className="platform-tabs">
        <button
          className={platform === 'macos' ? 'active' : ''}
          onClick={() => setPlatform('macos')}
        >
          macOS
        </button>
        <button
          className={platform === 'linux' ? 'active' : ''}
          onClick={() => setPlatform('linux')}
        >
          Linux
        </button>
        <button
          className={platform === 'windows' ? 'active' : ''}
          onClick={() => setPlatform('windows')}
        >
          Windows
        </button>
      </div>

      <div className="platform-content">
        {platform === 'macos' && (
          <>
            <div className="install-option recommended">
              <h4>
                Option 1: Rancher Desktop <span className="badge">Recommended</span>
              </h4>
              <p>Easiest for beginners - GUI-based k3s management.</p>
              <ol>
                <li>
                  Download from{' '}
                  <a href="https://rancherdesktop.io/" target="_blank" rel="noopener noreferrer">
                    rancherdesktop.io
                  </a>
                </li>
                <li>Install and launch Rancher Desktop</li>
                <li>Enable Kubernetes in settings</li>
                <li>Done! ✅</li>
              </ol>
            </div>

            <div className="install-option">
              <h4>Option 2: k3d (k3s in Docker)</h4>
              <p>Lightweight k3s cluster in Docker containers.</p>
              <div className="code-block">
                <pre>
                  <code>
{`# Install k3d
brew install k3d

# Create a cluster
k3d cluster create mycluster`}
                  </code>
                </pre>
                <button
                  className="copy-btn"
                  onClick={() => copyToClipboard('brew install k3d\nk3d cluster create mycluster')}
                >
                  Copy
                </button>
              </div>
            </div>
          </>
        )}

        {platform === 'linux' && (
          <>
            <div className="install-option recommended">
              <h4>
                Option 1: Native k3s <span className="badge">Recommended</span>
              </h4>
              <div className="code-block">
                <pre>
                  <code>
                    {`curl -sfL https://get.k3s.io | sh -s - \\
  --write-kubeconfig-mode 644 \\
  --disable traefik`}
                  </code>
                </pre>
                <button
                  className="copy-btn"
                  onClick={() =>
                    copyToClipboard(
                      'curl -sfL https://get.k3s.io | sh -s - --write-kubeconfig-mode 644 --disable traefik'
                    )
                  }
                >
                  Copy
                </button>
              </div>
            </div>

            <div className="install-option">
              <h4>Option 2: Rancher Desktop</h4>
              <p>
                Download from{' '}
                <a href="https://rancherdesktop.io/" target="_blank" rel="noopener noreferrer">
                  rancherdesktop.io
                </a>
              </p>
            </div>
          </>
        )}

        {platform === 'windows' && (
          <>
            <div className="install-option recommended">
              <h4>
                Option 1: Rancher Desktop <span className="badge">Recommended</span>
              </h4>
              <p>Easiest option for Windows.</p>
              <ol>
                <li>
                  Download from{' '}
                  <a href="https://rancherdesktop.io/" target="_blank" rel="noopener noreferrer">
                    rancherdesktop.io
                  </a>
                </li>
                <li>Install and launch Rancher Desktop</li>
                <li>Enable Kubernetes in settings</li>
                <li>Done! ✅</li>
              </ol>
            </div>

            <div className="install-option">
              <h4>Option 2: WSL2 + k3s</h4>
              <p>For advanced users.</p>
              <ol>
                <li>
                  Install WSL2:{' '}
                  <code>wsl --install</code>
                </li>
                <li>Inside WSL2, run the Linux k3s installation command</li>
              </ol>
            </div>
          </>
        )}
      </div>

      <div className="help-links">
        <a href="https://docs.k3s.io/" target="_blank" rel="noopener noreferrer">
          k3s Documentation
        </a>
        {' • '}
        <a href="https://docs.rancherdesktop.io/" target="_blank" rel="noopener noreferrer">
          Rancher Desktop Docs
        </a>
      </div>
    </div>
  );
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text).then(() => {
    // TODO: Show toast notification
    console.log('Copied to clipboard');
  });
}
