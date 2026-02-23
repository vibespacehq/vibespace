package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vibespacehq/vibespace/pkg/remote"
)

func newTestRemoteTab() *RemoteTab {
	shared := &SharedState{}
	tab := NewRemoteTab(shared)
	tab.SetSize(120, 40)
	return tab
}

func TestStripCIDR(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"10.100.0.2/32", "10.100.0.2"},
		{"10.100.0.1/24", "10.100.0.1"},
		{"10.100.0.2", "10.100.0.2"},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripCIDR(tt.input)
		if got != tt.want {
			t.Errorf("stripCIDR(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com:7780", "example.com"},
		{"192.168.1.1:443", "192.168.1.1"},
		{"example.com", "example.com"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractHost(tt.input)
		if got != tt.want {
			t.Errorf("extractHost(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDetectMode(t *testing.T) {
	tab := newTestRemoteTab()

	// No state → disconnected
	if tab.detectMode() != remoteModeDisconnected {
		t.Fatal("expected disconnected with no state")
	}

	// Server running → serving
	tab.serverState = &remote.ServerState{Running: true}
	if tab.detectMode() != remoteModeServing {
		t.Fatal("expected serving with serverState.Running=true")
	}

	// Connected but not serving
	tab.serverState = nil
	tab.remoteState = &remote.RemoteState{Connected: true}
	if tab.detectMode() != remoteModeConnected {
		t.Fatal("expected connected with remoteState.Connected=true")
	}

	// Both set: server takes priority
	tab.serverState = &remote.ServerState{Running: true}
	if tab.detectMode() != remoteModeServing {
		t.Fatal("expected serving when both are set (server priority)")
	}
}

func TestRemoteTabDisconnectedView(t *testing.T) {
	tab := newTestRemoteTab()

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "disconnected") {
		t.Error("disconnected view should contain 'disconnected'")
	}
	if !strings.Contains(view, "Remote") {
		t.Error("disconnected view should contain 'Remote'")
	}
}

func TestRemoteTabTokenInputMode(t *testing.T) {
	tab := newTestRemoteTab()

	// Key "c" enters token input
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if tab.mode != remoteModeTokenInput {
		t.Fatalf("expected tokenInput mode, got %d", tab.mode)
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "token") || !strings.Contains(view, "connect") {
		t.Errorf("token input view should contain 'token' and 'connect', got: %s", view)
	}
}

func TestRemoteTabTokenInputEsc(t *testing.T) {
	tab := newTestRemoteTab()
	tab.mode = remoteModeTokenInput

	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode == remoteModeTokenInput {
		t.Fatal("esc should exit token input mode")
	}
}

func TestRemoteTabConnectedView(t *testing.T) {
	tab := newTestRemoteTab()
	tab.remoteState = &remote.RemoteState{
		Connected:      true,
		ServerEndpoint: "example.com:7780",
		LocalIP:        "10.100.0.2/32",
		ServerIP:       "10.100.0.1/32",
		ConnectedAt:    time.Now(),
	}
	tab.mode = remoteModeConnected

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "connected") {
		t.Error("connected view should contain 'connected'")
	}
	if !strings.Contains(view, "example.com") {
		t.Error("connected view should contain server host")
	}
	if !strings.Contains(view, "10.100.0.2") {
		t.Error("connected view should contain local IP")
	}
}

func TestRemoteTabServingView(t *testing.T) {
	tab := newTestRemoteTab()
	tab.serverState = &remote.ServerState{
		Running:    true,
		ServerIP:   "10.100.0.1/32",
		ListenPort: 7780,
	}
	tab.mode = remoteModeServing

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "serving") {
		t.Error("serving view should contain 'serving'")
	}
	if !strings.Contains(view, "7780") {
		t.Error("serving view should contain listen port")
	}
}

func TestRemoteTabDisconnectConfirm(t *testing.T) {
	tab := newTestRemoteTab()
	tab.remoteState = &remote.RemoteState{Connected: true}
	tab.mode = remoteModeConnected

	// Key "D" sets confirmDisconnect
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("D")})
	if !tab.confirmDisconnect {
		t.Fatal("expected confirmDisconnect=true after 'D'")
	}

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "Disconnect") {
		t.Error("disconnect confirm view should contain 'Disconnect'")
	}
}

func TestRemoteTabDisconnectCancel(t *testing.T) {
	tab := newTestRemoteTab()
	tab.confirmDisconnect = true
	tab.remoteState = &remote.RemoteState{Connected: true}

	// Any key other than y/Y cancels
	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.confirmDisconnect {
		t.Fatal("expected confirmDisconnect=false after Esc")
	}
}

func TestRemoteTabSudoPromptView(t *testing.T) {
	tab := newTestRemoteTab()
	tab.mode = remoteModeSudoPrompt

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "sudo") || !strings.Contains(view, "Password") {
		t.Errorf("sudo prompt should contain 'sudo' and 'Password', got: %s", view)
	}
}

func TestRemoteTabSudoPromptEsc(t *testing.T) {
	tab := newTestRemoteTab()
	tab.mode = remoteModeSudoPrompt

	tab.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if tab.mode == remoteModeSudoPrompt {
		t.Fatal("esc should exit sudo prompt mode")
	}
	if !tab.sudoDismissed {
		t.Fatal("esc should set sudoDismissed=true")
	}
}

