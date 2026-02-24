package portforward

import "testing"

func TestCalculateLocalPort(t *testing.T) {
	tests := []struct {
		name       string
		agentNum   int
		remotePort int
		want       int
	}{
		{"agent 1 ssh", 1, 22, 22},
		{"agent 1 ttyd", 1, 7681, 7681},
		{"agent 1 http", 1, 8080, 8080},
		{"agent 2 ssh", 2, 22, 10022},
		{"agent 2 ttyd", 2, 7681, 17681},
		{"agent 3 http", 3, 3000, 23000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateLocalPort(tt.agentNum, tt.remotePort)
			if got != tt.want {
				t.Errorf("CalculateLocalPort(%d, %d) = %d, want %d", tt.agentNum, tt.remotePort, got, tt.want)
			}
		})
	}
}

func TestParseAgentNumber(t *testing.T) {
	tests := []struct {
		name      string
		agentName string
		want      int
	}{
		{"claude-1", "claude-1", 1},
		{"claude-2", "claude-2", 2},
		{"claude-3", "claude-3", 3},
		{"claude-10", "claude-10", 10},
		{"invalid name", "invalid", 1},
		{"empty string", "", 1},
		{"no number", "claude-", 1},
		{"zero", "claude-0", 1},
		{"negative", "claude--1", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseAgentNumber(tt.agentName)
			if got != tt.want {
				t.Errorf("ParseAgentNumber(%q) = %d, want %d", tt.agentName, got, tt.want)
			}
		})
	}
}

func TestForwardStatusConstants(t *testing.T) {
	// Verify all statuses are distinct
	statuses := []ForwardStatus{StatusPending, StatusActive, StatusStopped, StatusError, StatusReconnecting}
	seen := make(map[ForwardStatus]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("duplicate status: %s", s)
		}
		seen[s] = true
	}
}

func TestForwardTypeConstants(t *testing.T) {
	types := []ForwardType{TypeSSH, TypeTTYD, TypeManual, TypePermission}
	seen := make(map[ForwardType]bool)
	for _, ft := range types {
		if seen[ft] {
			t.Errorf("duplicate type: %s", ft)
		}
		seen[ft] = true
	}
}
