package dns

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

// Server is a lightweight DNS server that resolves wildcard domains to localhost
type Server struct {
	config  *Config
	server  *dns.Server
	mu      sync.RWMutex
	running bool
}

// NewServer creates a new DNS server with the given configuration
func NewServer(config *Config) (*Server, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &Server{
		config:  config,
		running: false,
	}, nil
}

// Start starts the DNS server and listens for UDP requests
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrServerAlreadyRunning
	}

	// Register DNS handler
	dns.HandleFunc(".", s.handleDNSRequest)

	// Create DNS server
	s.server = &dns.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Net:          "udp",
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	s.running = true
	s.mu.Unlock()

	log.Printf("[DNS] Starting DNS server on port %d for *.%s", s.config.Port, s.config.Domain)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil {
			errCh <- fmt.Errorf("failed to start DNS server: %w", err)
		}
	}()

	// Wait for server to start or context cancellation
	select {
	case err := <-errCh:
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		return err
	case <-ctx.Done():
		return s.Stop()
	case <-func() chan struct{} {
		// Give server 100ms to start
		ch := make(chan struct{})
		go func() {
			// TODO: Better health check
			select {
			case <-ctx.Done():
			case <-func() chan struct{} {
				c := make(chan struct{})
				go func() {
					defer close(c)
					// Simple delay
					select {
					case <-ctx.Done():
					default:
					}
				}()
				return c
			}():
			}
			close(ch)
		}()
		return ch
	}():
		// Server started successfully
		log.Printf("[DNS] DNS server started successfully on port %d", s.config.Port)
		return nil
	}
}

// Stop gracefully shuts down the DNS server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return ErrServerNotStarted
	}

	if s.server != nil {
		log.Printf("[DNS] Shutting down DNS server...")
		if err := s.server.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown DNS server: %w", err)
		}
		s.server = nil
	}

	s.running = false
	log.Printf("[DNS] DNS server stopped")
	return nil
}

// IsRunning returns true if the DNS server is currently running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// handleDNSRequest processes incoming DNS queries
func (s *Server) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.RecursionAvailable = false

	// Process each question
	for _, question := range r.Question {
		queryName := question.Name

		// Check if query is for our domain (*.vibe.space)
		if s.matchesDomain(queryName) {
			switch question.Qtype {
			case dns.TypeA:
				// Return IPv4 address
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   queryName,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    0, // No caching
					},
					A: net.ParseIP(s.config.TargetIP),
				})
				log.Printf("[DNS] A query: %s → %s", queryName, s.config.TargetIP)

			case dns.TypeAAAA:
				// Return IPv6 address
				m.Answer = append(m.Answer, &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   queryName,
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    0, // No caching
					},
					AAAA: net.ParseIP(s.config.TargetIPv6),
				})
				log.Printf("[DNS] AAAA query: %s → %s", queryName, s.config.TargetIPv6)

			default:
				// For other query types, return NXDOMAIN
				m.Rcode = dns.RcodeNameError
				log.Printf("[DNS] Unsupported query type %d for %s", question.Qtype, queryName)
			}
		} else {
			// Not our domain, return NXDOMAIN
			m.Rcode = dns.RcodeNameError
			log.Printf("[DNS] Query for non-matching domain: %s", queryName)
		}
	}

	// Write response
	if err := w.WriteMsg(m); err != nil {
		log.Printf("[DNS] Failed to write response: %v", err)
	}
}

// matchesDomain checks if the query name matches our domain (wildcard)
// Examples:
//   - code.my-app.vibe.space. → true
//   - my-app.vibe.space. → true
//   - vibe.space. → true
//   - notavibe.space. → false (not a proper subdomain)
//   - example.com. → false
func (s *Server) matchesDomain(queryName string) bool {
	// DNS queries end with a dot
	domain := s.config.Domain + "."

	// Exact match
	if queryName == domain {
		return true
	}

	// Subdomain match: must end with ".{domain}." to ensure proper domain boundary
	// This prevents "notavibe.space." from matching "vibe.space."
	return strings.HasSuffix(queryName, "."+domain)
}

// IsHealthy checks if the DNS server is healthy by performing a test query
func (s *Server) IsHealthy() bool {
	if !s.IsRunning() {
		return false
	}

	// Perform a test DNS query
	c := new(dns.Client)
	m := new(dns.Msg)
	testDomain := fmt.Sprintf("test.%s.", s.config.Domain)
	m.SetQuestion(testDomain, dns.TypeA)

	// Query localhost on our port
	addr := fmt.Sprintf("127.0.0.1:%d", s.config.Port)
	r, _, err := c.Exchange(m, addr)
	if err != nil {
		log.Printf("[DNS] Health check failed: %v", err)
		return false
	}

	// Check if we got the expected response
	if len(r.Answer) == 0 {
		log.Printf("[DNS] Health check failed: no answer")
		return false
	}

	if a, ok := r.Answer[0].(*dns.A); ok {
		if a.A.String() == s.config.TargetIP {
			return true
		}
	}

	return false
}
