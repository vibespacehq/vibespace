package dns

import (
	"fmt"
	"net"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
)

func TestAddRemoveRecord(t *testing.T) {
	s := NewServer(0)

	s.AddRecord("myapp", "192.168.1.100")

	fqdn := "myapp." + Domain() + "."
	s.mu.RLock()
	ip, ok := s.records[fqdn]
	s.mu.RUnlock()
	if !ok {
		t.Fatalf("record for %q not found", fqdn)
	}
	if ip != "192.168.1.100" {
		t.Errorf("record IP = %q, want %q", ip, "192.168.1.100")
	}

	s.RemoveRecord("myapp")

	s.mu.RLock()
	_, ok = s.records[fqdn]
	s.mu.RUnlock()
	if ok {
		t.Error("record should be removed")
	}
}

func TestDNSServerStartStop(t *testing.T) {
	// Find a free UDP port
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	s := NewServer(port)
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	s.Stop()
}

func TestDNSResolution(t *testing.T) {
	// Find a free UDP port
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	s := NewServer(port)
	s.AddRecord("test-app", "10.0.0.42")

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	// Send a real DNS query
	c := new(mdns.Client)
	m := new(mdns.Msg)
	m.SetQuestion("test-app."+Domain()+".", mdns.TypeA)

	r, _, err := c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("DNS query error: %v", err)
	}

	if len(r.Answer) == 0 {
		t.Fatal("DNS query returned no answers")
	}

	a, ok := r.Answer[0].(*mdns.A)
	if !ok {
		t.Fatalf("answer is not A record: %T", r.Answer[0])
	}

	if a.A.String() != "10.0.0.42" {
		t.Errorf("DNS answer = %q, want %q", a.A.String(), "10.0.0.42")
	}
}

func TestDefaultFallback(t *testing.T) {
	// Find a free UDP port
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()

	s := NewServer(port)
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer s.Stop()

	time.Sleep(50 * time.Millisecond)

	// Query an unregistered name — should return NXDOMAIN
	c := new(mdns.Client)
	m := new(mdns.Msg)
	m.SetQuestion("unknown-service."+Domain()+".", mdns.TypeA)

	r, _, err := c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("DNS query error: %v", err)
	}

	if r.Rcode != mdns.RcodeNameError {
		t.Errorf("expected NXDOMAIN (rcode %d), got rcode %d", mdns.RcodeNameError, r.Rcode)
	}

	if len(r.Answer) != 0 {
		t.Errorf("expected no answers for unregistered name, got %d", len(r.Answer))
	}
}
