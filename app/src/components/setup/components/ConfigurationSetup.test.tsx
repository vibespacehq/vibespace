import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConfigurationSetup } from './ConfigurationSetup';

describe('ConfigurationSetup', () => {
  describe('Template Selection', () => {
    it('renders all template options', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByText('Next.js')).toBeInTheDocument();
      expect(screen.getByText('Vue')).toBeInTheDocument();
      expect(screen.getByText('Jupyter')).toBeInTheDocument();
    });

    it('pre-selects Next.js template by default', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const nextjsCard = screen.getByText('Next.js').closest('button');
      expect(nextjsCard).toHaveClass('selected');
    });

    it('allows selecting a different template', async () => {
      const user = userEvent.setup();
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const vueCard = screen.getByText('Vue').closest('button');
      await user.click(vueCard!);

      expect(vueCard).toHaveClass('selected');
    });
  });

  describe('AI Agent Selection', () => {
    it('renders all AI agent options', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByText('Claude Code')).toBeInTheDocument();
      expect(screen.getByText('OpenAI Codex')).toBeInTheDocument();
      expect(screen.getByText('Gemini CLI')).toBeInTheDocument();
    });

    it('pre-selects Claude Code by default', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const claudeCard = screen.getByText('Claude Code').closest('button');
      expect(claudeCard).toHaveClass('selected');
    });

    it('shows config file badge for each agent', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByText('CLAUDE.md')).toBeInTheDocument();
      expect(screen.getByText('.codex')).toBeInTheDocument();
      expect(screen.getByText('.gemini')).toBeInTheDocument();
    });

    it('allows selecting a different agent', async () => {
      const user = userEvent.setup();
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const codexCard = screen.getByText('OpenAI Codex').closest('button');
      await user.click(codexCard!);

      expect(codexCard).toHaveClass('selected');
    });
  });

  describe('Workspace Name Input', () => {
    it('renders workspace name input', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const input = screen.getByLabelText('Workspace name');
      expect(input).toBeInTheDocument();
      expect(input).toHaveAttribute('placeholder', 'my-awesome-project');
    });

    it('allows entering workspace name', async () => {
      const user = userEvent.setup();
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const input = screen.getByLabelText('Workspace name');
      await user.type(input, 'my-workspace');

      expect(input).toHaveValue('my-workspace');
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

      expect(screen.getByText('Leave empty to start with a blank workspace')).toBeInTheDocument();
    });

    it('allows entering GitHub repo URL', async () => {
      const user = userEvent.setup();
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const input = screen.getByLabelText('GitHub repository URL (optional)');
      await user.type(input, 'https://github.com/user/repo');

      expect(input).toHaveValue('https://github.com/user/repo');
    });
  });

  describe('Continue Button', () => {
    it('renders continue button', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByRole('button', { name: /continue/i })).toBeInTheDocument();
    });

    it('calls onComplete when continue is clicked with valid name', async () => {
      const user = userEvent.setup();
      const onComplete = vi.fn();
      render(<ConfigurationSetup onComplete={onComplete} />);

      // Enter workspace name (required)
      const nameInput = screen.getByLabelText('Workspace name');
      await user.type(nameInput, 'my-workspace');

      const continueButton = screen.getByRole('button', { name: /continue/i });
      await user.click(continueButton);

      expect(onComplete).toHaveBeenCalledTimes(1);
      expect(onComplete).toHaveBeenCalledWith({
        name: 'my-workspace',
        template: 'nextjs',
        agent: 'claude',
        githubRepo: '',
      });
    });

    it('shows alert when continue is clicked without workspace name', async () => {
      const user = userEvent.setup();
      const onComplete = vi.fn();
      const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});
      render(<ConfigurationSetup onComplete={onComplete} />);

      const continueButton = screen.getByRole('button', { name: /continue/i });
      await user.click(continueButton);

      expect(alertSpy).toHaveBeenCalledWith('Please enter a workspace name');
      expect(onComplete).not.toHaveBeenCalled();

      alertSpy.mockRestore();
    });
  });

  describe('Accessibility', () => {
    it('has proper ARIA labels on inputs', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      expect(screen.getByLabelText('Workspace name')).toBeInTheDocument();
      expect(screen.getByLabelText('GitHub repository URL (optional)')).toBeInTheDocument();
    });

    it('uses semantic HTML for sections', () => {
      const { container } = render(<ConfigurationSetup onComplete={vi.fn()} />);

      const sections = container.querySelectorAll('.config-section');
      expect(sections.length).toBeGreaterThan(0);
    });
  });

  describe('Display Logos', () => {
    it('displays template logos', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const nextjsLogo = screen.getByAltText('Next.js');
      expect(nextjsLogo).toBeInTheDocument();
      expect(nextjsLogo).toHaveAttribute('src', '/logos/templates/nextjs.svg');
    });

    it('displays agent logos', () => {
      render(<ConfigurationSetup onComplete={vi.fn()} />);

      const claudeLogo = screen.getByAltText('Claude Code');
      expect(claudeLogo).toBeInTheDocument();
      expect(claudeLogo).toHaveAttribute('src', '/logos/agents/claude.svg');
    });
  });
});
