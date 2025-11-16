package dns

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Port != 53535 {
		t.Errorf("Expected port 53535, got %d", config.Port)
	}

	if config.Domain != "vibe.space" {
		t.Errorf("Expected domain 'vibe.space', got %s", config.Domain)
	}

	if config.TargetIP != "127.0.0.1" {
		t.Errorf("Expected target IP '127.0.0.1', got %s", config.TargetIP)
	}

	if config.TargetIPv6 != "::1" {
		t.Errorf("Expected target IPv6 '::1', got %s", config.TargetIPv6)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr error
	}{
		{
			name:    "valid config",
			config:  DefaultConfig(),
			wantErr: nil,
		},
		{
			name: "invalid port (too low)",
			config: &Config{
				Port:       0,
				Domain:     "vibe.space",
				TargetIP:   "127.0.0.1",
				TargetIPv6: "::1",
			},
			wantErr: ErrInvalidPort,
		},
		{
			name: "invalid port (too high)",
			config: &Config{
				Port:       70000,
				Domain:     "vibe.space",
				TargetIP:   "127.0.0.1",
				TargetIPv6: "::1",
			},
			wantErr: ErrInvalidPort,
		},
		{
			name: "empty domain",
			config: &Config{
				Port:       53535,
				Domain:     "",
				TargetIP:   "127.0.0.1",
				TargetIPv6: "::1",
			},
			wantErr: ErrEmptyDomain,
		},
		{
			name: "empty target IP",
			config: &Config{
				Port:       53535,
				Domain:     "vibe.space",
				TargetIP:   "",
				TargetIPv6: "::1",
			},
			wantErr: ErrEmptyTargetIP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != tt.wantErr {
				t.Errorf("Expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewServer(t *testing.T) {
	config := DefaultConfig()
	server, err := NewServer(config)

	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if server == nil {
		t.Fatal("Server is nil")
	}

	if server.IsRunning() {
		t.Error("Server should not be running immediately after creation")
	}
}

func TestServerStartStop(t *testing.T) {
	config := DefaultConfig()
	config.Port = 15353 // Use different port for testing

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil && err != context.Canceled {
			t.Errorf("Server failed to start: %v", err)
		}
	}()

	// Give server time to start
	time.Sleep(200 * time.Millisecond)

	if !server.IsRunning() {
		t.Error("Server should be running after Start()")
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	if server.IsRunning() {
		t.Error("Server should not be running after Stop()")
	}

	// Double stop should return error
	if err := server.Stop(); err != ErrServerNotStarted {
		t.Errorf("Expected ErrServerNotStarted, got %v", err)
	}
}

func TestDNSWildcardResolution(t *testing.T) {
	config := DefaultConfig()
	config.Port = 15354 // Use different port

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)
	defer server.Stop()

	// Test cases
	tests := []struct {
		name       string
		query      string
		qtype      uint16
		wantAnswer bool
		wantIP     string
	}{
		{
			name:       "exact domain match",
			query:      "vibe.space",
			qtype:      dns.TypeA,
			wantAnswer: true,
			wantIP:     "127.0.0.1",
		},
		{
			name:       "single subdomain",
			query:      "my-app.vibe.space",
			qtype:      dns.TypeA,
			wantAnswer: true,
			wantIP:     "127.0.0.1",
		},
		{
			name:       "double subdomain",
			query:      "code.my-app.vibe.space",
			qtype:      dns.TypeA,
			wantAnswer: true,
			wantIP:     "127.0.0.1",
		},
		{
			name:       "triple subdomain",
			query:      "preview.my-app.vibe.space",
			qtype:      dns.TypeA,
			wantAnswer: true,
			wantIP:     "127.0.0.1",
		},
		{
			name:       "IPv6 query",
			query:      "code.my-app.vibe.space",
			qtype:      dns.TypeAAAA,
			wantAnswer: true,
			wantIP:     "::1",
		},
		{
			name:       "non-matching domain",
			query:      "example.com",
			qtype:      dns.TypeA,
			wantAnswer: false,
			wantIP:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := new(dns.Client)
			m := new(dns.Msg)
			m.SetQuestion(dns.Fqdn(tt.query), tt.qtype)

			addr := "127.0.0.1:15354"
			r, _, err := c.Exchange(m, addr)
			if err != nil {
				t.Fatalf("DNS query failed: %v", err)
			}

			if tt.wantAnswer {
				if len(r.Answer) == 0 {
					t.Errorf("Expected DNS answer for %s, got none", tt.query)
					return
				}

				switch tt.qtype {
				case dns.TypeA:
					if a, ok := r.Answer[0].(*dns.A); ok {
						if a.A.String() != tt.wantIP {
							t.Errorf("Expected IP %s, got %s", tt.wantIP, a.A.String())
						}
					} else {
						t.Error("Expected A record in answer")
					}

				case dns.TypeAAAA:
					if aaaa, ok := r.Answer[0].(*dns.AAAA); ok {
						if aaaa.AAAA.String() != tt.wantIP {
							t.Errorf("Expected IPv6 %s, got %s", tt.wantIP, aaaa.AAAA.String())
						}
					} else {
						t.Error("Expected AAAA record in answer")
					}
				}
			} else {
				if len(r.Answer) > 0 {
					t.Errorf("Expected no answer for %s, got %d answers", tt.query, len(r.Answer))
				}

				if r.Rcode != dns.RcodeNameError {
					t.Errorf("Expected NXDOMAIN (rcode %d), got rcode %d", dns.RcodeNameError, r.Rcode)
				}
			}
		})
	}
}

func TestHealthCheck(t *testing.T) {
	config := DefaultConfig()
	config.Port = 15355

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Health check before server starts
	if server.IsHealthy() {
		t.Error("Server should not be healthy before starting")
	}

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		server.Start(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	defer server.Stop()

	// Health check after server starts
	if !server.IsHealthy() {
		t.Error("Server should be healthy after starting")
	}

	// Stop server
	server.Stop()

	// Health check after server stops
	if server.IsHealthy() {
		t.Error("Server should not be healthy after stopping")
	}
}

func TestMatchesDomain(t *testing.T) {
	config := DefaultConfig()
	server, _ := NewServer(config)

	tests := []struct {
		queryName string
		want      bool
	}{
		{"vibe.space.", true},
		{"my-app.vibe.space.", true},
		{"code.my-app.vibe.space.", true},
		{"preview.my-app.vibe.space.", true},
		{"deeply.nested.subdomain.vibe.space.", true},
		{"example.com.", false},
		{"vibespace.com.", false},
		{"vibe.space.example.com.", false},
		{"notavibe.space.", false},
	}

	for _, tt := range tests {
		t.Run(tt.queryName, func(t *testing.T) {
			got := server.matchesDomain(tt.queryName)
			if got != tt.want {
				t.Errorf("matchesDomain(%s) = %v, want %v", tt.queryName, got, tt.want)
			}
		})
	}
}

func TestDoubleStart(t *testing.T) {
	config := DefaultConfig()
	config.Port = 15356

	server, err := NewServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start server
	go func() {
		server.Start(ctx)
	}()

	time.Sleep(200 * time.Millisecond)
	defer server.Stop()

	// Try to start again
	err = server.Start(ctx)
	if err != ErrServerAlreadyRunning {
		t.Errorf("Expected ErrServerAlreadyRunning, got %v", err)
	}
}
