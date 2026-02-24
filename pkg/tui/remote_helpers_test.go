package tui

// Tests for extractHost and stripCIDR are in tab_remote_test.go.
// This file adds additional edge case tests.

import "testing"

func TestExtractHostIPv6(t *testing.T) {
	// IPv6 with port
	got := extractHost("[::1]:8080")
	if got != "[::1]" {
		t.Errorf("extractHost IPv6 = %q, want %q", got, "[::1]")
	}
}

func TestStripCIDRNoSlash(t *testing.T) {
	got := stripCIDR("10.100.0.2")
	if got != "10.100.0.2" {
		t.Errorf("stripCIDR without CIDR = %q, want %q", got, "10.100.0.2")
	}
}
