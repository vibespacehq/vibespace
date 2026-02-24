package cli

import (
	"testing"
	"time"

	"github.com/vibespacehq/vibespace/pkg/session"
)

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 2, "he"},
		{"max 3", "hello", 3, "hel"},
		{"max 4", "hello world", 4, "h..."},
		{"empty string", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateStr(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateStr(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"just now", now.Add(-30 * time.Second), "just now"},
		{"1 minute", now.Add(-90 * time.Second), "1 minute ago"},
		{"5 minutes", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"1 hour", now.Add(-90 * time.Minute), "1 hour ago"},
		{"3 hours", now.Add(-3 * time.Hour), "3 hours ago"},
		{"yesterday", now.Add(-36 * time.Hour), "yesterday"},
		{"3 days", now.Add(-3 * 24 * time.Hour), "3 days ago"},
		{"old date", now.Add(-30 * 24 * time.Hour), now.Add(-30 * 24 * time.Hour).Format("2006-01-02")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeTime(tt.t)
			if got != tt.want {
				t.Errorf("formatRelativeTime() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name string
		strs []string
		sep  string
		want string
	}{
		{"empty", nil, ", ", ""},
		{"single", []string{"a"}, ", ", "a"},
		{"two", []string{"a", "b"}, ", ", "a, b"},
		{"three", []string{"a", "b", "c"}, " | ", "a | b | c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := joinStrings(tt.strs, tt.sep)
			if got != tt.want {
				t.Errorf("joinStrings(%v, %q) = %q, want %q", tt.strs, tt.sep, got, tt.want)
			}
		})
	}
}

func TestFormatSessionInfo(t *testing.T) {
	t.Run("empty session", func(t *testing.T) {
		sess := session.Session{}
		vs, agents := formatSessionInfo(sess)
		if vs != "(empty)" {
			t.Errorf("vibespaces = %q, want %q", vs, "(empty)")
		}
		if agents != "-" {
			t.Errorf("agents = %q, want %q", agents, "-")
		}
	})

	t.Run("single vibespace no agents", func(t *testing.T) {
		sess := session.Session{
			Vibespaces: []session.VibespaceEntry{
				{Name: "myproject"},
			},
		}
		vs, agents := formatSessionInfo(sess)
		if vs != "myproject" {
			t.Errorf("vibespaces = %q, want %q", vs, "myproject")
		}
		if agents != "all" {
			t.Errorf("agents = %q, want %q", agents, "all")
		}
	})

	t.Run("with specific agents", func(t *testing.T) {
		sess := session.Session{
			Vibespaces: []session.VibespaceEntry{
				{Name: "proj1", Agents: []string{"claude-1"}},
			},
		}
		vs, agents := formatSessionInfo(sess)
		if vs != "proj1" {
			t.Errorf("vibespaces = %q, want %q", vs, "proj1")
		}
		if agents != "claude-1@proj1" {
			t.Errorf("agents = %q, want %q", agents, "claude-1@proj1")
		}
	})

	t.Run("multiple vibespaces", func(t *testing.T) {
		sess := session.Session{
			Vibespaces: []session.VibespaceEntry{
				{Name: "proj1"},
				{Name: "proj2"},
			},
		}
		vs, _ := formatSessionInfo(sess)
		if vs != "proj1, proj2" {
			t.Errorf("vibespaces = %q, want %q", vs, "proj1, proj2")
		}
	})
}
