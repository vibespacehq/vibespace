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
  ],
  ignoreBinaries: [
    'tauri', // Tauri CLI binary
  ],
}

export default config
