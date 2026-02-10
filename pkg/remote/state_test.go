package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAllocateClientIP(t *testing.T) {
	state := &ServerState{
		NextClientIP: DefaultClientIPStart,
	}

	ip1 := state.AllocateClientIP()
	if ip1 != "10.100.0.2/32" {
		t.Errorf("first allocation = %q, want %q", ip1, "10.100.0.2/32")
	}

	ip2 := state.AllocateClientIP()
	if ip2 != "10.100.0.3/32" {
		t.Errorf("second allocation = %q, want %q", ip2, "10.100.0.3/32")
	}

	ip3 := state.AllocateClientIP()
	if ip3 != "10.100.0.4/32" {
		t.Errorf("third allocation = %q, want %q", ip3, "10.100.0.4/32")
	}
}

func TestAddClient(t *testing.T) {
	state := &ServerState{}

	state.AddClient("alice", "pubkey-alice", "10.100.0.2/32", "alice-laptop")

	if len(state.Clients) != 1 {
		t.Fatalf("Clients length = %d, want 1", len(state.Clients))
	}

	c := state.Clients[0]
	if c.Name != "alice" {
		t.Errorf("Name = %q, want %q", c.Name, "alice")
	}
	if c.PublicKey != "pubkey-alice" {
		t.Errorf("PublicKey = %q, want %q", c.PublicKey, "pubkey-alice")
	}
	if c.AssignedIP != "10.100.0.2/32" {
		t.Errorf("AssignedIP = %q, want %q", c.AssignedIP, "10.100.0.2/32")
	}
	if c.Hostname != "alice-laptop" {
		t.Errorf("Hostname = %q, want %q", c.Hostname, "alice-laptop")
	}
	if c.RegisteredAt.IsZero() {
		t.Error("RegisteredAt should not be zero")
	}
}

func TestFindClientByPublicKey(t *testing.T) {
	state := &ServerState{}
	state.AddClient("alice", "key-alice", "10.100.0.2/32", "")
	state.AddClient("bob", "key-bob", "10.100.0.3/32", "")

	found := state.FindClientByPublicKey("key-alice")
	if found == nil {
		t.Fatal("FindClientByPublicKey(key-alice) returned nil")
	}
	if found.Name != "alice" {
		t.Errorf("found.Name = %q, want %q", found.Name, "alice")
	}

	notFound := state.FindClientByPublicKey("key-unknown")
	if notFound != nil {
		t.Error("FindClientByPublicKey(unknown) should return nil")
	}
}

func TestServerStateSaveLoad(t *testing.T) {
	dir := t.TempDir()

	original := &ServerState{
		Running:      true,
		ListenPort:   51820,
		ServerIP:     "10.100.0.1/24",
		PublicKey:     "test-pub-key",
		NextClientIP: 5,
		Clients: []ClientRegistration{
			{Name: "client1", PublicKey: "key1", AssignedIP: "10.100.0.2/32"},
		},
	}

	// Save via direct JSON write
	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	// Load back
	loaded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}

	var restored ServerState
	if err := json.Unmarshal(loaded, &restored); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if restored.Running != original.Running {
		t.Errorf("Running = %v, want %v", restored.Running, original.Running)
	}
	if restored.ListenPort != original.ListenPort {
		t.Errorf("ListenPort = %d, want %d", restored.ListenPort, original.ListenPort)
	}
	if restored.ServerIP != original.ServerIP {
		t.Errorf("ServerIP = %q, want %q", restored.ServerIP, original.ServerIP)
	}
	if restored.NextClientIP != original.NextClientIP {
		t.Errorf("NextClientIP = %d, want %d", restored.NextClientIP, original.NextClientIP)
	}
	if len(restored.Clients) != 1 {
		t.Fatalf("Clients length = %d, want 1", len(restored.Clients))
	}
	if restored.Clients[0].Name != "client1" {
		t.Errorf("Client Name = %q, want %q", restored.Clients[0].Name, "client1")
	}
}
