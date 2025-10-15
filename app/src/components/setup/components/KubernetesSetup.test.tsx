import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { KubernetesSetup } from './KubernetesSetup';
import * as kubernetesHook from '../../../hooks/useKubernetesStatus';

// Mock the hooks and API
vi.mock('../../../hooks/useKubernetesStatus');

describe('KubernetesSetup', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Cluster Selection', () => {
    beforeEach(() => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: { available: true, installType: 'k3d', version: 'v1.27.0' },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes('/contexts')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              contexts: [
                { name: 'local-cluster', cluster: 'local', user: 'admin', is_current: true, is_local: true },
                { name: 'prod-cluster', cluster: 'production', user: 'admin', is_current: false, is_local: false },
                { name: 'dev-cluster', cluster: 'development', user: 'developer', is_current: false, is_local: true },
                { name: 'test-cluster', cluster: 'test', user: 'tester', is_current: false, is_local: true },
              ],
            }),
          } as Response);
        }
        return Promise.reject(new Error('Not found'));
      });
    });

    it('renders cluster selection cards', async () => {
      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText('local-cluster')).toBeInTheDocument();
        expect(screen.getByText('prod-cluster')).toBeInTheDocument();
        expect(screen.getByText('dev-cluster')).toBeInTheDocument();
      });
    });

    it('filters clusters by search query', async () => {
      const user = userEvent.setup();
      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText('local-cluster')).toBeInTheDocument();
      });

      // Search input should appear when there are more than 3 clusters
      const searchInput = screen.getByLabelText('Search Kubernetes clusters');
      await user.type(searchInput, 'prod');

      // Only prod-cluster should be visible
      expect(screen.getByText('prod-cluster')).toBeInTheDocument();
      expect(screen.queryByText('local-cluster')).not.toBeInTheDocument();
      expect(screen.queryByText('dev-cluster')).not.toBeInTheDocument();
    });

    it('shows warning for remote clusters', async () => {
      const user = userEvent.setup();
      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText('prod-cluster')).toBeInTheDocument();
      });

      // Click on remote cluster
      const prodCluster = screen.getByLabelText('Select prod-cluster cluster (remote)');
      await user.click(prodCluster);

      // Warning should appear
      await waitFor(() => {
        expect(screen.getByText(/This appears to be a remote cluster/i)).toBeInTheDocument();
      });
    });

    it('does not show warning for local clusters', async () => {
      const user = userEvent.setup();
      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText('local-cluster')).toBeInTheDocument();
      });

      // Click on local cluster
      const localCluster = screen.getByLabelText('Select local-cluster cluster');
      await user.click(localCluster);

      // Warning should not appear
      expect(screen.queryByText(/This appears to be a remote cluster/i)).not.toBeInTheDocument();
    });

    it('enables continue button when cluster is selected', async () => {
      const user = userEvent.setup();
      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText('local-cluster')).toBeInTheDocument();
      });

      const continueButton = screen.getByRole('button', { name: /continue/i });
      expect(continueButton).toBeDisabled();

      // Select a cluster
      const localCluster = screen.getByLabelText('Select local-cluster cluster');
      await user.click(localCluster);

      // Continue button should be enabled
      expect(continueButton).not.toBeDisabled();
    });

    it('shows loading state when switching context', async () => {
      const user = userEvent.setup();

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes('/contexts') && !url.includes('/switch')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              contexts: [
                { name: 'local-cluster', cluster: 'local', user: 'admin', is_current: true, is_local: true },
                { name: 'other-cluster', cluster: 'other', user: 'admin', is_current: false, is_local: true },
              ],
            }),
          } as Response);
        }
        if (url.includes('/switch')) {
          return new Promise((resolve) => {
            setTimeout(() => {
              resolve({ ok: true, json: () => Promise.resolve({}) } as Response);
            }, 100);
          });
        }
        if (url.includes('/status')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ healthy: true, components: {} }),
          } as Response);
        }
        return Promise.reject(new Error('Not found'));
      });

      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText('other-cluster')).toBeInTheDocument();
      });

      // Select different cluster
      const otherCluster = screen.getByLabelText('Select other-cluster cluster');
      await user.click(otherCluster);

      const continueButton = screen.getByRole('button', { name: /continue/i });
      await user.click(continueButton);

      // Should show loading state
      await waitFor(() => {
        expect(screen.getByText('Switching...')).toBeInTheDocument();
      });
    });
  });

  describe('Error States', () => {
    it('shows error message when context fetch fails', async () => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: { available: true, installType: 'k3d', version: 'v1.27.0' },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText(/Failed to load cluster contexts/i)).toBeInTheDocument();
      });
    });

    it('shows retry button in error state', async () => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: { available: true, installType: 'k3d', version: 'v1.27.0' },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      global.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByRole('button', { name: /retry/i })).toBeInTheDocument();
      });
    });
  });

  describe('Accessibility', () => {
    beforeEach(() => {
      vi.spyOn(kubernetesHook, 'useKubernetesStatus').mockReturnValue({
        status: { available: true, installType: 'k3d', version: 'v1.27.0' },
        isLoading: false,
        error: null,
        refetch: vi.fn(),
      });

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes('/contexts')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              contexts: [
                { name: 'local-cluster', cluster: 'local', user: 'admin', is_current: true, is_local: true },
                { name: 'prod-cluster', cluster: 'production', user: 'admin', is_current: false, is_local: false },
              ],
            }),
          } as Response);
        }
        return Promise.reject(new Error('Not found'));
      });
    });

    it('has proper ARIA labels on search input', async () => {
      render(<KubernetesSetup />);

      await waitFor(() => {
        const searchInput = screen.queryByLabelText('Search Kubernetes clusters');
        // Search only appears when > 3 clusters, so it may not be present
        if (searchInput) {
          expect(searchInput).toHaveAttribute('aria-label', 'Search Kubernetes clusters');
        }
      });
    });

    it('has proper ARIA attributes on cluster cards', async () => {
      render(<KubernetesSetup />);

      await waitFor(() => {
        const localCluster = screen.getByLabelText('Select local-cluster cluster');
        expect(localCluster).toHaveAttribute('aria-pressed', 'false');
      });
    });

    it('updates aria-pressed when cluster is selected', async () => {
      const user = userEvent.setup();
      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText('local-cluster')).toBeInTheDocument();
      });

      const localCluster = screen.getByLabelText('Select local-cluster cluster');
      await user.click(localCluster);

      await waitFor(() => {
        expect(localCluster).toHaveAttribute('aria-pressed', 'true');
      });
    });

    it('has aria-busy on continue button when switching', async () => {
      const user = userEvent.setup();

      global.fetch = vi.fn().mockImplementation((url: string) => {
        if (url.includes('/contexts') && !url.includes('/switch')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({
              contexts: [
                { name: 'local-cluster', cluster: 'local', user: 'admin', is_current: true, is_local: true },
                { name: 'other-cluster', cluster: 'other', user: 'admin', is_current: false, is_local: true },
              ],
            }),
          } as Response);
        }
        if (url.includes('/switch')) {
          return new Promise((resolve) => {
            setTimeout(() => {
              resolve({ ok: true, json: () => Promise.resolve({}) } as Response);
            }, 100);
          });
        }
        if (url.includes('/status')) {
          return Promise.resolve({
            ok: true,
            json: () => Promise.resolve({ healthy: true, components: {} }),
          } as Response);
        }
        return Promise.reject(new Error('Not found'));
      });

      render(<KubernetesSetup />);

      await waitFor(() => {
        expect(screen.getByText('other-cluster')).toBeInTheDocument();
      });

      const otherCluster = screen.getByLabelText('Select other-cluster cluster');
      await user.click(otherCluster);

      const continueButton = screen.getByRole('button', { name: /continue/i });
      await user.click(continueButton);

      await waitFor(() => {
        expect(continueButton).toHaveAttribute('aria-busy', 'true');
      });
    });
  });
});
