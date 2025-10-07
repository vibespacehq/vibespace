import { useState } from 'react'

function App() {
  const [workspaces, setWorkspaces] = useState([])

  return (
    <div className="min-h-screen bg-bg-primary text-text-primary">
      <div className="container mx-auto p-8">
        <header className="mb-12">
          <h1 className="text-4xl font-bold mb-2">Workspace</h1>
          <p className="text-text-secondary">
            Local Kubernetes workspace manager
          </p>
        </header>

        <main>
          <div className="flex justify-between items-center mb-6">
            <h2 className="text-2xl font-semibold">Workspaces</h2>
            <button className="px-4 py-2 bg-accent-primary hover:bg-accent-hover rounded-md transition-fast">
              New Workspace
            </button>
          </div>

          <div className="grid gap-4">
            {workspaces.length === 0 ? (
              <div className="text-center py-12 bg-bg-secondary rounded-lg">
                <p className="text-text-secondary mb-4">
                  No workspaces yet
                </p>
                <p className="text-sm text-text-tertiary">
                  Create your first workspace to get started
                </p>
              </div>
            ) : (
              workspaces.map((workspace: any) => (
                <div
                  key={workspace.id}
                  className="p-4 bg-bg-elevated rounded-md border border-border"
                >
                  <h3 className="font-semibold">{workspace.name}</h3>
                  <p className="text-sm text-text-secondary">
                    {workspace.template}
                  </p>
                </div>
              ))
            )}
          </div>
        </main>
      </div>
    </div>
  )
}

export default App
