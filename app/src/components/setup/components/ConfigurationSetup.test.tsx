import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConfigurationSetup } from './ConfigurationSetup';

describe('ConfigurationSetup', () => {
  describe('Vibespace Name Input', () => {
    it('renders vibespace name input', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const input = screen.getByLabelText('Vibespace name');
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('placeholder', 'my-awesome-project');
    });

    it('allows entering vibespace name', async () => {
      const user = userEvent.setup();
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const input = screen.getByLabelText('Vibespace name');
      await user.type(input, 'my-vibespace');

      expect(input).toHaveValue('my-vibespace');
    });
  });

  describe('GitHub Repository Input', () => {
    it('renders GitHub repo input', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const input = screen.getByLabelText('GitHub repository URL (optional)');
      expect(input).toBeInTheDocument();
    });

    it('shows hint about leaving empty', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByText('Leave empty to start with a blank vibespace')).toBeInTheDocument();
    });

    it('allows entering GitHub repo URL', async () => {
      const user = userEvent.setup();
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const input = screen.getByLabelText('GitHub repository URL (optional)');
      await user.type(input, 'https://github.com/user/repo');

      expect(input).toHaveValue('https://github.com/user/repo');
    });
  });

  describe('Claude Code Info', () => {
    it('displays Claude Code built-in info', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByText('Claude Code Built-in')).toBeInTheDocument();
      expect(screen.getByText(/Every vibespace includes Claude Code/)).toBeInTheDocument();
    });

    it('displays Claude logo', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const claudeLogo = screen.getByAltText('Claude Code');
      expect(claudeLogo).toBeInTheDocument();
      expect(claudeLogo).toHaveAttribute('src', '/logos/agents/claude.svg');
    });
  });

  describe('Create Button', () => {
    it('renders create vibespace button', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByRole('button', { name: /create vibespace/i })).toBeInTheDocument();
    });

    it('calls onComplete when button is clicked with valid name', async () => {
      const user = userEvent.setup();
      const onComplete = vi.fn();
      render(<ConfigurationSetup onComplete={onComplete} />);

      // Enter vibespace name (required)
      const nameInput = screen.getByLabelText('Vibespace name');
      await user.type(nameInput, 'my-vibespace');

      const createButton = screen.getByRole('button', { name: /create vibespace/i });
      await user.click(createButton);

      expect(onComplete).toHaveBeenCalledTimes(1);
      expect(onComplete).toHaveBeenCalledWith({
        name: 'my-vibespace',
        githubRepo: '',
      });
    });

    it('includes github repo in callback when provided', async () => {
      const user = userEvent.setup();
      const onComplete = vi.fn();
      render(<ConfigurationSetup onComplete={onComplete} />);

      // Enter vibespace name
      const nameInput = screen.getByLabelText('Vibespace name');
      await user.type(nameInput, 'my-project');

      // Enter github repo
      const repoInput = screen.getByLabelText('GitHub repository URL (optional)');
      await user.type(repoInput, 'https://github.com/user/repo');

      const createButton = screen.getByRole('button', { name: /create vibespace/i });
      await user.click(createButton);

      expect(onComplete).toHaveBeenCalledWith({
        name: 'my-project',
        githubRepo: 'https://github.com/user/repo',
      });
    });

    it('shows alert when button is clicked without vibespace name', async () => {
      const user = userEvent.setup();
      const onComplete = vi.fn();
      const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
      render(<ConfigurationSetup onComplete={onComplete} />);

      const createButton = screen.getByRole('button', { name: /create vibespace/i });
      await user.click(createButton);

      expect(alertSpy).toHaveBeenCalledWith('Please enter a vibespace name');
      expect(onComplete).not.toHaveBeenCalled();

      alertSpy.mockRestore();
    });
  });

  describe('Accessibility', () => {
    it('has proper ARIA labels on inputs', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByLabelText('Vibespace name')).toBeInTheDocument();
      expect(screen.getByLabelText('GitHub repository URL (optional)')).toBeInTheDocument();
    });

    it('uses semantic HTML for sections', () => {
      const { container } = render(<ConfigurationSetup onComplete={vi.fn()} />);

      const sections = container.querySelectorAll('.config-section');
      expect(sections.length).toBeGreaterThan(0);
    });
  });
});
