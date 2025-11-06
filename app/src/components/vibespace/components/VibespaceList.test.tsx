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
    urls: { 'code-server': 'http://localhost:8080' },
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

    it('opens vibespace URL when open is clicked', async () => {
      const user = userEvent.setup();
      const accessVibespace = vi.fn().mockResolvedValue({
        'code-server': 'http://127.0.0.1:8815',
        'preview': 'http://127.0.0.1:8916'
      });
      const windowOpenSpy = vi.spyOn(window, 'open');

      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        accessVibespace,
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      const openButtons = screen.getAllByLabelText('Open vibespace in browser');
      await user.click(openButtons[0]);

      expect(accessVibespace).toHaveBeenCalledWith('ws-1');
      expect(windowOpenSpy).toHaveBeenCalledWith('http://127.0.0.1:8815', '_blank');
    });

    it('calls accessVibespace when open is clicked', async () => {
      const user = userEvent.setup();
      const accessVibespace = vi.fn().mockResolvedValue({
        'code-server': 'http://127.0.0.1:8816',
        'preview': 'http://127.0.0.1:8917'
      });

      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        accessVibespace,
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      const openButtons = screen.getAllByLabelText('Open vibespace in browser');
      await user.click(openButtons[0]);

      expect(accessVibespace).toHaveBeenCalledWith('ws-1');
    });
  });
});
