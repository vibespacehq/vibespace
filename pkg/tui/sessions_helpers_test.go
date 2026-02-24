package tui

// Tests for sessVibespaceNames, sessAgentCount, and timeAgo are in tab_sessions_test.go.
// This file adds additional edge case tests.

import (
	"testing"

	"github.com/vibespacehq/vibespace/pkg/session"
)

func TestTruncateSessionHelper(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hell…"},
		{"line1\nline2", 20, "line1"},
		{"", 5, ""},
		{"multiline\ntext\nhere", 5, "mult…"},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestSessAgentCountMultipleVibespaces(t *testing.T) {
	// Test that agents across multiple vibespaces are deduplicated
	sess := makeSessionWithAgents(
		[]string{"project-a", "project-b"},
		[][]string{{"claude-1", "claude-2"}, {"claude-1", "claude-3"}},
	)
	got := sessAgentCount(sess)
	if got != "3" {
		t.Errorf("sessAgentCount with dedup = %q, want %q", got, "3")
	}
}

func makeSessionWithAgents(vibespaces []string, agents [][]string) session.Session {
	sess := session.Session{}
	for i, vs := range vibespaces {
		entry := session.VibespaceEntry{Name: vs}
		if i < len(agents) {
			entry.Agents = agents[i]
		}
		sess.Vibespaces = append(sess.Vibespaces, entry)
	}
	return sess
}
