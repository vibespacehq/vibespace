import { useState } from 'react';
import { ProgressSidebar } from './ProgressSidebar';
import '../styles/setup.css';
import '../styles/ConfigurationSetup.css';

export interface VibespaceConfiguration {
  name: string;
  githubRepo: string;
}

interface ConfigurationSetupProps {
  onComplete: (config: VibespaceConfiguration) => void;
}

/**
 * Configuration setup component for creating a new vibespace.
 *
 * With the simplified architecture (Multi-Claude + ttyd), this component
 * only needs to collect:
 * - Vibespace name
 * - Optional GitHub repository to clone
 *
 * Templates and agent selection have been removed since all vibespaces
 * now use the same unified image with Claude Code built-in.
 */
export function ConfigurationSetup({ onComplete }: ConfigurationSetupProps) {
  const [vibespaceName, setVibespaceName] = useState('');
  const [githubRepo, setGithubRepo] = useState('');

  const handleContinue = () => {
    if (!vibespaceName.trim()) {
      alert('Please enter a vibespace name');
      return;
    }

    onComplete({
      name: vibespaceName.trim(),
      githubRepo: githubRepo.trim(),
    });
  };

  return (
    <div className="setup-container">
      <ProgressSidebar currentStep={3} />
      <main className="setup-main">
        <header className="setup-header">
          <div className="step-badge">
            <span className="step-badge-number">3</span>
            <span>Step 3 of 4</span>
          </div>
          <h1 className="brand-title">Create Vibespace</h1>
          <p className="brand-subtitle">Set up your development environment</p>
          <div className="progress-bar-container">
            <div className="progress-bar-fill" data-progress="50"></div>
          </div>
        </header>

        <div className="setup-required">
          <div className="config-section">
            <h3 className="config-section-title">Vibespace name</h3>
            <p className="config-section-description">Choose a name for your vibespace</p>
            <input
              type="text"
              className="config-input"
              placeholder="my-awesome-project"
              value={vibespaceName}
              onChange={(e) => setVibespaceName(e.target.value)}
              aria-label="Vibespace name"
            />
          </div>

          <div className="config-section">
            <h3 className="config-section-title">GitHub Repository (Optional)</h3>
            <p className="config-section-description">Clone an existing repository or start fresh</p>
            <input
              type="text"
              className="config-input"
              placeholder="https://github.com/username/repo or username/repo"
              value={githubRepo}
              onChange={(e) => setGithubRepo(e.target.value)}
              aria-label="GitHub repository URL (optional)"
            />
            <p className="config-hint">Leave empty to start with a blank vibespace</p>
          </div>

          <div className="config-info">
            <div className="config-info-icon">
              <img src="/logos/agents/claude.svg" alt="Claude Code" className="info-logo" />
            </div>
            <div className="config-info-text">
              <h4>Claude Code Built-in</h4>
              <p>Every vibespace includes Claude Code for AI-assisted development via ttyd terminal.</p>
            </div>
          </div>

          <div className="setup-actions">
            <button onClick={handleContinue} className="btn-primary">
              Create Vibespace
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
