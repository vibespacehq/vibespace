package tui

import (
	"testing"
)

func TestParseInputBareText(t *testing.T) {
	action, err := ParseInput("hello world", "default-vs")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	send, ok := action.(SendAction)
	if !ok {
		t.Fatalf("expected SendAction, got %T", action)
	}
	if send.Message != "hello world" {
		t.Fatalf("expected message 'hello world', got %q", send.Message)
	}
	if len(send.Targets) != 1 || send.Targets[0].Agent != "all" {
		t.Fatalf("expected broadcast to all, got %v", send.Targets)
	}
}

func TestParseInputCommand(t *testing.T) {
	tests := []struct {
		input string
		cmd   string
		args  []string
	}{
		{"/list", "list", nil},
		{"/clear", "clear", nil},
		{"/quit", "quit", nil},
		{"/add myspace", "add", []string{"myspace"}},
		{"/focus agent1", "focus", []string{"agent1"}},
	}
	for _, tt := range tests {
		action, err := ParseInput(tt.input, "")
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", tt.input, err)
		}
		cmd, ok := action.(CommandAction)
		if !ok {
			t.Fatalf("input %q: expected CommandAction, got %T", tt.input, action)
		}
		if cmd.Cmd != tt.cmd {
			t.Fatalf("input %q: expected cmd %q, got %q", tt.input, tt.cmd, cmd.Cmd)
		}
		if len(cmd.Args) != len(tt.args) {
			t.Fatalf("input %q: expected %d args, got %d", tt.input, len(tt.args), len(cmd.Args))
		}
	}
}

func TestParseInputMention(t *testing.T) {
	tests := []struct {
		input     string
		agent     string
		vibespace string
		msg       string
	}{
		{"@agent1 hello", "agent1", "default-vs", "hello"},
		{"@agent1@myspace hello", "agent1", "myspace", "hello"},
		{"@all hello everyone", "all", "default-vs", "hello everyone"},
		{"@all@myspace hello", "all", "myspace", "hello"},
	}
	for _, tt := range tests {
		action, err := ParseInput(tt.input, "default-vs")
		if err != nil {
			t.Fatalf("input %q: unexpected error: %v", tt.input, err)
		}
		send, ok := action.(SendAction)
		if !ok {
			t.Fatalf("input %q: expected SendAction, got %T", tt.input, action)
		}
		if len(send.Targets) != 1 {
			t.Fatalf("input %q: expected 1 target, got %d", tt.input, len(send.Targets))
		}
		if send.Targets[0].Agent != tt.agent {
			t.Fatalf("input %q: expected agent %q, got %q", tt.input, tt.agent, send.Targets[0].Agent)
		}
		if send.Targets[0].Vibespace != tt.vibespace {
			t.Fatalf("input %q: expected vibespace %q, got %q", tt.input, tt.vibespace, send.Targets[0].Vibespace)
		}
		if send.Message != tt.msg {
			t.Fatalf("input %q: expected message %q, got %q", tt.input, tt.msg, send.Message)
		}
	}
}

func TestParseInputEmpty(t *testing.T) {
	action, err := ParseInput("", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("expected nil action for empty input, got %v", action)
	}

	// Whitespace only
	action, err = ParseInput("   ", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action != nil {
		t.Fatalf("expected nil action for whitespace input, got %v", action)
	}
}

func TestParseInputErrors(t *testing.T) {
	// @agent without message
	_, err := ParseInput("@agent1", "default")
	if err == nil {
		t.Fatal("expected error for @agent without message")
	}

	// Empty command
	_, err = ParseInput("/", "default")
	if err == nil {
		t.Fatal("expected error for empty command")
	}

	// @ with empty agent name is parsed as mention with empty message
	_, err = ParseInput("@ hello", "default")
	if err == nil {
		t.Fatal("expected error for empty agent name")
	}
}

func TestParseTarget(t *testing.T) {
	tests := []struct {
		target    string
		defaultVS string
		agent     string
		vibespace string
	}{
		{"agent1", "default-vs", "agent1", "default-vs"},
		{"agent1@myspace", "default-vs", "agent1", "myspace"},
		{"all", "default-vs", "all", "default-vs"},
		{"all@myspace", "default-vs", "all", "myspace"},
	}
	for _, tt := range tests {
		targets, err := parseTarget(tt.target, tt.defaultVS)
		if err != nil {
			t.Fatalf("target %q: unexpected error: %v", tt.target, err)
		}
		if len(targets) != 1 {
			t.Fatalf("target %q: expected 1 target, got %d", tt.target, len(targets))
		}
		if targets[0].Agent != tt.agent {
			t.Fatalf("target %q: expected agent %q, got %q", tt.target, tt.agent, targets[0].Agent)
		}
		if targets[0].Vibespace != tt.vibespace {
			t.Fatalf("target %q: expected vibespace %q, got %q", tt.target, tt.vibespace, targets[0].Vibespace)
		}
	}

	// Error case: empty agent
	_, err := parseTarget("", "default")
	if err == nil {
		t.Fatal("expected error for empty target")
	}
}
