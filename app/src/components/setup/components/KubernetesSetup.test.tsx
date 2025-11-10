import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { KubernetesSetup } from './KubernetesSetup';
import * as kubernetesHook from '../../../hooks/useKubernetesStatus';
import type { KubernetesStatus } from '../../../lib/types';

// Mock the hooks
vi.mock('../../../hooks/useKubernetesStatus', () => ({
  useKubernetesStatus: vi.fn(),
  useKubernetesInstall: vi.fn(),
  useKubernetesControl: vi.fn(),
  getOSType: vi.fn().mockResolvedValue('macos'),
}));

describe('KubernetesSetup', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Not Installed State', () => {
    it('shows install button when kubernetes is not installed', async () => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: {
          installed: false,
          running: false,
          is_external: false,
        } as KubernetesStatus,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      vi.spyOn(kubernetesHook, 'useKubernetesInstall').mockReturnValue({
        install: vi.fn(),
        isInstalling: false,
        progress: null,
        error: null,
        installComplete: false,
      });

      vi.spyOn(kubernetesHook, 'useKubernetesControl').mockReturnValue({
        start: vi.fn(),
        stop: vi.fn(),
        uninstall: vi.fn(),
        isStarting: false,
        isStopping: false,
        error: null,
      });

      render(<KubernetesSetup onComplete={vi.fn()} />);

      await waitFor(() => {
        const button = screen.getByRole('button', { name: /Install vibespace/i });
        expect(button).toBeInTheDocument();
        expect(button).not.toBeDisabled();
      });
    });

    it('calls install function when button is clicked', async () => {
      const user = userEvent.setup();
      const installMock = vi.fn();

      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: {
          installed: false,
          running: false,
          is_external: false,
        } as KubernetesStatus,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      vi.spyOn(kubernetesHook, 'useKubernetesInstall').mockReturnValue({
        install: installMock,
        isInstalling: false,
        progress: null,
        error: null,
        installComplete: false,
      });

      vi.spyOn(kubernetesHook, 'useKubernetesControl').mockReturnValue({
        start: vi.fn(),
        stop: vi.fn(),
        uninstall: vi.fn(),
        isStarting: false,
        isStopping: false,
        error: null,
      });

      render(<KubernetesSetup onComplete={vi.fn()} />);

      const button = await screen.findByRole('button', { name: /Install vibespace/i });
      await user.click(button);

      expect(installMock).toHaveBeenCalled();
    });
  });

  describe('Component Installation', () => {
    beforeEach(() => {
      // Mock fetch for component status
      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes('/cluster/status')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              healthy: false,
              components: {
                knative: { installed: false, healthy: false },
                traefik: { installed: false, healthy: false },
                registry: { installed: false, healthy: false },
                buildkit: { installed: false, healthy: false },
              },
            }),
          } as Response);
        }
        return Promise.reject(new Error('Not found'));
      });
    });

    it('checks cluster status when kubernetes is running', async () => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: {
          installed: true,
          running: true,
          is_external: false,
          version: 'v1.27.0',
        } as KubernetesStatus,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      vi.spyOn(kubernetesHook, 'useKubernetesInstall').mockReturnValue({
        install: vi.fn(),
        isInstalling: false,
        progress: null,
        error: null,
        installComplete: true,
      });

      vi.spyOn(kubernetesHook, 'useKubernetesControl').mockReturnValue({
        start: vi.fn(),
        stop: vi.fn(),
        uninstall: vi.fn(),
        isStarting: false,
        isStopping: false,
        error: null,
      });

      render(<KubernetesSetup onComplete={vi.fn()} />);

      // Should fetch cluster status
      await waitFor(() => {
        expect(global.fetch).toHaveBeenCalled();
        const fetchCall = (global.fetch as any).mock.calls.find((call: any[]) =>
          call[0] && call[0].includes('/cluster/status')
        );
        expect(fetchCall).toBeTruthy();
      }, { timeout: 3000 });
    });
  });

  describe('Error States', () => {
    it('shows error message when installation fails', async () => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: {
          installed: false,
          running: false,
          is_external: false,
          error: 'Failed to download binaries',
        } as KubernetesStatus,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      vi.spyOn(kubernetesHook, 'useKubernetesInstall').mockReturnValue({
        install: vi.fn(),
        isInstalling: false,
        progress: null,
        error: 'Failed to download binaries',
        installComplete: false,
      });

      vi.spyOn(kubernetesHook, 'useKubernetesControl').mockReturnValue({
        start: vi.fn(),
        stop: vi.fn(),
        uninstall: vi.fn(),
        isStarting: false,
        isStopping: false,
        error: null,
      });

      render(<KubernetesSetup onComplete={vi.fn()} />);

      await waitFor(() => {
        // Error appears in status.error
        const errorElements = screen.getAllByText(/Failed to download binaries/i);
        expect(errorElements.length).toBeGreaterThan(0);
      });
    });

    it('shows retry button in error state', async () => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: {
          installed: false,
          running: false,
          is_external: false,
        } as KubernetesStatus,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      vi.spyOn(kubernetesHook, 'useKubernetesInstall').mockReturnValue({
        install: vi.fn(),
        isInstalling: false,
        progress: null,
        error: 'Installation failed',
        installComplete: false,
      });

      vi.spyOn(kubernetesHook, 'useKubernetesControl').mockReturnValue({
        start: vi.fn(),
        stop: vi.fn(),
        uninstall: vi.fn(),
        isStarting: false,
        isStopping: false,
        error: null,
      });

      render(<KubernetesSetup onComplete={vi.fn()} />);

      await waitFor(() => {
        // In error state (setupState='error'), shows Retry button
        const button = screen.getByRole('button', { name: /Retry/i });
        expect(button).toBeInTheDocument();
      });
    });
  });

  describe('Ready State', () => {
    it('calls onComplete when cluster is ready', async () => {
      const onCompleteMock = vi.fn();

      // Mock fetch for component status - all healthy
      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes('/cluster/status')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              healthy: true,
              components: {
                knative: { installed: true, healthy: true },
                traefik: { installed: true, healthy: true },
                registry: { installed: true, healthy: true },
                buildkit: { installed: true, healthy: true },
              },
            }),
          } as Response);
        }
        return Promise.reject(new Error('Not found'));
      });

      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: {
          installed: true,
          running: true,
          is_external: false,
          version: 'v1.27.0',
        } as KubernetesStatus,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      vi.spyOn(kubernetesHook, 'useKubernetesInstall').mockReturnValue({
        install: vi.fn(),
        isInstalling: false,
        progress: null,
        error: null,
        installComplete: false,
      });

      vi.spyOn(kubernetesHook, 'useKubernetesControl').mockReturnValue({
        start: vi.fn(),
        stop: vi.fn(),
        uninstall: vi.fn(),
        isStarting: false,
        isStopping: false,
        error: null,
      });

      render(<KubernetesSetup onComplete={onCompleteMock} />);

      // Wait for cluster check to complete and show Continue button
      await waitFor(() => {
        const button = screen.getByRole('button', { name: /Continue/i });
        expect(button).toBeInTheDocument();
      }, { timeout: 5000 });

      // Click continue button
      const user = userEvent.setup();
      const button = screen.getByRole('button', { name: /Continue/i });
      await user.click(button);

      expect(onCompleteMock).toHaveBeenCalled();
    });
  });

  describe('Accessibility', () => {
    it('has proper ARIA attributes on install button', async () => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: {
          installed: false,
          running: false,
          is_external: false,
        } as KubernetesStatus,
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      vi.spyOn(kubernetesHook, 'useKubernetesInstall').mockReturnValue({
        install: vi.fn(),
        isInstalling: false,
        progress: null,
        error: null,
        installComplete: false,
      });

      vi.spyOn(kubernetesHook, 'useKubernetesControl').mockReturnValue({
        start: vi.fn(),
        stop: vi.fn(),
        uninstall: vi.fn(),
        isStarting: false,
        isStopping: false,
        error: null,
      });

      render(<KubernetesSetup onComplete={vi.fn()} />);

      const installButton = await screen.findByRole('button', { name: /Install vibespace/i });
      expect(installButton).toBeInTheDocument();
      expect(installButton).not.toBeDisabled();
    });
  });
});
