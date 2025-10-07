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
    '@tauri-apps/cli', // CLI-only dependency, used in scripts
    '@tauri-apps/api', // Will be used for Tauri commands (Phase 1)
    '@tanstack/react-query', // Will be used for API state (Phase 2)
    'lucide-react', // Will be used for UI icons (Phase 2)
    'zustand', // Will be used for client state (Phase 2)
    'autoprefixer', // PostCSS plugin for Tailwind (configured in postcss.config.js)
    'postcss', // Required by Tailwind CSS (configured in postcss.config.js)
  ],
  ignoreBinaries: [
    'tauri', // Tauri CLI binary
  ],
}

export default config
