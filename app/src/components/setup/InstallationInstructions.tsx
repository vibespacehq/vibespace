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
              <p>✅ Works on macOS - GUI-based with k3s built-in</p>
              <ol>
                <li>
                  Download from{' '}
                  <a href="https://rancherdesktop.io/" target="_blank" rel="noopener noreferrer">
                    rancherdesktop.io
                  </a>
                </li>
                <li>Install and launch Rancher Desktop</li>
                <li>Go to Preferences → Kubernetes</li>
                <li>Check "Enable Kubernetes"</li>
                <li>Wait for cluster to start (green indicator)</li>
                <li>Done! ✅</li>
              </ol>
            </div>

            <div className="install-option">
              <h4>Option 2: k3d (k3s in Docker)</h4>
              <p>✅ Works on macOS - Runs k3s inside Docker containers</p>
              <div className="code-block">
                <h4>Prerequisites: Docker Desktop must be installed</h4>
                <pre>
                  <code>
{`# Install Docker Desktop first (if not installed)
# Download from: https://www.docker.com/products/docker-desktop

# Install k3d via Homebrew
brew install k3d

# Create a k3s cluster
k3d cluster create mycluster

# Verify cluster is running
kubectl get nodes`}
                  </code>
                </pre>
                <button
                  className="copy-btn"
                  onClick={() =>
                    copyToClipboard(
                      'brew install k3d\nk3d cluster create mycluster\nkubectl get nodes'
                    )
                  }
                >
                  Copy
                </button>
              </div>
            </div>

            <div className="install-option">
              <h4>Option 3: Docker Desktop Kubernetes</h4>
              <p>✅ Works on macOS - If you already have Docker Desktop</p>
              <ol>
                <li>Open Docker Desktop</li>
                <li>Go to Settings → Kubernetes</li>
                <li>Check "Enable Kubernetes"</li>
                <li>Click "Apply & Restart"</li>
                <li>Done! ✅</li>
              </ol>
            </div>
          </>
        )}

        {platform === 'linux' && (
          <>
            <div className="install-option recommended">
              <h4>
                Option 1: Native k3s <span className="badge">Recommended</span>
              </h4>
              <p>✅ Works on Linux - Lightweight and production-ready</p>
              <div className="code-block">
                <pre>
                  <code>
                    {`# Install k3s (requires sudo)
curl -sfL https://get.k3s.io | sh -s - \\
  --write-kubeconfig-mode 644 \\
  --disable traefik

# Verify installation
sudo systemctl status k3s
kubectl get nodes`}
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
              <h4>Option 2: k3d (k3s in Docker)</h4>
              <p>✅ Works on Linux - Easy to reset and manage multiple clusters</p>
              <div className="code-block">
                <h4>Prerequisites: Docker must be installed</h4>
                <pre>
                  <code>
{`# Install Docker first (if not installed)
sudo apt-get install docker.io  # Ubuntu/Debian
# or
sudo dnf install docker  # Fedora/RHEL

# Install k3d
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Create a cluster
k3d cluster create mycluster

# Verify cluster
kubectl get nodes`}
                  </code>
                </pre>
                <button
                  className="copy-btn"
                  onClick={() =>
                    copyToClipboard(
                      'curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash\nk3d cluster create mycluster\nkubectl get nodes'
                    )
                  }
                >
                  Copy
                </button>
              </div>
            </div>

            <div className="install-option">
              <h4>Option 3: Rancher Desktop</h4>
              <p>✅ Works on Linux - GUI-based management</p>
              <ol>
                <li>
                  Download .deb or .rpm from{' '}
                  <a href="https://rancherdesktop.io/" target="_blank" rel="noopener noreferrer">
                    rancherdesktop.io
                  </a>
                </li>
                <li>Install: <code>sudo dpkg -i rancher-desktop*.deb</code> or <code>sudo rpm -i rancher-desktop*.rpm</code></li>
                <li>Launch Rancher Desktop and enable Kubernetes</li>
              </ol>
            </div>
          </>
        )}

        {platform === 'windows' && (
          <>
            <div className="install-option recommended">
              <h4>
                Option 1: Rancher Desktop <span className="badge">Recommended</span>
              </h4>
              <p>✅ Works on Windows - Easiest option with GUI</p>
              <ol>
                <li>
                  Download installer from{' '}
                  <a href="https://rancherdesktop.io/" target="_blank" rel="noopener noreferrer">
                    rancherdesktop.io
                  </a>
                </li>
                <li>Run the installer (.exe)</li>
                <li>Launch Rancher Desktop</li>
                <li>Go to Preferences → Kubernetes</li>
                <li>Check "Enable Kubernetes"</li>
                <li>Wait for cluster to start</li>
                <li>Done! ✅</li>
              </ol>
            </div>

            <div className="install-option">
              <h4>Option 2: Docker Desktop Kubernetes</h4>
              <p>✅ Works on Windows - If you already have Docker Desktop</p>
              <ol>
                <li>Open Docker Desktop</li>
                <li>Go to Settings → Kubernetes</li>
                <li>Check "Enable Kubernetes"</li>
                <li>Click "Apply & Restart"</li>
                <li>Done! ✅</li>
              </ol>
            </div>

            <div className="install-option">
              <h4>Option 3: WSL2 + k3s (Advanced)</h4>
              <p>✅ Works on Windows - For developers comfortable with Linux</p>
              <div className="code-block">
                <h4>In PowerShell (as Administrator):</h4>
                <pre>
                  <code>
{`# Enable WSL2
wsl --install

# Restart computer, then open Ubuntu from Start Menu`}
                  </code>
                </pre>
              </div>
              <div className="code-block">
                <h4>Inside WSL2 Ubuntu:</h4>
                <pre>
                  <code>
{`# Install k3s
curl -sfL https://get.k3s.io | sh -s - \\
  --write-kubeconfig-mode 644 \\
  --disable traefik

# Verify
kubectl get nodes`}
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
