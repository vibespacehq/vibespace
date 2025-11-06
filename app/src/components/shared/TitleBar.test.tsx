import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TitleBar } from './TitleBar';
import * as window from '@tauri-apps/api/window';

// Mock the entire window module
vi.mock('@tauri-apps/api/window', () => ({
  getCurrentWindow: vi.fn(),
}));

describe('TitleBar', () => {
  let mockMinimize: ReturnType<typeof vi.fn>;
  let mockToggleMaximize: ReturnType<typeof vi.fn>;
  let mockClose: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockMinimize = vi.fn().mockResolvedValue(undefined);
    mockToggleMaximize = vi.fn().mockResolvedValue(undefined);
    mockClose = vi.fn().mockResolvedValue(undefined);

    vi.mocked(window.getCurrentWindow).mockReturnValue({
      minimize: mockMinimize,
      toggleMaximize: mockToggleMaximize,
      close: mockClose,
      isMaximized: vi.fn().mockResolvedValue(false),
    } as any);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders titlebar with window controls', () => {
    render(<TitleBar />);

    // Check window control buttons exist
    expect(screen.getByLabelText('Close')).toBeInTheDocument();
    expect(screen.getByLabelText('Minimize')).toBeInTheDocument();
    expect(screen.getByLabelText('Maximize')).toBeInTheDocument();
  });

  it('displays app icon and title', () => {
    render(<TitleBar />);

    // Check icon exists
    const icon = screen.getByAltText('vibespace');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('src', '/icon.png');

    // Check title text
    expect(screen.getByText('vibespace')).toBeInTheDocument();
  });

  it('calls minimize when minimize button is clicked', async () => {
    const user = userEvent.setup();
    render(<TitleBar />);

    const minimizeButton = screen.getByLabelText('Minimize');
    await user.click(minimizeButton);

    expect(mockMinimize).toHaveBeenCalledOnce();
  });

  it('calls toggleMaximize when maximize button is clicked', async () => {
    const user = userEvent.setup();
    render(<TitleBar />);

    const maximizeButton = screen.getByLabelText('Maximize');
    await user.click(maximizeButton);

    expect(mockToggleMaximize).toHaveBeenCalledOnce();
  });

  it('calls close when close button is clicked', async () => {
    const user = userEvent.setup();
    render(<TitleBar />);

    const closeButton = screen.getByLabelText('Close');
    await user.click(closeButton);

    expect(mockClose).toHaveBeenCalledOnce();
  });

  it('logs errors when window operations fail', async () => {
    const user = userEvent.setup();
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

    // Mock minimize to throw error
    mockMinimize.mockRejectedValueOnce(new Error('Failed to minimize'));

    render(<TitleBar />);
    const minimizeButton = screen.getByLabelText('Minimize');

    await user.click(minimizeButton);

    // Wait for error to be logged
    await waitFor(() => {
      expect(consoleErrorSpy).toHaveBeenCalledWith(
        '[TitleBar] Minimize error:',
        expect.any(Error)
      );
    });

    consoleErrorSpy.mockRestore();
  });
});
