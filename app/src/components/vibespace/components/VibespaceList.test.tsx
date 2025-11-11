import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { VibespaceList } from './VibespaceList';
import * as useVibespacesHook from '../../../hooks/useVibespaces';
import type { Vibespace } from '../../../lib/types';

const mockVibespaces: Vibespace[] = [
  {
    id: 'ws-1',
    name: 'nextjs-app',
    template: 'nextjs',
    status: 'running',
    resources: { cpu: '2', memory: '4Gi' },
    urls: {
      code: 'http://code.example.vibe.space',
      preview: 'http://preview.example.vibe.space',
      prod: 'http://prod.example.vibe.space',
    },
    persistent: true,
    created_at: '2025-01-15T10:00:00Z',
  },
  {
    id: 'ws-2',
    name: 'python-ml',
    template: 'jupyter',
    status: 'stopped',
    resources: { cpu: '4', memory: '8Gi' },
    urls: {},
    persistent: false,
    created_at: '2025-01-14T10:00:00Z',
  },
];

describe('VibespaceList', () => {
  const mockUseVibespaces = {
    vibespaces: mockVibespaces,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    createVibespace: vi.fn(),
    deleteVibespace: vi.fn(),
    startVibespace: vi.fn(),
    stopVibespace: vi.fn(),
    accessVibespace: vi.fn(),
  };

  beforeEach(() => {
    vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue(mockUseVibespaces);
    vi.spyOn(window, 'open').mockImplementation(() => null);
  });

  describe('Loading State', () => {
    it('shows loading state when fetching vibespaces', () => {
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        isLoading: true,
        vibespaces: [],
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('Loading vibespaces...')).toBeInTheDocument();
    });

    it('shows spinner in loading state', () => {
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        isLoading: true,
        vibespaces: [],
      });

      const { container } = render(<VibespaceList onCreateNew={vi.fn()} />);
      const spinner = container.querySelector('.spinner');

      expect(spinner).toBeInTheDocument();
    });
  });

  describe('Error State', () => {
    it('shows error message when fetch fails', () => {
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        error: 'Failed to connect to API',
        vibespaces: [],
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('Failed to connect to API')).toBeInTheDocument();
    });

    it('shows retry button in error state', () => {
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        error: 'Failed to connect to API',
        vibespaces: [],
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('Retry')).toBeInTheDocument();
    });

    it('calls refetch when retry button is clicked', async () => {
      const user = userEvent.setup();
      const refetch = vi.fn();
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        error: 'Failed to connect to API',
        vibespaces: [],
        refetch,
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      await user.click(screen.getByText('Retry'));

      expect(refetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('Empty State', () => {
    it('shows empty state when no vibespaces exist', () => {
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        vibespaces: [],
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('No vibespaces yet')).toBeInTheDocument();
    });

    it('calls onCreateNew when create button is clicked in empty state', async () => {
      const user = userEvent.setup();
      const onCreateNew = vi.fn();
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        vibespaces: [],
      });

      render(<VibespaceList onCreateNew={onCreateNew} />);

      await user.click(screen.getByText('Create vibespace'));

      expect(onCreateNew).toHaveBeenCalledTimes(1);
    });
  });

  describe('Populated State', () => {
    it('renders vibespace cards', () => {
      render(<VibespaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('nextjs-app')).toBeInTheDocument();
      expect(screen.getByText('python-ml')).toBeInTheDocument();
    });

    it('shows vibespace count in header', () => {
      render(<VibespaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('2 vibespaces')).toBeInTheDocument();
    });

    it('shows singular vibespace text when only one vibespace', () => {
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        vibespaces: [mockVibespaces[0]],
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('1 vibespace')).toBeInTheDocument();
    });

    it('renders new vibespace button in header', () => {
      render(<VibespaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('New vibespace')).toBeInTheDocument();
    });

    it('calls onCreateNew when header button is clicked', async () => {
      const user = userEvent.setup();
      const onCreateNew = vi.fn();

      render(<VibespaceList onCreateNew={onCreateNew} />);

      await user.click(screen.getByText('New vibespace'));

      expect(onCreateNew).toHaveBeenCalledTimes(1);
    });

    it('opens vibespace URL when code button is clicked', async () => {
      const user = userEvent.setup();
      const windowOpenSpy = vi.spyOn(window, 'open');

      render(<VibespaceList onCreateNew={vi.fn()} />);

      // Click the Code button (first vibespace, which has DNS URLs)
      const codeButton = screen.getAllByLabelText('Open code-server in browser')[0];
      await user.click(codeButton);

      // Should use DNS URL directly (no /access call needed)
      expect(windowOpenSpy).toHaveBeenCalledWith('http://code.example.vibe.space', '_blank');
    });

    it('uses DNS URLs directly when available (no /access call)', async () => {
      const user = userEvent.setup();
      const accessVibespace = vi.fn();

      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        accessVibespace,
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      const codeButton = screen.getAllByLabelText('Open code-server in browser')[0];
      await user.click(codeButton);

      // Should NOT call accessVibespace when DNS URLs are already available
      expect(accessVibespace).not.toHaveBeenCalled();
    });

    it('does not show buttons when vibespace has no URLs', () => {
      // Mock vibespace without URLs (happens when vibespace is stopped or URLs not yet populated)
      const vibespacesWithoutUrls = [{
        ...mockVibespaces[0],
        urls: {},
      }];

      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        vibespaces: vibespacesWithoutUrls,
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      // No buttons should be rendered when URLs are empty
      expect(screen.queryByLabelText('Open code-server in browser')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Open preview server in browser')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Open production server in browser')).not.toBeInTheDocument();
    });
  });
});
