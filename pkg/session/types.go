package session

import (
	"fmt"
	"time"
)

// Session represents a multi-agent session that can span multiple vibespaces
type Session struct {
	Name       string           `json:"name"`
	CreatedAt  time.Time        `json:"created_at"`
	LastUsed   time.Time        `json:"last_used"`
	Vibespaces []VibespaceEntry `json:"vibespaces"`
	Layout     Layout           `json:"layout"`
}

// VibespaceEntry represents a vibespace and its agents in a session
type VibespaceEntry struct {
	Name   string   `json:"name"`
	Agents []string `json:"agents"` // empty = all agents
}

// Layout represents the TUI layout configuration
type Layout struct {
	Mode         LayoutMode `json:"mode"`          // "split" or "focus"
	FocusedAgent string     `json:"focused_agent"` // "claude-1@projectA"
}

// LayoutMode represents the TUI layout mode
type LayoutMode string

const (
	LayoutModeSplit LayoutMode = "split"
	LayoutModeFocus LayoutMode = "focus"
)

// AgentAddress represents a unique identifier for an agent in a vibespace
type AgentAddress struct {
	Agent     string
	Vibespace string
}

// String returns the string representation of an agent address (agent@vibespace)
func (a AgentAddress) String() string {
	return fmt.Sprintf("%s@%s", a.Agent, a.Vibespace)
}

// ParseAgentAddress parses an agent address string into an AgentAddress.
// Supports formats:
//   - "agent@vibespace" -> AgentAddress{Agent: "agent", Vibespace: "vibespace"}
//   - "agent" (with default vibespace) -> AgentAddress{Agent: "agent", Vibespace: defaultVS}
//   - "@agent@vibespace" -> strips leading @ (defensive against autocomplete)
func ParseAgentAddress(s string, defaultVibespace string) AgentAddress {
	// Strip any leading @ (defensive against autocomplete double-@)
	for len(s) > 0 && s[0] == '@' {
		s = s[1:]
	}

	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '@' {
			return AgentAddress{
				Agent:     s[:i],
				Vibespace: s[i+1:],
			}
		}
	}
	// No @ found, use default vibespace
	return AgentAddress{
		Agent:     s,
		Vibespace: defaultVibespace,
	}
}

// HasAgent checks if the session has a specific agent
func (s *Session) HasAgent(addr AgentAddress) bool {
	for _, vs := range s.Vibespaces {
		if vs.Name == addr.Vibespace {
			// If no specific agents listed, all agents are included
			if len(vs.Agents) == 0 {
				return true
			}
			for _, agent := range vs.Agents {
				if agent == addr.Agent {
					return true
				}
			}
		}
	}
	return false
}

// GetVibespaceEntry returns the VibespaceEntry for a given vibespace name
func (s *Session) GetVibespaceEntry(name string) *VibespaceEntry {
	for i := range s.Vibespaces {
		if s.Vibespaces[i].Name == name {
			return &s.Vibespaces[i]
		}
	}
	return nil
}

// AddVibespace adds a vibespace to the session
func (s *Session) AddVibespace(name string, agents []string) {
	// Check if already exists
	for i := range s.Vibespaces {
		if s.Vibespaces[i].Name == name {
			// Update agents
			if len(agents) > 0 {
				s.Vibespaces[i].Agents = mergeAgents(s.Vibespaces[i].Agents, agents)
			}
			return
		}
	}
	// Add new entry
	s.Vibespaces = append(s.Vibespaces, VibespaceEntry{
		Name:   name,
		Agents: agents,
	})
}

// RemoveVibespace removes a vibespace from the session
func (s *Session) RemoveVibespace(name string) {
	for i := range s.Vibespaces {
		if s.Vibespaces[i].Name == name {
			s.Vibespaces = append(s.Vibespaces[:i], s.Vibespaces[i+1:]...)
			return
		}
	}
}

// RemoveAgent removes a specific agent from a vibespace in the session
func (s *Session) RemoveAgent(addr AgentAddress) {
	for i := range s.Vibespaces {
		if s.Vibespaces[i].Name == addr.Vibespace {
			agents := s.Vibespaces[i].Agents
			for j := range agents {
				if agents[j] == addr.Agent {
					s.Vibespaces[i].Agents = append(agents[:j], agents[j+1:]...)
					return
				}
			}
		}
	}
}

// mergeAgents merges two agent lists without duplicates
func mergeAgents(existing, new []string) []string {
	set := make(map[string]bool)
	for _, a := range existing {
		set[a] = true
	}
	for _, a := range new {
		set[a] = true
	}
	result := make([]string, 0, len(set))
	for a := range set {
		result = append(result, a)
	}
	return result
}
