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
    'src/test/**', // Test setup files
    'src/components/setup/components/CreateWorkspace.tsx', // WIP - Phase 1
  ],
  ignoreDependencies: [
    '@tanstack/react-query', // Will be used for API state (Phase 2)
    'lucide-react', // Will be used for UI icons (Phase 2)
    'zustand', // Will be used for client state (Phase 2)
    'autoprefixer', // PostCSS plugin for Tailwind (configured in postcss.config.js)
    'postcss', // Required by Tailwind CSS (configured in postcss.config.js)
    '@fontsource/space-grotesk', // Font imported in CSS (@import in index.css)
  ],
  ignoreExportsUsedInFile: {
    interface: true,
    type: true,
  },
}

export default config
