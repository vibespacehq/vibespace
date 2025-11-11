import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { VibespaceCard } from './VibespaceCard';
import type { Vibespace } from '../../../lib/types';

const mockVibespace: Vibespace = {
  id: 'ws-1',
  name: 'test-vibespace',
  template: 'nextjs',
  status: 'running',
  resources: {
    cpu: '2',
    memory: '4Gi',
  },
  urls: {
    code: 'http://code.example.vibe.space',
    preview: 'http://preview.example.vibe.space',
    prod: 'http://prod.example.vibe.space',
  },
  persistent: true,
  created_at: '2025-01-15T10:00:00Z',
};

describe('VibespaceCard', () => {
  const mockHandlers = {
    onOpen: vi.fn(),
    onStart: vi.fn(),
    onStop: vi.fn(),
    onDelete: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders vibespace information', () => {
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    expect(screen.getByText('test-vibespace')).toBeInTheDocument();
    expect(screen.getByText('running')).toBeInTheDocument();
    expect(screen.getByText('nextjs')).toBeInTheDocument();
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('4Gi')).toBeInTheDocument();
  });

  it('shows persistent badge for persistent vibespaces', () => {
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    expect(screen.getByText('Persistent')).toBeInTheDocument();
  });

  it('does not show persistent badge for non-persistent vibespaces', () => {
    const nonPersistentVibespace = { ...mockVibespace, persistent: false };
    render(<VibespaceCard vibespace={nonPersistentVibespace} {...mockHandlers} />);

    expect(screen.queryByText('Persistent')).not.toBeInTheDocument();
  });

  it('enables code button when vibespace is running', () => {
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    const codeButton = screen.getByLabelText('Open code-server in browser');
    expect(codeButton).not.toBeDisabled();
  });

  it('disables code button when vibespace is stopped', () => {
    const stoppedVibespace = { ...mockVibespace, status: 'stopped' as const };
    render(<VibespaceCard vibespace={stoppedVibespace} {...mockHandlers} />);

    expect(screen.queryByLabelText('Open code-server in browser')).not.toBeInTheDocument();
  });

  it('calls onOpen with code urlType when code button is clicked', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Open code-server in browser'));

    expect(mockHandlers.onOpen).toHaveBeenCalledWith('ws-1', 'code');
  });

  it('shows preview button when preview URL is available', () => {
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    const previewButton = screen.getByLabelText('Open preview server in browser');
    expect(previewButton).toBeInTheDocument();
    expect(previewButton).not.toBeDisabled();
  });

  it('calls onOpen with preview urlType when preview button is clicked', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Open preview server in browser'));

    expect(mockHandlers.onOpen).toHaveBeenCalledWith('ws-1', 'preview');
  });

  it('shows production button when prod URL is available', () => {
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    const prodButton = screen.getByLabelText('Open production server in browser');
    expect(prodButton).toBeInTheDocument();
    expect(prodButton).not.toBeDisabled();
  });

  it('calls onOpen with prod urlType when production button is clicked', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Open production server in browser'));

    expect(mockHandlers.onOpen).toHaveBeenCalledWith('ws-1', 'prod');
  });

  it('shows actions menu when menu button is clicked', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Vibespace actions'));

    expect(screen.getByLabelText('Stop vibespace')).toBeInTheDocument();
    expect(screen.getByLabelText('Delete vibespace')).toBeInTheDocument();
  });

  it('shows start action when vibespace is stopped', async () => {
    const user = userEvent.setup();
    const stoppedVibespace = { ...mockVibespace, status: 'stopped' as const };
    render(<VibespaceCard vibespace={stoppedVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Vibespace actions'));

    expect(screen.getByLabelText('Start vibespace')).toBeInTheDocument();
  });

  it('shows stop action when vibespace is running', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Vibespace actions'));

    expect(screen.getByLabelText('Stop vibespace')).toBeInTheDocument();
  });

  it('calls onStop when stop action is clicked', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Vibespace actions'));
    await user.click(screen.getByLabelText('Stop vibespace'));

    expect(mockHandlers.onStop).toHaveBeenCalledWith('ws-1');
  });

  it('shows confirmation modal before deleting', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Vibespace actions'));
    await user.click(screen.getByLabelText('Delete vibespace'));

    // Modal should appear
    expect(screen.getByRole('heading', { name: 'Delete Vibespace' })).toBeInTheDocument();
    expect(screen.getByText(/Type the vibespace name/)).toBeInTheDocument();

    // Delete button should be disabled until name is typed
    const deleteButton = screen.getByRole('button', { name: 'Delete Vibespace' });
    expect(deleteButton).toBeDisabled();

    // onDelete should not be called yet
    expect(mockHandlers.onDelete).not.toHaveBeenCalled();
  });

  it('calls onDelete when deletion is confirmed with correct vibespace name', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Vibespace actions'));
    await user.click(screen.getByLabelText('Delete vibespace'));

    // Type the vibespace name
    const input = screen.getByPlaceholderText('test-vibespace');
    await user.type(input, 'test-vibespace');

    // Click delete button
    const deleteButton = screen.getByRole('button', { name: 'Delete Vibespace' });
    expect(deleteButton).not.toBeDisabled();
    await user.click(deleteButton);

    expect(mockHandlers.onDelete).toHaveBeenCalledWith('ws-1');
  });

  it('does not call onDelete when modal is cancelled', async () => {
    const user = userEvent.setup();
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    await user.click(screen.getByLabelText('Vibespace actions'));
    await user.click(screen.getByLabelText('Delete vibespace'));

    // Modal should be visible
    expect(screen.getByRole('heading', { name: 'Delete Vibespace' })).toBeInTheDocument();

    // Click cancel button
    const cancelButton = screen.getByRole('button', { name: 'Cancel' });
    await user.click(cancelButton);

    // Modal should close and onDelete should not be called
    await waitFor(() => {
      expect(screen.queryByRole('heading', { name: 'Delete Vibespace' })).not.toBeInTheDocument();
    });
    expect(mockHandlers.onDelete).not.toHaveBeenCalled();
  });

  it('shows correct status color for running vibespace', () => {
    render(<VibespaceCard vibespace={mockVibespace} {...mockHandlers} />);

    const statusBadge = screen.getByText('running');
    expect(statusBadge).toHaveClass('status-running');
  });

  it('shows correct status color for stopped vibespace', () => {
    const stoppedVibespace = { ...mockVibespace, status: 'stopped' as const };
    render(<VibespaceCard vibespace={stoppedVibespace} {...mockHandlers} />);

    const statusBadge = screen.getByText('stopped');
    expect(statusBadge).toHaveClass('status-stopped');
  });

  it('shows correct status color for transitioning vibespace', () => {
    const startingVibespace = { ...mockVibespace, status: 'starting' as const };
    render(<VibespaceCard vibespace={startingVibespace} {...mockHandlers} />);

    const statusBadge = screen.getByText('starting');
    expect(statusBadge).toHaveClass('status-transitioning');
  });
});
