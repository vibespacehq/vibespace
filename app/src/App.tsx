import { useState } from 'react'
import { TitleBar } from './components/TitleBar'
import { AuthenticationSetup } from './components/setup/AuthenticationSetup'
import { KubernetesSetup } from './components/setup/KubernetesSetup'
import { ConfigurationSetup } from './components/setup/ConfigurationSetup'
import { ReadySetup } from './components/setup/ReadySetup'
import { WorkspaceList } from './components/workspace/WorkspaceList'
import { useKubernetesStatus } from './hooks/useKubernetesStatus'
import './components/workspace/WorkspaceList.css'

// Mock workspace data for design
const MOCK_WORKSPACES = [
  {
    id: 'ws-1',
    name: 'next-blog',
    template: 'Next.js',
    status: 'running' as const,
    createdAt: '2 hours ago',
  },
  {
    id: 'ws-2',
    name: 'python-ml',
    template: 'Jupyter',
    status: 'stopped' as const,
    createdAt: '1 day ago',
  },
  {
    id: 'ws-3',
    name: 'vue-dashboard',
    template: 'Vue',
    status: 'creating' as const,
    createdAt: 'Just now',
  },
]

type SetupStep = 'auth' | 'infrastructure' | 'configuration' | 'ready' | 'complete'

function App() {
  const { status } = useKubernetesStatus()
  const [workspaces] = useState(MOCK_WORKSPACES)
  const [setupStep, setSetupStep] = useState<SetupStep>('auth')

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
        <ConfigurationSetup onComplete={() => setSetupStep('ready')} />
      </>
    )
  }

  if (setupStep === 'ready') {
    return (
      <>
        <TitleBar />
        <ReadySetup onLaunch={() => setSetupStep('complete')} />
      </>
    )
  }

  // Setup complete - show workspace manager
  const handleCreateNew = () => {
    console.log('Create new workspace')
    // TODO: Open create workspace modal
  }

  return (
    <>
      <TitleBar />
      <WorkspaceList workspaces={workspaces} onCreateNew={handleCreateNew} />
    </>
  )
}

export default App
