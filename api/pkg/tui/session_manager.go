package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"vibespace/pkg/session"

	"github.com/google/uuid"
)

// ClaudeSession represents a single Claude CLI session for an agent
type ClaudeSession struct {
	ID           string    `json:"id"`            // UUID for Claude's --session-id/--resume
	CreatedAt    time.Time `json:"created_at"`    // When session was created
	LastUsedAt   time.Time `json:"last_used_at"`  // Last message timestamp
	MessageCount int       `json:"message_count"` // Number of messages in this session
	Summary      string    `json:"summary"`       // Optional: first message or description
}

// AgentSessionState tracks all Claude sessions for a single agent
type AgentSessionState struct {
	AgentAddress     string          `json:"agent_address"`     // e.g., "claude-1@myproject"
	CurrentSessionID string          `json:"current_session_id"` // Active session ID
	Sessions         []ClaudeSession `json:"sessions"`          // All sessions (most recent first)
}

// ClaudeSessionManager manages Claude CLI sessions across all agents
// Thread-safe for concurrent access from multiple agent connections
type ClaudeSessionManager struct {
	mu       sync.RWMutex
	agents   map[string]*AgentSessionState // key: agent address string
	filePath string                         // persistence path
}

// NewClaudeSessionManager creates a new session manager
// Sessions are persisted to ~/.vibespace/claude_sessions.json
func NewClaudeSessionManager() *ClaudeSessionManager {
	homeDir, _ := os.UserHomeDir()
	filePath := filepath.Join(homeDir, ".vibespace", "claude_sessions.json")

	mgr := &ClaudeSessionManager{
		agents:   make(map[string]*AgentSessionState),
		filePath: filePath,
	}

	// Load existing sessions from disk
	mgr.load()

	return mgr
}

// GetOrCreateSession gets the current session for an agent, creating one if needed
// Returns (sessionID, isNew) where isNew indicates if --session-id should be used
func (m *ClaudeSessionManager) GetOrCreateSession(addr session.AgentAddress) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := addr.String()
	state, exists := m.agents[key]

	if !exists {
		// First time seeing this agent - create new state and session
		sessionID := uuid.New().String()
		state = &AgentSessionState{
			AgentAddress:     key,
			CurrentSessionID: sessionID,
			Sessions: []ClaudeSession{{
				ID:           sessionID,
				CreatedAt:    time.Now(),
				LastUsedAt:   time.Now(),
				MessageCount: 0,
			}},
		}
		m.agents[key] = state
		m.save()
		return sessionID, true // isNew=true, use --session-id
	}

	// Agent exists - check if current session has messages
	currentSession := m.findSession(state, state.CurrentSessionID)
	if currentSession == nil {
		// Current session not found (shouldn't happen) - create new one
		sessionID := uuid.New().String()
		state.CurrentSessionID = sessionID
		state.Sessions = append([]ClaudeSession{{
			ID:           sessionID,
			CreatedAt:    time.Now(),
			LastUsedAt:   time.Now(),
			MessageCount: 0,
		}}, state.Sessions...)
		m.save()
		return sessionID, true
	}

	if currentSession.MessageCount == 0 {
		// No messages yet in this session - still use --session-id
		return state.CurrentSessionID, true
	}

	// Session has messages - use --resume
	return state.CurrentSessionID, false
}

// RecordMessage records that a message was sent in the current session
func (m *ClaudeSessionManager) RecordMessage(addr session.AgentAddress, firstMessageSummary string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := addr.String()
	state, exists := m.agents[key]
	if !exists {
		return
	}

	currentSession := m.findSession(state, state.CurrentSessionID)
	if currentSession == nil {
		return
	}

	currentSession.MessageCount++
	currentSession.LastUsedAt = time.Now()

	// Store summary from first message
	if currentSession.MessageCount == 1 && firstMessageSummary != "" {
		// Truncate to first 100 chars
		if len(firstMessageSummary) > 100 {
			currentSession.Summary = firstMessageSummary[:100] + "..."
		} else {
			currentSession.Summary = firstMessageSummary
		}
	}

	m.save()
}