func TestRemoteTabApplyState(t *testing.T) {
	tab := newTestRemoteTab()

	// Error
	tab.applyState(remoteStateMsg{err: fmt.Errorf("test error")})
	if tab.err != "test error" {
		t.Fatalf("expected error 'test error', got %q", tab.err)
	}

	// Successful state
	tab.applyState(remoteStateMsg{
		remoteState: &remote.RemoteState{Connected: true},
		ifaceStatus: "up",
	})
	if tab.err != "" {
		t.Fatalf("expected no error, got %q", tab.err)
	}
	if tab.mode != remoteModeConnected {
		t.Fatalf("expected connected mode, got %d", tab.mode)
	}
}

func TestRemoteTabApplyStatePreservesInputMode(t *testing.T) {
	tab := newTestRemoteTab()
	tab.mode = remoteModeTokenInput

	tab.applyState(remoteStateMsg{
		remoteState: &remote.RemoteState{Connected: false},
	})
	// Should NOT override token input mode
	if tab.mode != remoteModeTokenInput {
		t.Fatalf("expected tokenInput mode preserved, got %d", tab.mode)
	}
}

func TestRemoteTabTitle(t *testing.T) {
	tab := newTestRemoteTab()
	if tab.Title() != "Remote" {
		t.Fatalf("expected 'Remote', got %q", tab.Title())
	}
}

func TestRemoteTabShortHelp(t *testing.T) {
	tab := newTestRemoteTab()

	// Disconnected
	tab.mode = remoteModeDisconnected
	bindings := tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty bindings for disconnected")
	}

	// Connected
	tab.mode = remoteModeConnected
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty bindings for connected")
	}

	// Serving
	tab.mode = remoteModeServing
	bindings = tab.ShortHelp()
	if len(bindings) == 0 {
		t.Fatal("expected non-empty bindings for serving")
	}
}

func TestRemoteTabTokenInputTyping(t *testing.T) {
	tab := newTestRemoteTab()
	tab.mode = remoteModeTokenInput

	// Type into token input
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "token") || !strings.Contains(view, "connect") {
		t.Error("token input should still show prompt")
	}
}

func TestRemoteTabHandleKeyConnectedMode(t *testing.T) {
	tab := newTestRemoteTab()
	tab.remoteState = &remote.RemoteState{Connected: true}
	tab.mode = remoteModeConnected

	// "R" should trigger refresh (just make sure no panic)
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
}

func TestRemoteTabHandleKeyServingMode(t *testing.T) {
	tab := newTestRemoteTab()
	tab.serverState = &remote.ServerState{Running: true}
	tab.mode = remoteModeServing

	// "R" refresh in serving mode
	tab.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
}

func TestRemoteTabUpdateTabActivateDeactivate(t *testing.T) {
	tab := newTestRemoteTab()

	// TabActivateMsg
	tab.Update(TabActivateMsg{})
	// No panic = pass

	// TabDeactivateMsg
	tab.Update(TabDeactivateMsg{})
}

func TestRemoteTabUpdateRemoteStateMsg(t *testing.T) {
	tab := newTestRemoteTab()

	// Successful state
	tab.Update(remoteStateMsg{
		remoteState: &remote.RemoteState{Connected: true},
	})
	if tab.mode != remoteModeConnected {
		t.Fatalf("expected connected, got %d", tab.mode)
	}

	// Error state
	tab.Update(remoteStateMsg{err: fmt.Errorf("network error")})
	if tab.err != "network error" {
		t.Fatalf("expected error, got %q", tab.err)
	}
}

func TestRemoteTabUpdateExecReturnMsg(t *testing.T) {
	tab := newTestRemoteTab()
	tab.confirmDisconnect = true

	// Error case
	tab.Update(remoteExecReturnMsg{err: fmt.Errorf("exec failed")})
	if tab.err != "exec failed" {
		t.Fatalf("expected error, got %q", tab.err)
	}
	if tab.confirmDisconnect {
		t.Fatal("expected confirmDisconnect=false")
	}

	// Success case
	tab.err = "old"
	tab.Update(remoteExecReturnMsg{})
	if tab.confirmDisconnect {
		t.Fatal("expected confirmDisconnect=false")
	}
}

func TestRemoteTabUpdateWindowSizeMsg(t *testing.T) {
	tab := newTestRemoteTab()
	tab.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	if tab.width != 200 || tab.height != 50 {
		t.Fatalf("expected 200x50, got %dx%d", tab.width, tab.height)
	}
}

func TestRemoteTabServingViewWithClients(t *testing.T) {
	tab := newTestRemoteTab()
	tab.serverState = &remote.ServerState{
		Running:    true,
		ServerIP:   "10.100.0.1/32",
		ListenPort: 7780,
		Clients: []remote.ClientRegistration{
			{
				Name:         "client-1",
				AssignedIP:   "10.100.0.2/32",
				Hostname:     "macbook",
				RegisteredAt: time.Now(),
			},
		},
	}
	tab.mode = remoteModeServing

	view := stripAnsi(tab.View())
	if !strings.Contains(view, "client-1") {
		t.Error("serving view should contain client name")
	}
	if !strings.Contains(view, "macbook") {
		t.Error("serving view should contain client hostname")
	}
}
