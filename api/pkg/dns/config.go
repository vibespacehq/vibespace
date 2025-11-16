package dns

import "time"

// Config holds configuration for the DNS server
type Config struct {
	// Port is the UDP/TCP port to listen on (default: 53535 to avoid mDNS conflict on 5353)
	Port int

	// Domain is the root domain to handle (e.g., "vibe.space")
	Domain string

	// TargetIP is the IP address to return for all queries (e.g., "127.0.0.1")
	TargetIP string

	// TargetIPv6 is the IPv6 address to return for AAAA queries (e.g., "::1")
	TargetIPv6 string

	// ReadTimeout is the maximum duration for reading the entire request
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out writes
	WriteTimeout time.Duration
}

// DefaultConfig returns a Config with sensible defaults for local development
func DefaultConfig() *Config {
	return &Config{
		Port:         53535,           // Unprivileged port (avoids mDNS conflict on port 5353)
		Domain:       "vibe.space",    // vibespace domain
		TargetIP:     "127.0.0.1",     // Localhost IPv4
		TargetIPv6:   "::1",           // Localhost IPv6
		ReadTimeout:  3 * time.Second, // Standard DNS timeout
		WriteTimeout: 3 * time.Second,
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Port <= 0 || c.Port > 65535 {
		return ErrInvalidPort
	}

	if c.Domain == "" {
		return ErrEmptyDomain
	}

	if c.TargetIP == "" {
		return ErrEmptyTargetIP
	}

	return nil
}
