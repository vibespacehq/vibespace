import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkspaceCard } from './WorkspaceCard';
import type { Workspace } from '../../../lib/types';

const mockWorkspace: Workspace = {
  id: 'ws-1',
  name: 'test-workspace',
  template: 'nextjs',
  status: 'running',
  resources: {
    cpu: '2',
    memory: '4Gi',
  },
  urls: {
    'code-server': 'http://localhost:8080',
  },
  persistent: true,
  created_at: '2025-01-15T10:00:00Z',
};

describe('WorkspaceCard', () => {
  const mockHandlers = {
    onOpen: vi.fn(),
    onStart: vi.fn(),
    onStop: vi.fn(),
    onDelete: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders workspace information', () => {
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    expect(screen.getByText('test-workspace')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
    expect(screen.getByText('nextjs')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('4Gi')).toBeInTheDocument();
  });

  it('shows persistent badge for persistent workspaces', () => {
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    expect(screen.getByText('Persistent')).toBeInTheDocument();
  });

  it('does not show persistent badge for non-persistent workspaces', () => {
    const nonPersistentWorkspace = { ...mockWorkspace, persistent: false };
    render(<WorkspaceCard workspace={nonPersistentWorkspace} {...mockHandlers} />);

    expect(screen.queryByText('Persistent')).not.toBeInTheDocument();
  });

  it('enables open button when workspace is running', () => {
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    const openButton = screen.getByLabelText('Open workspace in browser');
    expect(openButton).not.toBeDisabled();
  });

  it('disables open button when workspace is stopped', () => {
    const stoppedWorkspace = { ...mockWorkspace, status: 'stopped' as const };
    render(<WorkspaceCard workspace={stoppedWorkspace} {...mockHandlers} />);

    const openButton = screen.getByLabelText('Open workspace in browser');
    expect(openButton).toBeDisabled();
  });

  it('calls onOpen when open button is clicked', async () => {
    const user = userEvent.setup();
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Open workspace in browser'));

    expect(mockHandlers.onOpen).toHaveBeenCalledWith('ws-1');
  });

  it('shows actions menu when menu button is clicked', async () => {
    const user = userEvent.setup();
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Workspace actions'));

    expect(screen.getByLabelText('Stop workspace')).toBeInTheDocument();
    expect(screen.getByLabelText('Delete workspace')).toBeInTheDocument();
  });

  it('shows start action when workspace is stopped', async () => {
    const user = userEvent.setup();
    const stoppedWorkspace = { ...mockWorkspace, status: 'stopped' as const };
    render(<WorkspaceCard workspace={stoppedWorkspace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Workspace actions'));

    expect(screen.getByLabelText('Start workspace')).toBeInTheDocument();
  });

  it('shows stop action when workspace is running', async () => {
    const user = userEvent.setup();
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Workspace actions'));

    expect(screen.getByLabelText('Stop workspace')).toBeInTheDocument();
  });

  it('calls onStop when stop action is clicked', async () => {
    const user = userEvent.setup();
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Workspace actions'));
    await user.click(screen.getByLabelText('Stop workspace'));

    expect(mockHandlers.onStop).toHaveBeenCalledWith('ws-1');
  });

  it('shows confirmation modal before deleting', async () => {
    const user = userEvent.setup();
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Workspace actions'));
    await user.click(screen.getByLabelText('Delete workspace'));

    // Modal should appear
    expect(screen.getByRole('heading', { name: 'Delete Workspace' })).toBeInTheDocument();
    expect(screen.getByText(/Type the workspace name/)).toBeInTheDocument();

    // Delete button should be disabled until name is typed
    const deleteButton = screen.getByRole('button', { name: 'Delete Workspace' });
    expect(deleteButton).toBeDisabled();

    // onDelete should not be called yet
    expect(mockHandlers.onDelete).not.toHaveBeenCalled();
  });

  it('calls onDelete when deletion is confirmed with correct workspace name', async () => {
    const user = userEvent.setup();
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Workspace actions'));
    await user.click(screen.getByLabelText('Delete workspace'));

    // Type the workspace name
    const input = screen.getByPlaceholderText('test-workspace');
    await user.type(input, 'test-workspace');

    // Click delete button
    const deleteButton = screen.getByRole('button', { name: 'Delete Workspace' });
    expect(deleteButton).not.toBeDisabled();
    await user.click(deleteButton);

    expect(mockHandlers.onDelete).toHaveBeenCalledWith('ws-1');
  });

  it('does not call onDelete when modal is cancelled', async () => {
    const user = userEvent.setup();
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Workspace actions'));
    await user.click(screen.getByLabelText('Delete workspace'));

    // Modal should be visible
    expect(screen.getByRole('heading', { name: 'Delete Workspace' })).toBeInTheDocument();

    // Click cancel button
    const cancelButton = screen.getByRole('button', { name: 'Cancel' });
    await user.click(cancelButton);

    // Modal should close and onDelete should not be called
    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: 'Delete Workspace' })).not.toBeInTheDocument();
    });
    expect(mockHandlers.onDelete).not.toHaveBeenCalled();
  });

  it('shows correct status color for running workspace', () => {
    render(<WorkspaceCard workspace={mockWorkspace} {...mockHandlers} />);

    const statusBadge = screen.getByText('running');
    expect(statusBadge).toHaveClass('status-running');
  });

  it('shows correct status color for stopped workspace', () => {
    const stoppedWorkspace = { ...mockWorkspace, status: 'stopped' as const };
    render(<WorkspaceCard workspace={stoppedWorkspace} {...mockHandlers} />);

    const statusBadge = screen.getByText('stopped');
    expect(statusBadge).toHaveClass('status-stopped');
  });

  it('shows correct status color for transitioning workspace', () => {
    const startingWorkspace = { ...mockWorkspace, status: 'starting' as const };
    render(<WorkspaceCard workspace={startingWorkspace} {...mockHandlers} />);

    const statusBadge = screen.getByText('starting');
    expect(statusBadge).toHaveClass('status-transitioning');
  });
});
