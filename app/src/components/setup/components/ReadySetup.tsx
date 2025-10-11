import { ProgressSidebar } from './ProgressSidebar';
import '../styles/setup.css';
import '../styles/ReadySetup.css';

interface ReadySetupProps {
  onLaunch: () => void;
}

export function ReadySetup({ onLaunch }: ReadySetupProps) {
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
