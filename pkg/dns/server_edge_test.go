package dns

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
)

func TestDefaultFallbackForUnknownSubdomain(t *testing.T) {
	port := getFreeUDPPort(t)
	s := NewServer(port)
	s.AddRecord("known-app", "10.0.0.1")

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)

	c := new(mdns.Client)
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn("unknown-app."+Domain), mdns.TypeA)

	r, _, err := c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("Exchange error: %v", err)
	}

	// Server falls back to 127.0.0.1 for unknown subdomains
	if len(r.Answer) == 0 {
		t.Fatal("expected fallback answer for unknown subdomain")
	}

	a, ok := r.Answer[0].(*mdns.A)
	if !ok {
		t.Fatalf("answer is not A record: %T", r.Answer[0])
	}
	if a.A.String() != "127.0.0.1" {
		t.Errorf("fallback IP = %q, want %q", a.A.String(), "127.0.0.1")
	}
}

func TestConcurrentAddAndLookup(t *testing.T) {
	port := getFreeUDPPort(t)
	s := NewServer(port)

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ip := fmt.Sprintf("10.0.0.%d", n+1)
			s.AddRecord("concurrent-app", ip)
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := new(mdns.Client)
			m := new(mdns.Msg)
			m.SetQuestion(mdns.Fqdn("concurrent-app."+Domain), mdns.TypeA)
			c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", port))
		}()
	}

	wg.Wait()
}

func TestRemoveRecordThenFallback(t *testing.T) {
	port := getFreeUDPPort(t)
	s := NewServer(port)
	s.AddRecord("temp-app", "10.0.0.99")

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()
	time.Sleep(50 * time.Millisecond)

	// Verify record resolves to explicit IP
	c := new(mdns.Client)
	m := new(mdns.Msg)
	m.SetQuestion(mdns.Fqdn("temp-app."+Domain), mdns.TypeA)

	r, _, err := c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("Exchange error: %v", err)
	}
	if len(r.Answer) == 0 {
		t.Fatal("expected answer for temp-app")
	}
	a := r.Answer[0].(*mdns.A)
	if a.A.String() != "10.0.0.99" {
		t.Errorf("before remove: IP = %q, want %q", a.A.String(), "10.0.0.99")
	}

	// Remove and verify fallback to 127.0.0.1
	s.RemoveRecord("temp-app")

	m2 := new(mdns.Msg)
	m2.SetQuestion(mdns.Fqdn("temp-app."+Domain), mdns.TypeA)
	r2, _, err := c.Exchange(m2, fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("Exchange error after remove: %v", err)
	}
	if len(r2.Answer) == 0 {
		t.Fatal("expected fallback answer after remove")
	}
	a2 := r2.Answer[0].(*mdns.A)
	if a2.A.String() != "127.0.0.1" {
		t.Errorf("after remove: IP = %q, want %q (fallback)", a2.A.String(), "127.0.0.1")
	}
}

func TestRemoveNonexistentRecord(t *testing.T) {
	s := NewServer(0)
	// Should not panic
	s.RemoveRecord("nonexistent")
}

func TestMultipleRecordsIndependent(t *testing.T) {
	s := NewServer(0)
	s.AddRecord("app-a", "10.0.0.1")
	s.AddRecord("app-b", "10.0.0.2")

	s.mu.RLock()
	count := len(s.records)
	s.mu.RUnlock()
	if count != 2 {
		t.Errorf("expected 2 records, got %d", count)
	}

	s.RemoveRecord("app-a")

	s.mu.RLock()
	count = len(s.records)
	s.mu.RUnlock()
	if count != 1 {
		t.Errorf("expected 1 record after remove, got %d", count)
	}
}

func TestToFQDN(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"myapp", "myapp." + Domain + "."},
		{"myapp.", "myapp." + Domain + "."},
		{"MYAPP", "myapp." + Domain + "."},
		{"sub.myapp", "sub.myapp." + Domain + "."},
		{"myapp." + Domain, "myapp." + Domain + "."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toFQDN(tt.input)
			if got != tt.want {
				t.Errorf("toFQDN(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// getFreeUDPPort finds a free UDP port for testing.
func getFreeUDPPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return port
}
