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
    'autoprefixer', // PostCSS plugin for Tailwind (auto-loaded by Vite)
    'postcss', // Required by Tailwind CSS (auto-loaded by Vite for @tailwind directives)
    '@fontsource/space-grotesk', // Font imported in CSS (@import in index.css)
  ],
  ignoreExportsUsedInFile: {
    interface: true,
    type: true,
  },
}

export default config
