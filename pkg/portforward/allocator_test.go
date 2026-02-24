package portforward

import "testing"

func TestAllocatePort(t *testing.T) {
	a := NewPortAllocator("test-vibespace")

	port, err := a.AllocatePort("claude-1", 8080)
	if err != nil {
		t.Fatalf("AllocatePort: %v", err)
	}
	if port == 0 {
		t.Error("expected non-zero port")
	}
}

func TestAllocatePortIdempotent(t *testing.T) {
	a := NewPortAllocator("test-vibespace")

	port1, err := a.AllocatePort("claude-1", 8080)
	if err != nil {
		t.Fatalf("first AllocatePort: %v", err)
	}

	port2, err := a.AllocatePort("claude-1", 8080)
	if err != nil {
		t.Fatalf("second AllocatePort: %v", err)
	}

	if port1 != port2 {
		t.Errorf("idempotent allocation returned different ports: %d vs %d", port1, port2)
	}
}

func TestReleasePort(t *testing.T) {
	a := NewPortAllocator("test-vibespace")

	_, err := a.AllocatePort("claude-1", 8080)
	if err != nil {
		t.Fatalf("AllocatePort: %v", err)
	}

	a.ReleasePort("claude-1", 8080)

	// After release, a new allocation should still work
	_, err = a.AllocatePort("claude-1", 8080)
	if err != nil {
		t.Fatalf("AllocatePort after release: %v", err)
	}
}

func TestReleaseAllForAgent(t *testing.T) {
	a := NewPortAllocator("test-vibespace")

	_, err := a.AllocatePort("claude-1", 8080)
	if err != nil {
		t.Fatalf("AllocatePort 8080: %v", err)
	}
	_, err = a.AllocatePort("claude-1", 3000)
	if err != nil {
		t.Fatalf("AllocatePort 3000: %v", err)
	}
	_, err = a.AllocatePort("claude-2", 8080)
	if err != nil {
		t.Fatalf("AllocatePort claude-2: %v", err)
	}

	a.ReleaseAllForAgent("claude-1")

	// claude-1 ports should be released, claude-2 should be unaffected
	// Re-allocate claude-1 ports to verify they were freed
	_, err = a.AllocatePort("claude-1", 8080)
	if err != nil {
		t.Fatalf("re-AllocatePort after release: %v", err)
	}
}

func TestAllocateDifferentAgents(t *testing.T) {
	a := NewPortAllocator("test-vibespace")

	port1, err := a.AllocatePort("claude-1", 8080)
	if err != nil {
		t.Fatalf("AllocatePort claude-1: %v", err)
	}

	port2, err := a.AllocatePort("claude-2", 8080)
	if err != nil {
		t.Fatalf("AllocatePort claude-2: %v", err)
	}

	if port1 == port2 {
		t.Errorf("different agents got same port: %d", port1)
	}
}
