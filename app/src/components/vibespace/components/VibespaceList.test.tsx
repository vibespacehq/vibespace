import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { VibespaceList } from './VibespaceList';
import * as useVibespacesHook from '../../../hooks/useVibespaces';
import type { Vibespace } from '../../../lib/types';

const mockVibespaces: Vibespace[] = [
  {
    id: 'ws-1',
    name: 'my-app',
    project_name: 'brave-fox-42',
    status: 'running',
    resources: { cpu: '2', memory: '4Gi' },
    persistent: true,
    created_at: '2025-01-15T10:00:00Z',
  },
  {
    id: 'ws-2',
    name: 'ml-project',
    project_name: 'swift-owl-13',
    status: 'stopped',
    resources: { cpu: '4', memory: '8Gi' },
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
    accessVibespace: vi.fn().mockResolvedValue({ main: 'https://brave-fox-42.vibe.space' }),
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

      expect(screen.getByText('my-app')).toBeInTheDocument();
      expect(screen.getByText('ml-project')).toBeInTheDocument();
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

    it('shows Open button for running vibespaces', () => {
      render(<VibespaceList onCreateNew={vi.fn()} />);

      // Running vibespace should have Open button
      const openButtons = screen.getAllByLabelText('Open vibespace');
      expect(openButtons.length).toBeGreaterThan(0);
    });

    it('calls accessVibespace and opens URL when Open is clicked', async () => {
      const user = userEvent.setup();
      const accessVibespace = vi.fn().mockResolvedValue({ main: 'https://brave-fox-42.vibe.space' });
      const windowOpenSpy = vi.spyOn(window, 'open');

      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        accessVibespace,
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      // Click the Open button for the running vibespace
      const openButton = screen.getAllByLabelText('Open vibespace')[0];
      await user.click(openButton);

      // Should call accessVibespace to get the URL
      expect(accessVibespace).toHaveBeenCalledWith('ws-1');

      // Should open the URL in a new window
      expect(windowOpenSpy).toHaveBeenCalledWith('https://brave-fox-42.vibe.space', '_blank');
    });

    it('does not show Open button for stopped vibespaces', () => {
      // Only show stopped vibespace
      vi.spyOn(useVibespacesHook, 'useVibespaces').mockReturnValue({
        ...mockUseVibespaces,
        vibespaces: [mockVibespaces[1]], // Stopped vibespace
      });

      render(<VibespaceList onCreateNew={vi.fn()} />);

      // Stopped vibespace should not have Open button
      expect(screen.queryByLabelText('Open vibespace')).not.toBeInTheDocument();
    });
  });
});
