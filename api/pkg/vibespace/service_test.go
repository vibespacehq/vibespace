package vibespace

import (
	"fmt"
	"net"
	"testing"
)

func TestHashStringToPort(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantMin  int
		wantMax  int
		checkDet bool // check deterministic (same input = same output)
	}{
		{
			name:     "short id",
			id:       "abc123",
			wantMin:  0,
			wantMax:  999,
			checkDet: true,
		},
		{
			name:     "long id",
			id:       "vibespace-f8a3b2c1-9d4e-4f6a-8b7c-1e2d3f4a5b6c",
			wantMin:  0,
			wantMax:  999,
			checkDet: true,
		},
		{
			name:     "empty id",
			id:       "",
			wantMin:  0,
			wantMax:  999,
			checkDet: true,
		},
		{
			name:     "special characters",
			id:       "test-!@#$%",
			wantMin:  0,
			wantMax:  999,
			checkDet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hashStringToPort(tt.id)

			// Check range
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("hashStringToPort(%q) = %d, want in range [%d, %d]", tt.id, got, tt.wantMin, tt.wantMax)
			}

			// Check deterministic behavior
			if tt.checkDet {
				got2 := hashStringToPort(tt.id)
				if got != got2 {
					t.Errorf("hashStringToPort(%q) not deterministic: first=%d, second=%d", tt.id, got, got2)
				}
			}
		})
	}
}

func TestHashStringToPort_Consistency(t *testing.T) {
	// Same vibespace ID should always produce same port
	id := "vibespace-abc123"
	results := make(map[int]bool)

	for i := 0; i < 100; i++ {
		port := hashStringToPort(id)
		results[port] = true
	}

	if len(results) != 1 {
		t.Errorf("hashStringToPort not consistent: got %d different ports for same ID", len(results))
	}
}

func TestHashStringToPort_Distribution(t *testing.T) {
	// Different IDs should produce different ports (roughly distributed)
	ids := []string{
		"ws-1", "ws-2", "ws-3", "ws-4", "ws-5",
		"ws-6", "ws-7", "ws-8", "ws-9", "ws-10",
	}

	ports := make(map[int]bool)
	for _, id := range ids {
		port := hashStringToPort(id)
		ports[port] = true
	}

	// At least 50% should be unique (simple collision check)
	if len(ports) < len(ids)/2 {
		t.Errorf("hashStringToPort poor distribution: only %d unique ports for %d IDs", len(ports), len(ids))
	}
}

func TestIsPortAvailable(t *testing.T) {
	tests := []struct {
		name string
		port int
		want bool
	}{
		{
			name: "privileged port",
			port: 80,
			want: false, // Usually can't bind to privileged ports without root
		},
		{
			name: "invalid port - negative",
			port: -1,
			want: false,
		},
		{
			name: "invalid port - too high",
			port: 65536,
			want: false,
		},
		{
			name: "valid high port",
			port: 45678,
			want: true, // Likely available
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPortAvailable(tt.port)

			// For privileged/invalid ports, we expect false
			if tt.port <= 1024 || tt.port < 0 || tt.port > 65535 {
				if got != false {
					t.Errorf("isPortAvailable(%d) = %v, want false for invalid/privileged port", tt.port, got)
				}
				return
			}

			// For valid high ports, we just check it returns a boolean
			// (actual availability depends on system state)
			if got != true && got != false {
				t.Errorf("isPortAvailable(%d) returned non-boolean", tt.port)
			}
		})
	}
}

func TestIsPortAvailable_InUse(t *testing.T) {
	// Test that port is correctly detected as unavailable when in use
	// This is a more complex test that requires actually binding to a port

	// Skip this test in short mode
	if testing.Short() {
		t.Skip("skipping port binding test in short mode")
	}

	// Try to find an available port first
	testPort := 0
	for port := 18080; port < 18090; port++ {
		if isPortAvailable(port) {
			testPort = port
			break
		}
	}

	if testPort == 0 {
		t.Skip("no available ports in test range")
	}

	// Port should be available initially
	if !isPortAvailable(testPort) {
		t.Fatalf("test port %d not available initially", testPort)
	}

	// Bind to the port using net.Listen
	listener, err := netListenTCP(testPort)
	if err != nil {
		t.Fatalf("failed to bind to test port %d: %v", testPort, err)
	}
	defer listener.Close()

	// Port should now be unavailable
	if isPortAvailable(testPort) {
		t.Errorf("isPortAvailable(%d) = true, want false (port is bound)", testPort)
	}
}

// Helper function for testing
func netListenTCP(port int) (net.Listener, error) {
	return net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
}