// NewSession creates a new Claude session for an agent
// Returns the new session ID
func (m *ClaudeSessionManager) NewSession(addr session.AgentAddress) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := addr.String()
	state, exists := m.agents[key]

	sessionID := uuid.New().String()
	newSession := ClaudeSession{
		ID:           sessionID,
		CreatedAt:    time.Now(),
		LastUsedAt:   time.Now(),
		MessageCount: 0,
	}

	if !exists {
		state = &AgentSessionState{
			AgentAddress:     key,
			CurrentSessionID: sessionID,
			Sessions:         []ClaudeSession{newSession},
		}
		m.agents[key] = state
	} else {
		// Prepend new session to history
		state.Sessions = append([]ClaudeSession{newSession}, state.Sessions...)
		state.CurrentSessionID = sessionID
	}

	m.save()
	return sessionID
}

// ResumeSession switches to an existing session by ID
// Returns error if session not found
func (m *ClaudeSessionManager) ResumeSession(addr session.AgentAddress, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := addr.String()
	state, exists := m.agents[key]
	if !exists {
		return fmt.Errorf("no sessions found for agent %s", key)
	}

	// Find the session
	found := false
	for _, s := range state.Sessions {
		if s.ID == sessionID || matchesShortID(s.ID, sessionID) {
			state.CurrentSessionID = s.ID
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("session %s not found for agent %s", sessionID, key)
	}

	m.save()
	return nil
}

// ListSessions returns all sessions for an agent
func (m *ClaudeSessionManager) ListSessions(addr session.AgentAddress) []ClaudeSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := addr.String()
	state, exists := m.agents[key]
	if !exists {
		return nil
	}

	// Return a copy
	result := make([]ClaudeSession, len(state.Sessions))
	copy(result, state.Sessions)
	return result
}

// GetCurrentSession returns info about the current session
func (m *ClaudeSessionManager) GetCurrentSession(addr session.AgentAddress) *ClaudeSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := addr.String()
	state, exists := m.agents[key]
	if !exists {
		return nil
	}

	return m.findSession(state, state.CurrentSessionID)
}

// GetCurrentSessionID returns just the current session ID for an agent
func (m *ClaudeSessionManager) GetCurrentSessionID(addr session.AgentAddress) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := addr.String()
	state, exists := m.agents[key]
	if !exists {
		return ""
	}
	return state.CurrentSessionID
}

// findSession finds a session by ID in the agent's session list
// Must be called with lock held
func (m *ClaudeSessionManager) findSession(state *AgentSessionState, sessionID string) *ClaudeSession {
	for i := range state.Sessions {
		if state.Sessions[i].ID == sessionID {
			return &state.Sessions[i]
		}
	}
	return nil
}

// matchesShortID checks if a full UUID matches a short ID prefix
// Allows users to type just first 8 chars of UUID
func matchesShortID(fullID, shortID string) bool {
	if len(shortID) < 8 {
		return false
	}
	return len(fullID) >= len(shortID) && fullID[:len(shortID)] == shortID
}

// load reads sessions from disk
func (m *ClaudeSessionManager) load() {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return // File doesn't exist yet, that's fine
	}

	var agents map[string]*AgentSessionState
	if err := json.Unmarshal(data, &agents); err != nil {
		return // Corrupted file, start fresh
	}

	m.agents = agents
}

// save writes sessions to disk
func (m *ClaudeSessionManager) save() {
	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(m.agents, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(m.filePath, data, 0644)
}

// FormatSessionList formats a list of sessions for display
func FormatSessionList(sessions []ClaudeSession, currentID string) string {
	if len(sessions) == 0 {
		return "No sessions found"
	}

	result := ""
	for _, s := range sessions {
		marker := "  "
		if s.ID == currentID {
			marker = "→ "
		}

		// Show short ID (first 8 chars)
		shortID := s.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		// Format time
		timeStr := s.LastUsedAt.Format("Jan 2 15:04")

		// Build line
		line := fmt.Sprintf("%s%s  %s  %d msgs", marker, shortID, timeStr, s.MessageCount)
		if s.Summary != "" {
			line += fmt.Sprintf("  \"%s\"", s.Summary)
		}
		result += line + "\n"
	}

	return result
}
