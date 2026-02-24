package tui

// Tests for parseHistoryJSONL and parseCodexHistory are in tab_vibespaces_test.go.
// This file adds additional edge case tests and tests for primaryAgent.

import (
	"testing"

	"github.com/vibespacehq/vibespace/pkg/vibespace"
)

func TestPrimaryAgent(t *testing.T) {
	tests := []struct {
		name   string
		agents []vibespace.AgentInfo
		want   string // expected agent name, or "" for nil
	}{
		{
			name:   "empty",
			agents: nil,
			want:   "",
		},
		{
			name: "single primary",
			agents: []vibespace.AgentInfo{
				{AgentName: "claude-1", AgentNum: 1},
			},
			want: "claude-1",
		},
		{
			name: "multiple with primary",
			agents: []vibespace.AgentInfo{
				{AgentName: "claude-2", AgentNum: 2},
				{AgentName: "claude-1", AgentNum: 1},
			},
			want: "claude-1",
		},
		{
			name: "no primary returns first",
			agents: []vibespace.AgentInfo{
				{AgentName: "claude-3", AgentNum: 3},
				{AgentName: "claude-2", AgentNum: 2},
			},
			want: "claude-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := primaryAgent(tt.agents)
			if tt.want == "" {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
			} else {
				if got == nil {
					t.Fatalf("expected %q, got nil", tt.want)
				}
				if got.AgentName != tt.want {
					t.Errorf("primaryAgent().AgentName = %q, want %q", got.AgentName, tt.want)
				}
			}
		})
	}
}

func TestParseHistoryJSONLEmpty(t *testing.T) {
	sessions := parseHistoryJSONL(nil, "/myproject")
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestParseHistoryJSONLCorrupt(t *testing.T) {
	input := []byte("{invalid\n{\"display\":\"ok\",\"timestamp\":1000,\"project\":\"/p\",\"sessionId\":\"s1\"}\n")
	sessions := parseHistoryJSONL(input, "/p")
	if len(sessions) != 1 {
		t.Errorf("expected 1 session (corrupt skipped), got %d", len(sessions))
	}
}

func TestParseCodexHistoryEmpty(t *testing.T) {
	sessions := parseCodexHistory(nil)
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}
