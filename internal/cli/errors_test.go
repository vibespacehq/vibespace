package cli

import (
	"testing"

	"github.com/vibespacehq/vibespace/pkg/daemon"
	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

func TestFormatAvailableAgents(t *testing.T) {
	tests := []struct {
		name   string
		agents []daemon.AgentStatus
		want   string
	}{
		{"nil", nil, "(none)"},
		{"empty", []daemon.AgentStatus{}, "(none)"},
		{"single", []daemon.AgentStatus{{Name: "claude-1"}}, "claude-1"},
		{"multiple", []daemon.AgentStatus{{Name: "claude-1"}, {Name: "claude-2"}}, "claude-1, claude-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAvailableAgents(tt.agents)
			if got != tt.want {
				t.Errorf("formatAvailableAgents() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatAllAgents(t *testing.T) {
	tests := []struct {
		name   string
		agents []vibespace.AgentInfo
		want   string
	}{
		{"nil", nil, "(none)"},
		{"empty", []vibespace.AgentInfo{}, "(none)"},
		{"running", []vibespace.AgentInfo{{AgentName: "claude-1", Status: "running"}}, "claude-1"},
		{"not running", []vibespace.AgentInfo{{AgentName: "claude-1", Status: "stopped"}}, "claude-1 (stopped)"},
		{"mixed", []vibespace.AgentInfo{
			{AgentName: "claude-1", Status: "running"},
			{AgentName: "claude-2", Status: "creating"},
		}, "claude-1, claude-2 (creating)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAllAgents(tt.agents)
			if got != tt.want {
				t.Errorf("formatAllAgents() = %q, want %q", got, tt.want)
			}
		})
	}
}
