package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"vibespace/pkg/dns"
)

const (
	defaultPort       = 5353
	defaultDomain     = "vibe.space"
	defaultTargetIP   = "127.0.0.1"
	defaultTargetIPv6 = "::1"
)

func main() {
	// Configure logger
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.SetPrefix("[dnsd] ")

	// Parse configuration from environment variables
	config := parseConfig()

	// Validate configuration
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create DNS server
	server, err := dns.NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create DNS server: %v", err)
	}

	// Setup context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		log.Printf("Starting DNS server...")
		log.Printf("  Port:       %d", config.Port)
		log.Printf("  Domain:     *.%s", config.Domain)
		log.Printf("  Target IP:  %s (IPv4)", config.TargetIP)
		log.Printf("  Target IP:  %s (IPv6)", config.TargetIPv6)

		if err := server.Start(ctx); err != nil {
			errCh <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Wait for shutdown signal or error
	select {
	case sig := <-sigCh:
		log.Printf("Received signal: %v", sig)
		log.Printf("Initiating graceful shutdown...")
		cancel()

		// Stop server
		if err := server.Stop(); err != nil {
			log.Printf("Error during shutdown: %v", err)
			os.Exit(1)
		}

		log.Printf("DNS server stopped gracefully")
		os.Exit(0)

	case err := <-errCh:
		log.Printf("Server error: %v", err)
		os.Exit(1)
	}
}

// parseConfig reads configuration from environment variables
func parseConfig() *dns.Config {
	config := dns.DefaultConfig()

	// DNS_PORT: Override default port
	if portStr := os.Getenv("DNS_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			config.Port = port
		} else {
			log.Printf("Warning: Invalid DNS_PORT value '%s', using default %d", portStr, defaultPort)
		}
	}

	// DNS_DOMAIN: Override default domain
	if domain := os.Getenv("DNS_DOMAIN"); domain != "" {
		config.Domain = domain
	}

	// DNS_TARGET_IP: Override default IPv4 target
	if targetIP := os.Getenv("DNS_TARGET_IP"); targetIP != "" {
		config.TargetIP = targetIP
	}

	// DNS_TARGET_IPV6: Override default IPv6 target
	if targetIPv6 := os.Getenv("DNS_TARGET_IPV6"); targetIPv6 != "" {
		config.TargetIPv6 = targetIPv6
	}

	return config
}
