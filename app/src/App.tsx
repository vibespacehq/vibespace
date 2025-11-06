import { useState } from 'react'
import { TitleBar } from './components/shared/TitleBar'
import { AuthenticationSetup } from './components/setup/components/AuthenticationSetup'
import { KubernetesSetup } from './components/setup/components/KubernetesSetup'
import { ConfigurationSetup, type VibespaceConfiguration } from './components/setup/components/ConfigurationSetup'
import { VibespaceList } from './components/vibespace/components/VibespaceList'
import { useVibespaces } from './hooks/useVibespaces'

type SetupStep = 'auth' | 'infrastructure' | 'configuration' | 'creating' | 'complete'

function App() {
  const [setupStep, setSetupStep] = useState<SetupStep>('auth')
  const [creatingVibespace, setCreatingVibespace] = useState(false)
  const { createVibespace } = useVibespaces()

  /**
   * Handles vibespace creation from configuration setup.
   * Creates vibespace via API and transitions to complete state.
   */
  const handleVibespaceCreation = async (config: VibespaceConfiguration) => {
    setSetupStep('creating')
    setCreatingVibespace(true)

    try {
      await createVibespace({
        name: config.name,
        template: config.template,
        persistent: true,
        github_repo: config.githubRepo || undefined,
        agent: config.agent || undefined,
      })

      setSetupStep('complete')
    } catch (error) {
      console.error('Failed to create vibespace:', error)
      alert(`Failed to create vibespace: ${error instanceof Error ? error.message : 'Unknown error'}`)
      setSetupStep('configuration')
    } finally {
      setCreatingVibespace(false)
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
        <ConfigurationSetup onComplete={handleVibespaceCreation} />
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
            {creatingVibespace ? 'Creating your vibespace...' : 'Preparing...'}
          </p>
        </div>
      </>
    )
  }

  // Setup complete - show vibespace manager
  return (
    <>
      <TitleBar />
      <VibespaceList onCreateNew={handleCreateNew} />
    </>
  )
}

export default App
