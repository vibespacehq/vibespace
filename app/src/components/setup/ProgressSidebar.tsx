interface Step {
  number: number;
  title: string;
  subtitle: string;
  status: 'completed' | 'active' | 'pending';
}

interface ProgressSidebarProps {
  currentStep: number;
}

export function ProgressSidebar({ currentStep }: ProgressSidebarProps) {
  const steps: Step[] = [
    {
      number: 1,
      title: 'Authentication',
      subtitle: 'Sign in complete',
      status: currentStep > 1 ? 'completed' : currentStep === 1 ? 'active' : 'pending',
    },
    {
      number: 2,
      title: 'Infrastructure',
      subtitle:
        currentStep > 2
          ? 'Environment ready'
          : currentStep === 2
          ? 'Setting up environment'
          : 'Checking environment',
      status: currentStep > 2 ? 'completed' : currentStep === 2 ? 'active' : 'pending',
    },
    {
      number: 3,
      title: 'Configuration',
      subtitle: 'workspace settings',
      status: currentStep > 3 ? 'completed' : currentStep === 3 ? 'active' : 'pending',
    },
    {
      number: 4,
      title: 'Ready',
      subtitle: 'Launch workspace',
      status: currentStep >= 4 ? 'active' : 'pending',
    },
  ];

  return (
    <aside className="setup-sidebar">
      <div className="sidebar-logo">
        <img src="/src-tauri/icons/icon-transparent.png" alt="workspaces" className="sidebar-icon" />
        <p>setup</p>
      </div>
      <div className="progress-steps">
        {steps.map((step) => (
          <div key={step.number} className={`progress-step ${step.status}`}>
            <div className="step-indicator">{step.number}</div>
            <div className="step-content">
              <h3>{step.title}</h3>
              <p>{step.subtitle}</p>
            </div>
          </div>
        ))}
      </div>
    </aside>
  );
}
