import { useState } from 'react';
import { ProgressSidebar } from './ProgressSidebar';
import '../styles/setup.css';
import '../styles/ConfigurationSetup.css';

export interface WorkspaceConfiguration {
  name: string;
  template: string;
  agent: string | null;
  githubRepo: string;
}

interface ConfigurationSetupProps {
  onComplete: (config: WorkspaceConfiguration) => void;
}

export function ConfigurationSetup({ onComplete }: ConfigurationSetupProps) {
  const [workspaceName, setWorkspaceName] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('nextjs');
  const [selectedAgent, setSelectedAgent] = useState<string | null>('claude');
  const [githubRepo, setGithubRepo] = useState('');

  const templates = [
    { id: 'nextjs', name: 'Next.js', description: 'React framework for production', logo: '/logos/templates/nextjs.svg' },
    { id: 'vue', name: 'Vue', description: 'Progressive JavaScript framework', logo: '/logos/templates/vue.svg' },
    { id: 'jupyter', name: 'Jupyter', description: 'Interactive Python notebooks', logo: '/logos/templates/jupyter.svg' },
  ];

  const agents = [
    {
      id: 'claude',
      name: 'Claude Code',
      description: 'Anthropic\'s CLI coding agent',
      logo: '/logos/agents/claude.svg',
      configFile: 'CLAUDE.md'
    },
    {
      id: 'codex',
      name: 'OpenAI Codex',
      description: 'Terminal coding assistant',
      logo: '/logos/agents/codex.svg',
      configFile: '.codex'
    },
    {
      id: 'gemini',
      name: 'Gemini CLI',
      description: 'Google\'s AI coding agent',
      logo: '/logos/agents/gemini.svg',
      configFile: '.gemini'
    },
  ];

  const handleContinue = () => {
    if (!workspaceName.trim()) {
      alert('Please enter a workspace name');
      return;
    }

    onComplete({
      name: workspaceName.trim(),
      template: selectedTemplate,
      agent: selectedAgent,
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
          <h1 className="brand-title">Configuration</h1>
          <p className="brand-subtitle">Set up your workspace preferences</p>
          <div className="progress-bar-container">
            <div className="progress-bar-fill" data-progress="50"></div>
          </div>
        </header>

        <div className="setup-required">
          <div className="config-section">
            <h3 className="config-section-title">Workspace name</h3>
            <p className="config-section-description">Choose a name for your first workspace</p>
            <input
              type="text"
              className="config-input"
              placeholder="my-awesome-project"
              value={workspaceName}
              onChange={(e) => setWorkspaceName(e.target.value)}
              aria-label="Workspace name"
            />
          </div>

          <div className="config-section">
            <h3 className="config-section-title">Template</h3>
            <p className="config-section-description">Select a development template</p>
            <div className="template-grid">
              {templates.map((template) => (
                <button
                  key={template.id}
                  className={`template-card ${selectedTemplate === template.id ? 'selected' : ''}`}
                  onClick={() => setSelectedTemplate(template.id)}
                >
                  <img src={template.logo} alt={template.name} className="template-logo" />
                  <h4 className="template-name">{template.name}</h4>
                  <p className="template-description">{template.description}</p>
                </button>
              ))}
            </div>
          </div>

          <div className="config-section">
            <h3 className="config-section-title">AI Coding Agent</h3>
            <p className="config-section-description">Select an AI agent to assist with development</p>
            <div className="template-grid">
              {agents.map((agent) => (
                <button
                  key={agent.id}
                  className={`template-card agent-card ${selectedAgent === agent.id ? 'selected' : ''}`}
                  onClick={() => setSelectedAgent(agent.id)}
                >
                  <img src={agent.logo} alt={agent.name} className="template-logo agent-logo" />
                  <h4 className="template-name">{agent.name}</h4>
                  <p className="template-description">{agent.description}</p>
                  <span className="agent-config-badge">{agent.configFile}</span>
                </button>
              ))}
            </div>
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
            <p className="config-hint">Leave empty to start with a blank workspace</p>
          </div>

          <div className="setup-actions">
            <button onClick={handleContinue} className="btn-primary">
              Continue
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
