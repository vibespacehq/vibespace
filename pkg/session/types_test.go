package session

import (
	"sort"
	"testing"
)

func TestAgentAddressString(t *testing.T) {
	addr := AgentAddress{Agent: "claude-1", Vibespace: "myproject"}
	if s := addr.String(); s != "claude-1@myproject" {
		t.Errorf("String() = %q, want %q", s, "claude-1@myproject")
	}
}

func TestParseAgentAddress(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		defaultVS string
		wantAgent string
		wantVS    string
	}{
		{"agent@vibespace", "claude-1@myproject", "", "claude-1", "myproject"},
		{"agent only with default", "claude-1", "fallback", "claude-1", "fallback"},
		{"agent only no default", "claude-1", "", "claude-1", ""},
		{"leading @", "@claude-1@myproject", "", "claude-1", "myproject"},
		{"double leading @@", "@@claude-1@myproject", "", "claude-1", "myproject"},
		{"empty string", "", "fallback", "", "fallback"},
		{"multiple @ picks last", "a@b@c", "", "a@b", "c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr := ParseAgentAddress(tt.input, tt.defaultVS)
			if addr.Agent != tt.wantAgent {
				t.Errorf("Agent = %q, want %q", addr.Agent, tt.wantAgent)
			}
			if addr.Vibespace != tt.wantVS {
				t.Errorf("Vibespace = %q, want %q", addr.Vibespace, tt.wantVS)
			}
		})
	}
}

func TestParseAgentAddressRoundTrip(t *testing.T) {
	original := AgentAddress{Agent: "claude-1", Vibespace: "myproject"}
	parsed := ParseAgentAddress(original.String(), "")
	if parsed != original {
		t.Errorf("round trip failed: got %+v, want %+v", parsed, original)
	}
}

