// WIP: CreateWorkspace component for Phase 1 workspace creation flow
// This component will be integrated in a future PR
// Currently excluded from knip checks via knip.config.ts

import { useState } from 'react';
import { ProgressSidebar } from './ProgressSidebar';
import '../styles/setup.css';
import '../styles/CreateWorkspace.css';

interface CreateWorkspaceProps {
  onComplete: (workspaceId: string) => void;
}

type CreationState = 'selecting' | 'creating' | 'ready' | 'error';

const TEMPLATES = [
  {
    id: 'nextjs',
    name: 'Next.js',
    description: 'React framework with TypeScript, Tailwind CSS',
    icon: '⚛️',
    recommended: true,
  },
  {
    id: 'vue',
    name: 'Vue 3',
    description: 'Progressive JavaScript framework with Vite',
    icon: '💚',
  },
  {
    id: 'jupyter',
    name: 'Jupyter',
    description: 'Python data science notebook environment',
    icon: '📊',
  },
];

export function CreateWorkspace({ onComplete }: CreateWorkspaceProps) {
  const [creationState, setCreationState] = useState<CreationState>('selecting');
  const [selectedTemplate, setSelectedTemplate] = useState('nextjs');
  const [workspaceName, setWorkspaceName] = useState('my-workspace');
  const [createdWorkspaceId, setCreatedWorkspaceId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const createWorkspace = async () => {
    setCreationState('creating');
    setError(null);

    try {
      const response = await fetch('http://localhost:8090/api/v1/workspaces', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: workspaceName,
          template: selectedTemplate,
        }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || 'Failed to create workspace');
      }

      const workspace = await response.json();
      setCreatedWorkspaceId(workspace.id);
      setCreationState('ready');
    } catch (err) {
      console.error('Failed to create workspace:', err);
      setError(err instanceof Error ? err.message : 'Failed to create workspace');
      setCreationState('error');
    }
  };

  if (creationState === 'creating') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={4} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">4</span>
              <span>Step 4 of 4</span>
            </div>
            <h1 className="brand-title">Creating workspace...</h1>
            <p className="brand-subtitle">Setting up your development environment</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="50"></div>
            </div>
          </header>
          <div className="setup-loading">
            <div className="spinner" />
            <p>Building container image and starting services...</p>
          </div>
        </main>
      </div>
    );
  }

  if (creationState === 'ready' && createdWorkspaceId) {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={4} />
        <main className="setup-main">
          <header className="setup-header">
            <div className="step-badge">
              <span className="step-badge-number">4</span>
              <span>Step 4 of 4</span>
            </div>
            <h1 className="brand-title">Workspace ready!</h1>
            <p className="brand-subtitle">Your development environment is ready to use</p>
            <div className="progress-bar-container">
              <div className="progress-bar-fill" data-progress="100"></div>
            </div>
          </header>

          <div className="setup-success">
            <div className="success-icon">✓</div>
            <h2>Setup complete</h2>

            <div className="workspace-created-info">
              <p>
                <strong>Workspace:</strong> {workspaceName}
              </p>
              <p>
                <strong>Template:</strong> {TEMPLATES.find(t => t.id === selectedTemplate)?.name}
              </p>
            </div>

            <div className="ready-info">
              <h3>What's next?</h3>
              <ul className="ready-list">
                <li>Open your workspace to start coding</li>
                <li>Create additional workspaces for different projects</li>
                <li>Integrate AI coding agents (Claude Code, OpenAI Codex)</li>
                <li>Scale workspaces up or down as needed</li>
              </ul>
            </div>

            <div className="setup-actions">
              <button onClick={() => onComplete(createdWorkspaceId)} className="btn-primary btn-launch">
                Open Workspace
              </button>
            </div>
          </div>
        </main>
      </div>
    );
  }

  if (creationState === 'error') {
    return (
      <div className="setup-container">
        <ProgressSidebar currentStep={4} />
        <main className="setup-main">
          <header className="setup-header">
            <h1 className="brand-title">Creation Error</h1>
            <p className="error-text">{error}</p>
          </header>
          <div className="setup-actions">
            <button onClick={createWorkspace} className="btn-secondary">
              Retry
            </button>
          </div>
        </main>
      </div>
    );
  }

  // Template selection
  return (
    <div className="setup-container">
      <ProgressSidebar currentStep={4} />
      <main className="setup-main">
        <header className="setup-header">
          <div className="step-badge">
            <span className="step-badge-number">4</span>
            <span>Step 4 of 4</span>
          </div>
          <h1 className="brand-title">Create your first workspace</h1>
          <p className="brand-subtitle">Choose a template to get started</p>
          <div className="progress-bar-container">
            <div className="progress-bar-fill" data-progress="75"></div>
          </div>
        </header>

        <div className="create-workspace-form">
          <div className="form-group">
            <label htmlFor="workspace-name">Workspace name</label>
            <input
              id="workspace-name"
              type="text"
              value={workspaceName}
              onChange={(e) => setWorkspaceName(e.target.value)}
              className="form-input"
              placeholder="my-workspace"
            />
          </div>

          <div className="form-group">
            <label>Select template</label>
            <div className="template-grid">
              {TEMPLATES.map((template) => (
                <div
                  key={template.id}
                  className={`template-card ${selectedTemplate === template.id ? 'selected' : ''}`}
                  onClick={() => setSelectedTemplate(template.id)}
                >
                  <div className="template-icon">{template.icon}</div>
                  <h3>{template.name}</h3>
                  {template.recommended && <span className="template-badge">Recommended</span>}
                  <p>{template.description}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="setup-actions">
            <button onClick={createWorkspace} className="btn-primary">
              Create Workspace
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
