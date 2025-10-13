import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkspaceList } from './WorkspaceList';

const mockWorkspaces = [
  {
    id: 'ws-1',
    name: 'test-workspace',
    template: 'Next.js',
    status: 'running' as const,
    createdAt: '1 hour ago',
  },
  {
    id: 'ws-2',
    name: 'python-project',
    template: 'Jupyter',
    status: 'stopped' as const,
    createdAt: '2 days ago',
  },
];

describe('WorkspaceList', () => {
  describe('Empty State', () => {
    it('shows empty state when no workspaces exist', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={[]} onCreateNew={onCreateNew} />);

      expect(screen.getByText('No workspaces yet')).toBeInTheDocument();
      expect(screen.getByText('Create your first workspace to get started')).toBeInTheDocument();
    });

    it('displays create workspace button in empty state', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={[]} onCreateNew={onCreateNew} />);

      expect(screen.getByText('Create workspace')).toBeInTheDocument();
    });

    it('calls onCreateNew when create button is clicked in empty state', async () => {
      const user = userEvent.setup();
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={[]} onCreateNew={onCreateNew} />);

      const createButton = screen.getByText('Create workspace');
      await user.click(createButton);

      expect(onCreateNew).toHaveBeenCalledOnce();
    });

    it('shows header with new workspace button in empty state', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={[]} onCreateNew={onCreateNew} />);

      expect(screen.getByText('workspaces')).toBeInTheDocument();
      expect(screen.getByText('+ New workspace')).toBeInTheDocument();
    });
  });

  describe('Populated State', () => {
    it('renders all workspace cards', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={mockWorkspaces} onCreateNew={onCreateNew} />);

      expect(screen.getByText('test-workspace')).toBeInTheDocument();
      expect(screen.getByText('python-project')).toBeInTheDocument();
    });

    it('displays workspace count in header', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={mockWorkspaces} onCreateNew={onCreateNew} />);

      expect(screen.getByText('2 workspaces')).toBeInTheDocument();
    });

    it('displays singular "workspace" for single workspace', () => {
      const onCreateNew = vi.fn();
      const singleWorkspace = [mockWorkspaces[0]];
      render(<WorkspaceList workspaces={singleWorkspace} onCreateNew={onCreateNew} />);

      expect(screen.getByText('1 workspace')).toBeInTheDocument();
    });

    it('shows status badges for each workspace', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={mockWorkspaces} onCreateNew={onCreateNew} />);

      expect(screen.getByText('running')).toBeInTheDocument();
      expect(screen.getByText('stopped')).toBeInTheDocument();
    });

    it('displays template information', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={mockWorkspaces} onCreateNew={onCreateNew} />);

      expect(screen.getByText('Next.js')).toBeInTheDocument();
      expect(screen.getByText('Jupyter')).toBeInTheDocument();
    });

    it('displays creation timestamps', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={mockWorkspaces} onCreateNew={onCreateNew} />);

      expect(screen.getByText('1 hour ago')).toBeInTheDocument();
      expect(screen.getByText('2 days ago')).toBeInTheDocument();
    });

    it('renders Open and More actions buttons for each workspace', () => {
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={mockWorkspaces} onCreateNew={onCreateNew} />);

      const openButtons = screen.getAllByText('Open');
      expect(openButtons).toHaveLength(2);

      const moreButtons = screen.getAllByLabelText('More actions');
      expect(moreButtons).toHaveLength(2);
    });

    it('calls onCreateNew when new workspace button is clicked', async () => {
      const user = userEvent.setup();
      const onCreateNew = vi.fn();
      render(<WorkspaceList workspaces={mockWorkspaces} onCreateNew={onCreateNew} />);

      const newWorkspaceButton = screen.getByText('+ New workspace');
      await user.click(newWorkspaceButton);

      expect(onCreateNew).toHaveBeenCalledOnce();
    });

    it('applies correct CSS classes to status badges', () => {
      const onCreateNew = vi.fn();
      const { container } = render(<WorkspaceList workspaces={mockWorkspaces} onCreateNew={onCreateNew} />);

      const runningBadge = container.querySelector('.status-running');
      const stoppedBadge = container.querySelector('.status-stopped');

      expect(runningBadge).toBeInTheDocument();
      expect(stoppedBadge).toBeInTheDocument();
    });
  });
});