func TestSessionHasAgent(t *testing.T) {
	s := &Session{
		Vibespaces: []VibespaceEntry{
			{Name: "project-a", Agents: []string{"claude-1", "claude-2"}},
			{Name: "project-b", Agents: []string{}}, // empty = all agents
		},
	}

	tests := []struct {
		name string
		addr AgentAddress
		want bool
	}{
		{"found exact", AgentAddress{"claude-1", "project-a"}, true},
		{"found second", AgentAddress{"claude-2", "project-a"}, true},
		{"wrong agent", AgentAddress{"claude-3", "project-a"}, false},
		{"wrong vibespace", AgentAddress{"claude-1", "project-c"}, false},
		{"wildcard (empty agents)", AgentAddress{"anything", "project-b"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.HasAgent(tt.addr); got != tt.want {
				t.Errorf("HasAgent(%+v) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}

func TestSessionHasAgentEmptySession(t *testing.T) {
	s := &Session{}
	if s.HasAgent(AgentAddress{"claude-1", "project"}) {
		t.Error("empty session should not have any agent")
	}
}

func TestGetVibespaceEntry(t *testing.T) {
	s := &Session{
		Vibespaces: []VibespaceEntry{
			{Name: "alpha", Agents: []string{"a1"}},
			{Name: "beta", Agents: []string{"b1"}},
		},
	}

	entry := s.GetVibespaceEntry("alpha")
	if entry == nil {
		t.Fatal("GetVibespaceEntry(alpha) returned nil")
	}
	if entry.Name != "alpha" {
		t.Errorf("entry.Name = %q, want %q", entry.Name, "alpha")
	}

	if s.GetVibespaceEntry("nonexistent") != nil {
		t.Error("GetVibespaceEntry(nonexistent) should return nil")
	}
}

func TestAddVibespace(t *testing.T) {
	t.Run("add new", func(t *testing.T) {
		s := &Session{}
		s.AddVibespace("project-a", []string{"claude-1"})

		if len(s.Vibespaces) != 1 {
			t.Fatalf("Vibespaces length = %d, want 1", len(s.Vibespaces))
		}
		if s.Vibespaces[0].Name != "project-a" {
			t.Errorf("Name = %q, want %q", s.Vibespaces[0].Name, "project-a")
		}
		if len(s.Vibespaces[0].Agents) != 1 || s.Vibespaces[0].Agents[0] != "claude-1" {
			t.Errorf("Agents = %v, want [claude-1]", s.Vibespaces[0].Agents)
		}
	})

	t.Run("update existing merges agents", func(t *testing.T) {
		s := &Session{
			Vibespaces: []VibespaceEntry{
				{Name: "project-a", Agents: []string{"claude-1"}},
			},
		}
		s.AddVibespace("project-a", []string{"claude-2"})

		if len(s.Vibespaces) != 1 {
			t.Fatalf("should still have 1 vibespace, got %d", len(s.Vibespaces))
		}

		agents := s.Vibespaces[0].Agents
		sort.Strings(agents)
		if len(agents) != 2 {
			t.Fatalf("Agents length = %d, want 2", len(agents))
		}
	})

	t.Run("update existing with empty agents is noop", func(t *testing.T) {
		s := &Session{
			Vibespaces: []VibespaceEntry{
				{Name: "project-a", Agents: []string{"claude-1"}},
			},
		}
		s.AddVibespace("project-a", []string{})

		if len(s.Vibespaces[0].Agents) != 1 {
			t.Errorf("empty agents should not modify existing, got %v", s.Vibespaces[0].Agents)
		}
	})
}

func TestRemoveVibespace(t *testing.T) {
	s := &Session{
		Vibespaces: []VibespaceEntry{
			{Name: "alpha"},
			{Name: "beta"},
			{Name: "gamma"},
		},
	}

	s.RemoveVibespace("beta")
	if len(s.Vibespaces) != 2 {
		t.Fatalf("Vibespaces length = %d, want 2", len(s.Vibespaces))
	}
	for _, vs := range s.Vibespaces {
		if vs.Name == "beta" {
			t.Error("beta should have been removed")
		}
	}

	// Removing non-existent is a no-op
	s.RemoveVibespace("nonexistent")
	if len(s.Vibespaces) != 2 {
		t.Errorf("removing non-existent should be no-op, got length %d", len(s.Vibespaces))
	}
}

func TestRemoveAgent(t *testing.T) {
	s := &Session{
		Vibespaces: []VibespaceEntry{
			{Name: "project-a", Agents: []string{"claude-1", "claude-2", "claude-3"}},
		},
	}

	s.RemoveAgent(AgentAddress{Agent: "claude-2", Vibespace: "project-a"})
	agents := s.Vibespaces[0].Agents
	if len(agents) != 2 {
		t.Fatalf("Agents length = %d, want 2", len(agents))
	}
	for _, a := range agents {
		if a == "claude-2" {
			t.Error("claude-2 should have been removed")
		}
	}

	// Removing from wrong vibespace is a no-op
	s.RemoveAgent(AgentAddress{Agent: "claude-1", Vibespace: "wrong"})
	if len(s.Vibespaces[0].Agents) != 2 {
		t.Error("removing from wrong vibespace should be no-op")
	}

	// Removing non-existent agent is a no-op
	s.RemoveAgent(AgentAddress{Agent: "claude-99", Vibespace: "project-a"})
	if len(s.Vibespaces[0].Agents) != 2 {
		t.Error("removing non-existent agent should be no-op")
	}
}

func TestMergeAgents(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		new      []string
		wantLen  int
	}{
		{"no overlap", []string{"a", "b"}, []string{"c", "d"}, 4},
		{"with duplicates", []string{"a", "b"}, []string{"b", "c"}, 3},
		{"all duplicates", []string{"a", "b"}, []string{"a", "b"}, 2},
		{"empty existing", []string{}, []string{"a", "b"}, 2},
		{"empty new", []string{"a", "b"}, []string{}, 2},
		{"both empty", []string{}, []string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeAgents(tt.existing, tt.new)
			if len(result) != tt.wantLen {
				t.Errorf("mergeAgents(%v, %v) returned %d items, want %d: %v",
					tt.existing, tt.new, len(result), tt.wantLen, result)
			}

			// Verify all inputs are present
			set := make(map[string]bool)
			for _, r := range result {
				set[r] = true
			}
			for _, a := range tt.existing {
				if !set[a] {
					t.Errorf("missing existing agent %q in result", a)
				}
			}
			for _, a := range tt.new {
				if !set[a] {
					t.Errorf("missing new agent %q in result", a)
				}
			}
		})
	}
}
