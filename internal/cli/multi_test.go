package cli

import (
	"testing"
	"time"

	"github.com/vibespacehq/vibespace/pkg/tui"
)

func TestBuildSessionDefaults(t *testing.T) {
	sess := buildSession("", nil, nil)
	if sess.Name == "" {
		t.Error("empty name should generate a UUID")
	}
	if len(sess.Vibespaces) != 0 {
		t.Errorf("expected 0 vibespaces, got %d", len(sess.Vibespaces))
	}
	if sess.Layout.Mode != "split" {
		t.Errorf("Layout.Mode = %q, want %q", sess.Layout.Mode, "split")
	}
}

func TestBuildSessionWithName(t *testing.T) {
	sess := buildSession("my-session", nil, nil)
	if sess.Name != "my-session" {
		t.Errorf("Name = %q, want %q", sess.Name, "my-session")
	}
}

func TestBuildSessionWithVibespaces(t *testing.T) {
	sess := buildSession("test", []string{"project-a", "project-b"}, nil)
	if len(sess.Vibespaces) != 2 {
		t.Fatalf("expected 2 vibespaces, got %d", len(sess.Vibespaces))
	}
	if sess.Vibespaces[0].Name != "project-a" {
		t.Errorf("Vibespaces[0].Name = %q, want %q", sess.Vibespaces[0].Name, "project-a")
	}
	if sess.Vibespaces[1].Name != "project-b" {
		t.Errorf("Vibespaces[1].Name = %q, want %q", sess.Vibespaces[1].Name, "project-b")
	}
}

func TestBuildSessionWithAgents(t *testing.T) {
	// Agent addresses use @ format: agent@vibespace
	sess := buildSession("test", nil, []string{"claude-1@project-a"})
	if len(sess.Vibespaces) != 1 {
		t.Fatalf("expected 1 vibespace, got %d", len(sess.Vibespaces))
	}
	if sess.Vibespaces[0].Name != "project-a" {
		t.Errorf("Vibespaces[0].Name = %q, want %q", sess.Vibespaces[0].Name, "project-a")
	}
}

func TestConvertMultiResponse(t *testing.T) {
	now := time.Now()
	resp := &tui.MultiResponse{
		Session: "test-session",
		Request: tui.RequestInfo{
			Target:  "claude-1",
			Message: "hello",
		},
		Responses: []tui.AgentResponse{
			{
				Agent:     "claude-1",
				Timestamp: now,
				Content:   "hi there",
				ToolUses: []tui.ToolUse{
					{Tool: "Bash", Input: "ls"},
				},
			},
			{
				Agent:     "claude-2",
				Timestamp: now.Add(time.Second),
				Content:   "hello too",
				Error:     "timeout",
			},
		},
	}

	output := convertMultiResponse(resp)

	if output.Session != "test-session" {
		t.Errorf("Session = %q, want %q", output.Session, "test-session")
	}
	if output.Request.Target != "claude-1" {
		t.Errorf("Request.Target = %q, want %q", output.Request.Target, "claude-1")
	}
	if output.Request.Message != "hello" {
		t.Errorf("Request.Message = %q, want %q", output.Request.Message, "hello")
	}
	if len(output.Responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(output.Responses))
	}

	r0 := output.Responses[0]
	if r0.Agent != "claude-1" {
		t.Errorf("Responses[0].Agent = %q, want %q", r0.Agent, "claude-1")
	}
	if r0.Content != "hi there" {
		t.Errorf("Responses[0].Content = %q, want %q", r0.Content, "hi there")
	}
	if len(r0.ToolUses) != 1 {
		t.Fatalf("expected 1 tool use, got %d", len(r0.ToolUses))
	}
	if r0.ToolUses[0].Tool != "Bash" {
		t.Errorf("ToolUses[0].Tool = %q, want %q", r0.ToolUses[0].Tool, "Bash")
	}

	r1 := output.Responses[1]
	if r1.Error != "timeout" {
		t.Errorf("Responses[1].Error = %q, want %q", r1.Error, "timeout")
	}
}

func TestConvertMultiResponseEmpty(t *testing.T) {
	resp := &tui.MultiResponse{
		Session:   "empty",
		Request:   tui.RequestInfo{},
		Responses: nil,
	}

	output := convertMultiResponse(resp)
	if output.Session != "empty" {
		t.Errorf("Session = %q, want %q", output.Session, "empty")
	}
	if len(output.Responses) != 0 {
		t.Errorf("expected 0 responses, got %d", len(output.Responses))
	}
}
