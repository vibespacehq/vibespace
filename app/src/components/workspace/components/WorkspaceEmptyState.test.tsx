import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { WorkspaceEmptyState } from './WorkspaceEmptyState';

describe('WorkspaceEmptyState', () => {
  it('renders empty state message', () => {
    render(<WorkspaceEmptyState onCreateNew={vi.fn()} />);

    expect(screen.getByText('No workspaces yet')).toBeInTheDocument();
    expect(
      screen.getByText(/Create your first containerized development environment/i)
    ).toBeInTheDocument();
  });

  it('renders create workspace button', () => {
    render(<WorkspaceEmptyState onCreateNew={vi.fn()} />);

    expect(screen.getByText('Create workspace')).toBeInTheDocument();
  });

  it('calls onCreateNew when button is clicked', async () => {
    const user = userEvent.setup();
    const onCreateNew = vi.fn();
    render(<WorkspaceEmptyState onCreateNew={onCreateNew} />);

    await user.click(screen.getByText('Create workspace'));

    expect(onCreateNew).toHaveBeenCalledTimes(1);
  });

  it('renders empty state icon', () => {
    const { container } = render(<WorkspaceEmptyState onCreateNew={vi.fn()} />);

    const svg = container.querySelector('svg.empty-icon');
    expect(svg).toBeInTheDocument();
  });
});
