package tui

import (
	"log/slog"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/k8s"
	"github.com/vibespacehq/vibespace/pkg/metrics"
	"github.com/vibespacehq/vibespace/pkg/session"
	"github.com/vibespacehq/vibespace/pkg/update"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

// SharedState holds data shared across all tabs.
type SharedState struct {
	SessionStore *session.Store
	HistoryStore *HistoryStore
	Daemon       *daemon.Client
	Vibespace    *vibespace.Service
	Metrics      *metrics.Fetcher

	// Version info (set at build time)
	Version   string
	Commit    string
	BuildDate string

	// Cached status (refreshed async via Refresh)
	mu              sync.RWMutex
	DaemonRunning   bool
	DaemonPid       int
	DaemonUptime    string
	UpdateAvailable bool
	LatestVersion   string
}

// NewSharedState creates clients for shared services.
// Failures are non-fatal — tabs degrade gracefully.
func NewSharedState(version, commit, buildDate string) *SharedState {
	s := &SharedState{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
	}

	if store, err := session.NewStore(); err == nil {
		s.SessionStore = store
	} else {
		slog.Debug("shared state: session store unavailable", "err", err)
	}

	if hs, err := NewHistoryStore(); err == nil {
		s.HistoryStore = hs
	} else {
		slog.Debug("shared state: history store unavailable", "err", err)
	}

	if dc, err := daemon.NewClient(); err == nil {
		s.Daemon = dc
	} else {
		slog.Debug("shared state: daemon client unavailable", "err", err)
	}

	// Vibespace service requires a k8s client.
	// resolveKubeconfig is in internal/cli, so we rely on KUBECONFIG being set
	// (which it is when the TUI is launched via CLI commands).
	var kc *k8s.Client
	if c, err := k8s.NewClient(); err == nil {
		kc = c
		s.Vibespace = vibespace.NewService(kc)
		s.Metrics = metrics.NewFetcher(kc)
	} else {
		// Still create a nil-client service for graceful degradation
		s.Vibespace = vibespace.NewService(nil)
		slog.Debug("shared state: k8s client unavailable, vibespace service degraded", "err", err)
	}

	return s
}

// SharedStateRefreshedMsg is sent after Refresh completes.
type SharedStateRefreshedMsg struct{}

// Refresh updates all cached fields from live services.
// If the k8s client was unavailable at startup (no cluster), it retries.
func (s *SharedState) Refresh() tea.Msg {
	// Retry k8s client if cluster wasn't available at startup
	s.mu.Lock()
	if s.Metrics == nil {
		if kc, err := k8s.NewClient(); err == nil {
			s.Vibespace = vibespace.NewService(kc)
			s.Metrics = metrics.NewFetcher(kc)
			slog.Debug("shared state: k8s client now available after cluster init")
		}
	}
	s.mu.Unlock()

	// Retry daemon client if unavailable at startup
	if s.Daemon == nil {
		if dc, err := daemon.NewClient(); err == nil {
			s.Daemon = dc
			slog.Debug("shared state: daemon client now available")
		}
	}

	if s.Daemon != nil {
		if status, err := s.Daemon.DaemonStatus(); err == nil {
			s.mu.Lock()
			s.DaemonRunning = true
			s.DaemonPid = status.Pid
			s.DaemonUptime = status.Uptime
			s.mu.Unlock()
		} else {
			s.mu.Lock()
			s.DaemonRunning = false
			s.DaemonPid = 0
			s.DaemonUptime = ""
			s.mu.Unlock()
		}
	}
	// Check for updates (uses 24h cache, non-blocking)
	if info := update.CheckForUpdate(s.Version); info != nil {
		s.mu.Lock()
		s.UpdateAvailable = true
		s.LatestVersion = info.LatestVersion
		s.mu.Unlock()
	}

	return SharedStateRefreshedMsg{}
}

// IsDaemonRunning returns the cached daemon running status (thread-safe).
func (s *SharedState) IsDaemonRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DaemonRunning
}

// GetUpdateInfo returns the latest version if an update is available (thread-safe).
func (s *SharedState) GetUpdateInfo() (available bool, version string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.UpdateAvailable, s.LatestVersion
}

// refreshSharedState returns a Cmd that refreshes the shared state.
func refreshSharedState(s *SharedState) tea.Cmd {
	return func() tea.Msg {
		return s.Refresh()
	}
}
