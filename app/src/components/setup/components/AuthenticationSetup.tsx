import { useState } from 'react';
import { ProgressSidebar } from './ProgressSidebar';
import '../styles/setup.css';
import '../styles/AuthenticationSetup.css';

interface AuthenticationSetupProps {
  onComplete: () => void;
}

export function AuthenticationSetup({ onComplete }: AuthenticationSetupProps) {
  const [isSigningIn, setIsSigningIn] = useState(false);

  const handleSignIn = () => {
    setIsSigningIn(true);
    // Simulate sign-in process
    setTimeout(() => {
      onComplete();
    }, 1500);
  };

  return (
    <div className="setup-container">
      <ProgressSidebar currentStep={1} />
      <main className="setup-main">
        <header className="setup-header">
          <div className="step-badge">
            <span className="step-badge-number">1</span>
            <span>Step 1 of 4</span>
          </div>
          <h1 className="brand-title">Welcome to vibespace</h1>
          <p className="brand-subtitle">Sign in to get started</p>
          <div className="progress-bar-container">
            <div className="progress-bar-fill" data-progress="0"></div>
          </div>
        </header>

        <div className="setup-required">
          <div className="auth-container">
            <p className="auth-description">
              Sign in with your account to access vibespace and manage your development environments.
            </p>

            <div className="auth-methods">
              <button
                className="auth-method-btn"
                onClick={handleSignIn}
                disabled={isSigningIn}
                aria-busy={isSigningIn}
              >
                {isSigningIn ? (
                  <>
                    <div className="spinner-small" />
                    Signing in...
                  </>
                ) : (
                  <>
                    <span className="auth-icon-small" aria-hidden="true">⚡</span>
                    Sign in with GitHub
                  </>
                )}
              </button>

              <button
                className="auth-method-btn"
                disabled={isSigningIn}
                aria-label="Sign in with Google (Coming soon)"
              >
                <span className="auth-icon-small" aria-hidden="true">@</span>
                Sign in with Google
              </button>

              <button
                className="auth-method-btn"
                disabled={isSigningIn}
                aria-label="Sign in with Email (Coming soon)"
              >
                <span className="auth-icon-small" aria-hidden="true">✉</span>
                Sign in with Email
              </button>
            </div>

            <div className="auth-footer">
              <p className="auth-note">
                By signing in, you agree to our Terms of Service and Privacy Policy.
              </p>
            </div>

            {/* Temporary skip button for testing */}
            <div className="setup-actions" style={{ marginTop: '2rem' }}>
              <button onClick={onComplete} className="btn-primary">
                Skip (Testing)
              </button>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
