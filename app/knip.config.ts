import type { KnipConfig } from 'knip'

const config: KnipConfig = {
  entry: [
    'src/main.tsx',
    'src/App.tsx',
  ],
  project: [
    'src/**/*.{ts,tsx}',
  ],
  ignore: [
    'src/**/*.test.{ts,tsx}',
    'src/**/*.spec.{ts,tsx}',
    'src/**/*.stories.{ts,tsx}',
    'src/**/__tests__/**',
    'src/**/__mocks__/**',
  ],
  ignoreDependencies: [
    '@tanstack/react-query', // Will be used for API state (Phase 2)
    'lucide-react', // Will be used for UI icons (Phase 2)
    'zustand', // Will be used for client state (Phase 2)
    'autoprefixer', // PostCSS plugin for Tailwind (configured in postcss.config.js)
    'postcss', // Required by Tailwind CSS (configured in postcss.config.js)
  ],
  ignoreExportsUsedInFile: {
    interface: true,
    type: true,
  },
  // Exports for future features (Phase 1-2)
  ignoreExports: [
    // Individual detection functions - will be used in Phase 2 for workspace health monitoring
    'checkKubectl',
    'findKubeconfig',
    'checkClusterHealth',
    'detectInstallType',
    'getClusterVersion',
    // Types for future CRUD operations
    'KubernetesInstallType',
    'Workspace',
    'CreateWorkspaceRequest',
    'Template',
    'Credential',
    'CredentialData',
    'SshKeyPair',
  ],
}

export default config
