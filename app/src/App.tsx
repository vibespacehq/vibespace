import { useState } from 'react'
import { TitleBar } from './components/shared/TitleBar'
import { AuthenticationSetup } from './components/setup/components/AuthenticationSetup'
import { KubernetesSetup } from './components/setup/components/KubernetesSetup'
import { ConfigurationSetup, type WorkspaceConfiguration } from './components/setup/components/ConfigurationSetup'
import { WorkspaceList } from './components/workspace/components/WorkspaceList'
import { useWorkspaces } from './hooks/useWorkspaces'

type SetupStep = 'auth' | 'infrastructure' | 'configuration' | 'creating' | 'complete'

function App() {
  const [setupStep, setSetupStep] = useState<SetupStep>('auth')
  const [creatingWorkspace, setCreatingWorkspace] = useState(false)
  const { createWorkspace } = useWorkspaces()

  /**
   * Handles workspace creation from configuration setup.
   * Creates workspace via API and transitions to complete state.
   */
  const handleWorkspaceCreation = async (config: WorkspaceConfiguration) => {
    setSetupStep('creating')
    setCreatingWorkspace(true)

    try {
      await createWorkspace({
        name: config.name,
        template: config.template,
        persistent: true,
        github_repo: config.githubRepo || undefined,
        agent: config.agent || undefined,
      })

      setSetupStep('complete')
    } catch (error) {
      console.error('Failed to create workspace:', error)
      alert(`Failed to create workspace: ${error instanceof Error ? error.message : 'Unknown error'}`)
      setSetupStep('configuration')
    } finally {
      setCreatingWorkspace(false)
    }
  }

  const handleCreateNew = () => {
    setSetupStep('configuration')
  }

  // Setup wizard flow
  if (setupStep === 'auth') {
    return (
      <>
        <TitleBar />
        <AuthenticationSetup onComplete={() => setSetupStep('infrastructure')} />
      </>
    )
  }

  if (setupStep === 'infrastructure') {
    return (
      <>
        <TitleBar />
        <KubernetesSetup
          onComplete={() => setSetupStep('configuration')}
        />
      </>
    )
  }

  if (setupStep === 'configuration') {
    return (
      <>
        <TitleBar />
        <ConfigurationSetup onComplete={handleWorkspaceCreation} />
      </>
    )
  }

  if (setupStep === 'creating') {
    return (
      <>
        <TitleBar />
        <div style={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          height: '100vh',
          gap: '1rem',
          paddingTop: '40px'
        }}>
          <div className="spinner" />
          <p style={{ color: 'var(--text-secondary)' }}>
            {creatingWorkspace ? 'Creating your workspace...' : 'Preparing...'}
          </p>
        </div>
      </>
    )
  }

  // Setup complete - show workspace manager
  return (
    <>
      <TitleBar />
      <WorkspaceList onCreateNew={handleCreateNew} />
    </>
  )
}

export default App
