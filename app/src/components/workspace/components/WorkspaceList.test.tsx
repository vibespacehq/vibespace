import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkspaceList } from './WorkspaceList';
import * as useWorkspacesHook from '../../../hooks/useWorkspaces';
import type { Workspace } from '../../../lib/types';

const mockWorkspaces: Workspace[] = [
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

describe('WorkspaceList', () => {
  const mockUseWorkspaces = {
    workspaces: mockWorkspaces,
    isLoading: false,
    error: null,
    refetch: vi.fn(),
    createWorkspace: vi.fn(),
    deleteWorkspace: vi.fn(),
    startWorkspace: vi.fn(),
    stopWorkspace: vi.fn(),
    accessWorkspace: vi.fn(),
  };

  beforeEach(() => {
    vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue(mockUseWorkspaces);
    vi.spyOn(window, 'open').mockImplementation(() => null);
  });

  describe('Loading State', () => {
    it('shows loading state when fetching workspaces', () => {
      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        isLoading: true,
        workspaces: [],
      });

      render(<WorkspaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('Loading workspaces...')).toBeInTheDocument();
    });

    it('shows spinner in loading state', () => {
      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        isLoading: true,
        workspaces: [],
      });

      const { container } = render(<WorkspaceList onCreateNew={vi.fn()} />);
      const spinner = container.querySelector('.spinner');

      expect(spinner).toBeInTheDocument();
    });
  });

  describe('Error State', () => {
    it('shows error message when fetch fails', () => {
      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        error: 'Failed to connect to API',
        workspaces: [],
      });

      render(<WorkspaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('Failed to connect to API')).toBeInTheDocument();
    });

    it('shows retry button in error state', () => {
      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        error: 'Failed to connect to API',
        workspaces: [],
      });

      render(<WorkspaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('Retry')).toBeInTheDocument();
    });

    it('calls refetch when retry button is clicked', async () => {
      const user = userEvent.setup();
      const refetch = vi.fn();
      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        error: 'Failed to connect to API',
        workspaces: [],
        refetch,
      });

      render(<WorkspaceList onCreateNew={vi.fn()} />);

      await user.click(screen.getByText('Retry'));

      expect(refetch).toHaveBeenCalledTimes(1);
    });
  });

  describe('Empty State', () => {
    it('shows empty state when no workspaces exist', () => {
      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        workspaces: [],
      });

      render(<WorkspaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('No workspaces yet')).toBeInTheDocument();
    });

    it('calls onCreateNew when create button is clicked in empty state', async () => {
      const user = userEvent.setup();
      const onCreateNew = vi.fn();
      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        workspaces: [],
      });

      render(<WorkspaceList onCreateNew={onCreateNew} />);

      await user.click(screen.getByText('Create workspace'));

      expect(onCreateNew).toHaveBeenCalledTimes(1);
    });
  });

  describe('Populated State', () => {
    it('renders workspace cards', () => {
      render(<WorkspaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('nextjs-app')).toBeInTheDocument();
      expect(screen.getByText('python-ml')).toBeInTheDocument();
    });

    it('shows workspace count in header', () => {
      render(<WorkspaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('2 workspaces')).toBeInTheDocument();
    });

    it('shows singular workspace text when only one workspace', () => {
      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        workspaces: [mockWorkspaces[0]],
      });

      render(<WorkspaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('1 workspace')).toBeInTheDocument();
    });

    it('renders new workspace button in header', () => {
      render(<WorkspaceList onCreateNew={vi.fn()} />);

      expect(screen.getByText('New workspace')).toBeInTheDocument();
    });

    it('calls onCreateNew when header button is clicked', async () => {
      const user = userEvent.setup();
      const onCreateNew = vi.fn();

      render(<WorkspaceList onCreateNew={onCreateNew} />);

      await user.click(screen.getByText('New workspace'));

      expect(onCreateNew).toHaveBeenCalledTimes(1);
    });

    it('opens workspace URL when open is clicked', async () => {
      const user = userEvent.setup();
      const accessWorkspace = vi.fn().mockResolvedValue('http://127.0.0.1:8815');
      const windowOpenSpy = vi.spyOn(window, 'open');

      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        accessWorkspace,
      });

      render(<WorkspaceList onCreateNew={vi.fn()} />);

      const openButtons = screen.getAllByLabelText('Open workspace in browser');
      await user.click(openButtons[0]);

      expect(accessWorkspace).toHaveBeenCalledWith('ws-1');
      expect(windowOpenSpy).toHaveBeenCalledWith('http://127.0.0.1:8815', '_blank');
    });

    it('calls accessWorkspace when open is clicked', async () => {
      const user = userEvent.setup();
      const accessWorkspace = vi.fn().mockResolvedValue('http://127.0.0.1:8816');

      vi.spyOn(useWorkspacesHook, 'useWorkspaces').mockReturnValue({
        ...mockUseWorkspaces,
        accessWorkspace,
      });

      render(<WorkspaceList onCreateNew={vi.fn()} />);

      const openButtons = screen.getAllByLabelText('Open workspace in browser');
      await user.click(openButtons[0]);

      expect(accessWorkspace).toHaveBeenCalledWith('ws-1');
    });
  });
});
