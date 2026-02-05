// Package dns provides an embedded DNS server for resolving *.vibespace.internal
// to localhost, enabling friendly hostnames for port-forwarded services.
package dns

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// Domain is the TLD used for vibespace DNS resolution.
// We use .internal instead of .local to avoid conflicts with mDNS/Bonjour on macOS.
const Domain = "vibespace.internal"

// Server is an embedded DNS server that resolves *.vibespace.internal records.
type Server struct {
	port    int
	records map[string]string // FQDN -> IP
	mu      sync.RWMutex
	server  *dns.Server
}

// NewServer creates a new DNS server on the given port.
func NewServer(port int) *Server {
	return &Server{
		port:    port,
		records: make(map[string]string),
	}
}

// AddRecord adds a DNS A record mapping name.vibespace.internal -> ip.
// The name should not include the domain suffix.
func (s *Server) AddRecord(name, ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fqdn := toFQDN(name)
	s.records[fqdn] = ip
	slog.Debug("DNS record added", "name", fqdn, "ip", ip)
}

// RemoveRecord removes a DNS record by name.
func (s *Server) RemoveRecord(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fqdn := toFQDN(name)
	delete(s.records, fqdn)
	slog.Debug("DNS record removed", "name", fqdn)
}

// Start starts the DNS server.
func (s *Server) Start() error {
	mux := dns.NewServeMux()
	mux.HandleFunc(Domain+".", s.handleQuery)

	s.server = &dns.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", s.port),
		Net:     "udp",
		Handler: mux,
	}

	slog.Info("starting DNS server", "addr", s.server.Addr, "domain", Domain)

	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			slog.Error("DNS server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the DNS server.
func (s *Server) Stop() {
	if s.server != nil {
		s.server.Shutdown()
	}
}

func (s *Server) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	for _, q := range r.Question {
		if q.Qtype != dns.TypeA {
			continue
		}

		name := strings.ToLower(q.Name)
		s.mu.RLock()
		ip, ok := s.records[name]
		s.mu.RUnlock()

		if !ok {
			// Default: resolve anything under vibespace.internal to 127.0.0.1
			ip = "127.0.0.1"
		}

		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.To4() == nil {
			continue
		}

		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   q.Name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    30,
			},
			A: parsed.To4(),
		}
		m.Answer = append(m.Answer, rr)
	}

	w.WriteMsg(m)
}

// toFQDN converts a short name to a fully qualified domain name under vibespace.internal.
func toFQDN(name string) string {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if strings.HasSuffix(name, "."+Domain) {
		return name + "."
	}
	return name + "." + Domain + "."
}
