import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'path';

export default defineConfig({
  plugins: [react()],
  test: {
    // Use jsdom environment for DOM testing
    environment: 'jsdom',

    // Setup files to run before each test file
    setupFiles: ['./src/test/setup.ts'],

    // Global test utilities
    globals: true,

    // Coverage configuration
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      exclude: [
        'node_modules/',
        'src/test/',
        '**/*.test.{ts,tsx}',
        '**/*.spec.{ts,tsx}',
        'src-tauri/',
        'dist/',
        'build/',
      ],
    },

    // Test file patterns
    include: ['src/**/*.{test,spec}.{ts,tsx}'],

    // Exclude patterns
    exclude: [
      'node_modules',
      'dist',
      'build',
      'src-tauri',
      '.git',
    ],
  },

  // Path resolution (same as main vite.config.ts)
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
});
