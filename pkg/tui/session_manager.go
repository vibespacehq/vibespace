package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// AgentSessions tracks Claude sessions for one agent within a multi-session
type AgentSessions struct {
	Current string   `json:"current"` // Currently active session ID
	History []string `json:"history"` // All session IDs (most recent first)
}

// ClaudeSessionManager manages Claude CLI sessions across all agents and multi-sessions
// Thread-safe for concurrent access from multiple agent connections
// Sessions are keyed by (multiSessionID, agentAddress) to isolate sessions between
// different multi-sessions, preventing stale session IDs from being reused
type ClaudeSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]map[string]*AgentSessions // multiSessionID -> agentAddr -> sessions
	filePath string                               // persistence path
}

// NewClaudeSessionManager creates a new session manager
// Sessions are persisted to ~/.vibespace/claude_sessions.json
func NewClaudeSessionManager() *ClaudeSessionManager {
	homeDir, _ := os.UserHomeDir()
	filePath := filepath.Join(homeDir, ".vibespace", "claude_sessions.json")

	mgr := &ClaudeSessionManager{
		sessions: make(map[string]map[string]*AgentSessions),
		filePath: filePath,
	}

	// Load existing sessions from disk
	mgr.load()

	return mgr
}

// GetOrCreateSession gets the current session for an agent within a multi-session,
// creating a new session if one doesn't exist.
// This is used when starting a NEW multi-session (resume=false).
// Returns the session ID to use with --session-id flag.
func (m *ClaudeSessionManager) GetOrCreateSession(multiSessionID, agentAddr string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure multi-session map exists
	if m.sessions[multiSessionID] == nil {
		m.sessions[multiSessionID] = make(map[string]*AgentSessions)
	}

	// Check if agent already has sessions in this multi-session
	agentSessions, exists := m.sessions[multiSessionID][agentAddr]
	if !exists || agentSessions == nil || agentSessions.Current == "" {
		// Create new session
		sessionID := uuid.New().String()
		m.sessions[multiSessionID][agentAddr] = &AgentSessions{
			Current: sessionID,
			History: []string{sessionID},
		}
		m.save()
		return sessionID
	}

	return agentSessions.Current
}

// GetSession gets the current session ID for an agent within a multi-session.
// This is used when RESUMING an existing multi-session (resume=true).
// Returns empty string if no session exists (caller should then use GetOrCreateSession).
func (m *ClaudeSessionManager) GetSession(multiSessionID, agentAddr string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.sessions[multiSessionID] == nil {
		return ""
	}

	agentSessions := m.sessions[multiSessionID][agentAddr]
	if agentSessions == nil {
		return ""
	}

	return agentSessions.Current
}

// NewSession creates a new Claude session for an agent within a multi-session.
// Used by /session @agent new command to start a fresh conversation.
// Returns the new session ID.
func (m *ClaudeSessionManager) NewSession(multiSessionID, agentAddr string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure multi-session map exists
	if m.sessions[multiSessionID] == nil {
		m.sessions[multiSessionID] = make(map[string]*AgentSessions)
	}

	sessionID := uuid.New().String()

	agentSessions := m.sessions[multiSessionID][agentAddr]
	if agentSessions == nil {
		m.sessions[multiSessionID][agentAddr] = &AgentSessions{
			Current: sessionID,
			History: []string{sessionID},
		}
	} else {
		// Prepend new session to history
		agentSessions.History = append([]string{sessionID}, agentSessions.History...)
		agentSessions.Current = sessionID
	}

	m.save()
	return sessionID
}

// ResumeSession switches to an existing session by ID within a multi-session.
// The session ID must exist in the agent's history.
// Used by /session @agent resume <id> command.
// Returns error if session not found in history.
func (m *ClaudeSessionManager) ResumeSession(multiSessionID, agentAddr, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sessions[multiSessionID] == nil {
		return fmt.Errorf("no sessions found for multi-session %s", multiSessionID)
	}

	agentSessions := m.sessions[multiSessionID][agentAddr]
	if agentSessions == nil {
		return fmt.Errorf("no sessions found for agent %s", agentAddr)
	}

	// Find the session in history (support short ID prefix matching)
	found := false
	for _, id := range agentSessions.History {
		if id == sessionID || matchesShortID(id, sessionID) {
			agentSessions.Current = id
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("session %s not found for agent %s", sessionID, agentAddr)
	}

	m.save()
	return nil
}

// ListSessions returns all session IDs for an agent within a multi-session.
// Used by /session @agent list command.
func (m *ClaudeSessionManager) ListSessions(multiSessionID, agentAddr string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.sessions[multiSessionID] == nil {
		return nil
	}

	agentSessions := m.sessions[multiSessionID][agentAddr]
	if agentSessions == nil {
		return nil
	}

	// Return a copy
	result := make([]string, len(agentSessions.History))
	copy(result, agentSessions.History)
	return result
}

// GetCurrentSessionID returns the current session ID for an agent within a multi-session.
// Used by /session @agent info command.
func (m *ClaudeSessionManager) GetCurrentSessionID(multiSessionID, agentAddr string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.sessions[multiSessionID] == nil {
		return ""
	}

	agentSessions := m.sessions[multiSessionID][agentAddr]
	if agentSessions == nil {
		return ""
	}

	return agentSessions.Current
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

	var sessions map[string]map[string]*AgentSessions
	if err := json.Unmarshal(data, &sessions); err != nil {
		return // Corrupted file, start fresh
	}

	m.sessions = sessions
}

// save writes sessions to disk
func (m *ClaudeSessionManager) save() {
	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	os.MkdirAll(dir, 0755)

	data, err := json.MarshalIndent(m.sessions, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(m.filePath, data, 0644)
}

// FormatSessionList formats a list of sessions for display
func FormatSessionList(sessions []string, currentID string) string {
	if len(sessions) == 0 {
		return "No sessions found"
	}

	result := ""
	for _, id := range sessions {
		marker := "  "
		if id == currentID {
			marker = "→ "
		}

		// Show short ID (first 8 chars)
		shortID := id
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		result += fmt.Sprintf("%s%s\n", marker, shortID)
	}

	return result
}
