import { useState } from 'react';
import { ProgressSidebar } from './ProgressSidebar';
import '../styles/setup.css';
import '../styles/ConfigurationSetup.css';

interface ConfigurationSetupProps {
  onComplete: () => void;
}

export function ConfigurationSetup({ onComplete }: ConfigurationSetupProps) {
  const [workspaceName, setWorkspaceName] = useState('');
  const [selectedTemplate, setSelectedTemplate] = useState('nextjs');

  const templates = [
    { id: 'nextjs', name: 'Next.js', description: 'React framework for production' },
    { id: 'vue', name: 'Vue', description: 'Progressive JavaScript framework' },
    { id: 'jupyter', name: 'Jupyter', description: 'Interactive Python notebooks' },
    { id: 'go', name: 'Go', description: 'Fast and efficient backend' },
  ];

  const handleContinue = () => {
    onComplete();
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
                  <div className="template-icon">◇</div>
                  <h4 className="template-name">{template.name}</h4>
                  <p className="template-description">{template.description}</p>
                </button>
              ))}
            </div>
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
