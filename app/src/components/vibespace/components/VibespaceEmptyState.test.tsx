import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { VibespaceEmptyState } from './VibespaceEmptyState';

describe('VibespaceEmptyState', () => {
  it('renders empty state message', () => {
    render(<VibespaceEmptyState onCreateNew={vi.fn()} />);

    expect(screen.getByText('No vibespaces yet')).toBeInTheDocument();
    expect(
      screen.getByText(/Create your first containerized development environment/i)
    ).toBeInTheDocument();
  });

  it('renders create vibespace button', () => {
    render(<VibespaceEmptyState onCreateNew={vi.fn()} />);

    expect(screen.getByText('Create vibespace')).toBeInTheDocument();
  });

  it('calls onCreateNew when button is clicked', async () => {
    const user = userEvent.setup();
    const onCreateNew = vi.fn();
    render(<VibespaceEmptyState onCreateNew={onCreateNew} />);

    await user.click(screen.getByText('Create vibespace'));

    expect(onCreateNew).toHaveBeenCalledTimes(1);
  });

  it('renders empty state icon', () => {
    const { container } = render(<VibespaceEmptyState onCreateNew={vi.fn()} />);

    const svg = container.querySelector('svg.empty-icon');
    expect(svg).toBeInTheDocument();
  });
});
