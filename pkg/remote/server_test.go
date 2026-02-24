package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServerWGIP(t *testing.T) {
	tests := []struct {
		name     string
		serverIP string
		want     string
	}{
		{"with CIDR", "10.100.0.1/24", "10.100.0.1"},
		{"plain IP", "10.100.0.1", "10.100.0.1"},
		{"empty uses default", "", DefaultServerIP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := serverWGIP(tt.serverIP)
			if got != tt.want {
				t.Errorf("serverWGIP(%q) = %q, want %q", tt.serverIP, got, tt.want)
			}
		})
	}
}

func TestCertSHA256(t *testing.T) {
	input := []byte("test certificate data")
	got := certSHA256(input)

	if !strings.HasPrefix(got, "sha256:") {
		t.Errorf("certSHA256() = %q, want sha256: prefix", got)
	}

	// Verify hash is correct
	hash := sha256.Sum256(input)
	expected := "sha256:" + hex.EncodeToString(hash[:])
	if got != expected {
		t.Errorf("certSHA256() = %q, want %q", got, expected)
	}
}

func TestCertSHA256Deterministic(t *testing.T) {
	input := []byte("deterministic test")
	got1 := certSHA256(input)
	got2 := certSHA256(input)
	if got1 != got2 {
		t.Errorf("certSHA256 not deterministic: %q vs %q", got1, got2)
	}
}

func TestNewTokenNonce(t *testing.T) {
	nonce := newTokenNonce()
	if nonce == "" {
		t.Error("newTokenNonce() returned empty string")
	}
	// Base64url encoded 16 bytes = 22 chars (no padding)
	if len(nonce) < 10 {
		t.Errorf("nonce too short: %q", nonce)
	}
}

func TestNewTokenNonceUniqueness(t *testing.T) {
	n1 := newTokenNonce()
	n2 := newTokenNonce()
	if n1 == n2 {
		t.Errorf("two nonces should be unique: %q", n1)
	}
}

func TestSecurityHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := securityHeaders(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "default-src 'none'",
		"Cache-Control":           "no-store",
	}
	for header, want := range expectedHeaders {
		got := w.Header().Get(header)
		if got != want {
			t.Errorf("header %q = %q, want %q", header, got, want)
		}
	}
}

func TestCheckPortBindable(t *testing.T) {
	// Test with an ephemeral port that should be available
	result := checkPortBindable("tcp", 0, "test-port")
	// Port 0 lets the OS assign, but the function uses a fixed format "0.0.0.0:0"
	// which will succeed since port 0 means "any available"
	if !result.Status {
		t.Errorf("expected available port to be bindable, got: %s", result.Message)
	}
	if result.Check == "" {
		t.Error("Check field should be populated")
	}
}
