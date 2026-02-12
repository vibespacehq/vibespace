package tui

import (
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/session"
)

// SharedState holds data shared across all tabs.
type SharedState struct {
	SessionStore *session.Store
	Daemon       *daemon.Client

	// Cached status (refreshed async via Refresh)
	DaemonRunning bool
	DaemonPid     int
	DaemonUptime  string
}

// NewSharedState creates clients for shared services.
// Failures are non-fatal — tabs degrade gracefully.
func NewSharedState() *SharedState {
	s := &SharedState{}

	if store, err := session.NewStore(); err == nil {
		s.SessionStore = store
	} else {
		slog.Debug("shared state: session store unavailable", "err", err)
	}

	if dc, err := daemon.NewClient(); err == nil {
		s.Daemon = dc
	} else {
		slog.Debug("shared state: daemon client unavailable", "err", err)
	}

	return s
}

// SharedStateRefreshedMsg is sent after Refresh completes.
type SharedStateRefreshedMsg struct{}

// Refresh updates all cached fields from live services.
func (s *SharedState) Refresh() tea.Msg {
	if s.Daemon != nil {
		if status, err := s.Daemon.DaemonStatus(); err == nil {
			s.DaemonRunning = true
			s.DaemonPid = status.Pid
			s.DaemonUptime = status.Uptime
		} else {
			s.DaemonRunning = false
			s.DaemonPid = 0
			s.DaemonUptime = ""
		}
	}
	return SharedStateRefreshedMsg{}
}

// refreshSharedState returns a Cmd that refreshes the shared state.
func refreshSharedState(s *SharedState) tea.Cmd {
	return func() tea.Msg {
		return s.Refresh()
	}
}
