import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthenticationSetup } from './AuthenticationSetup';

describe('AuthenticationSetup', () => {
  let onComplete: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    onComplete = vi.fn();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders authentication page with title and description', () => {
    render(<AuthenticationSetup onComplete={onComplete} />);

    expect(screen.getByText('Welcome to vibespace')).toBeInTheDocument();
    expect(screen.getByText('Sign in to get started')).toBeInTheDocument();
    expect(screen.getAllByText('Authentication').length).toBeGreaterThan(0);
  });

  it('displays step progress indicator', () => {
    render(<AuthenticationSetup onComplete={onComplete} />);

    expect(screen.getAllByText('1').length).toBeGreaterThan(0);
    expect(screen.getByText('Step 1 of 4')).toBeInTheDocument();
  });

  it('renders all sign-in method buttons', () => {
    render(<AuthenticationSetup onComplete={onComplete} />);

    expect(screen.getByText('Sign in with GitHub')).toBeInTheDocument();
    expect(screen.getByText('Sign in with Google')).toBeInTheDocument();
    expect(screen.getByText('Sign in with Email')).toBeInTheDocument();
  });

  it('renders skip button for testing', () => {
    render(<AuthenticationSetup onComplete={onComplete} />);

    expect(screen.getByText('Skip (Testing)')).toBeInTheDocument();
  });

  describe('GitHub Sign-in', () => {
    it('shows loading state when GitHub sign-in is clicked', async () => {
      const user = userEvent.setup();
      render(<AuthenticationSetup onComplete={onComplete} />);

      const githubButton = screen.getByText('Sign in with GitHub');
      await user.click(githubButton);

      expect(screen.getByText('Signing in...')).toBeInTheDocument();
    });

    it('has aria-busy attribute during loading', async () => {
      const user = userEvent.setup();
      render(<AuthenticationSetup onComplete={onComplete} />);

      const githubButton = screen.getByText('Sign in with GitHub');
      await user.click(githubButton);

      await waitFor(() => {
        const buttons = screen.getAllByRole('button');
        const githubBtn = buttons.find(btn => btn.textContent?.includes('Signing in'));
        expect(githubBtn).toHaveAttribute('aria-busy', 'true');
      });
    });

    it('disables GitHub button during sign-in', async () => {
      const user = userEvent.setup();
      render(<AuthenticationSetup onComplete={onComplete} />);

      const githubButton = screen.getByText('Sign in with GitHub');
      await user.click(githubButton);

      await waitFor(() => {
        const buttons = screen.getAllByRole('button');
        const githubBtn = buttons.find(btn => btn.textContent?.includes('Signing in'));
        expect(githubBtn).toBeDisabled();
      });
    });

    it('calls onComplete after sign-in simulation', async () => {
      const user = userEvent.setup();
      render(<AuthenticationSetup onComplete={onComplete} />);

      const githubButton = screen.getByText('Sign in with GitHub');
      await user.click(githubButton);

      // Wait for the 1500ms timeout to complete
      await waitFor(() => {
        expect(onComplete).toHaveBeenCalledOnce();
      }, { timeout: 2000 });
    });
  });

  describe('Sign-in Method Buttons', () => {
    it('Google and Email buttons have coming soon labels', () => {
      render(<AuthenticationSetup onComplete={onComplete} />);

      const googleButton = screen.getByLabelText('Sign in with Google (Coming soon)');
      const emailButton = screen.getByLabelText('Sign in with Email (Coming soon)');

      expect(googleButton).toBeInTheDocument();
      expect(emailButton).toBeInTheDocument();
    });

    it('all sign-in buttons are enabled initially', () => {
      render(<AuthenticationSetup onComplete={onComplete} />);

      const githubButton = screen.getByText('Sign in with GitHub');
      const googleButton = screen.getByLabelText('Sign in with Google (Coming soon)');
      const emailButton = screen.getByLabelText('Sign in with Email (Coming soon)');

      expect(githubButton).not.toBeDisabled();
      expect(googleButton).not.toBeDisabled();
      expect(emailButton).not.toBeDisabled();
    });
  });

  describe('Accessibility', () => {
    it('has proper aria-label for disabled methods', () => {
      render(<AuthenticationSetup onComplete={onComplete} />);

      expect(screen.getByLabelText('Sign in with Google (Coming soon)')).toBeInTheDocument();
      expect(screen.getByLabelText('Sign in with Email (Coming soon)')).toBeInTheDocument();
    });

    it('decorative icons have aria-hidden', () => {
      const { container } = render(<AuthenticationSetup onComplete={onComplete} />);

      const icons = container.querySelectorAll('[aria-hidden="true"]');
      expect(icons.length).toBeGreaterThan(0);
    });
  });

  describe('Skip Button', () => {
    it('calls onComplete when skip button is clicked', async () => {
      const user = userEvent.setup();
      render(<AuthenticationSetup onComplete={onComplete} />);

      const skipButton = screen.getByText('Skip (Testing)');
      await user.click(skipButton);

      expect(onComplete).toHaveBeenCalledOnce();
    });

    it('skip button works immediately without delay', async () => {
      const user = userEvent.setup();
      render(<AuthenticationSetup onComplete={onComplete} />);

      const skipButton = screen.getByText('Skip (Testing)');
      await user.click(skipButton);

      // Should call immediately, not after timeout
      expect(onComplete).toHaveBeenCalledOnce();
    });
  });
});
