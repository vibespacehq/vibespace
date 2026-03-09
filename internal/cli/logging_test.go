package cli

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestRedactingHandler(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := &redactingHandler{inner: inner}
	logger := slog.New(handler)

	tests := []struct {
		name      string
		key       string
		value     string
		redacted  bool
	}{
		{"public key", "publicKey", "abc123456789", true},
		{"private key", "PrivateKeyPath", "/path/to/key", true},
		{"token", "access_token", "ghp_xxxxxxxxxxxx", true},
		{"fingerprint", "fingerprint", "sha256:abcdef1234567890", true},
		{"nonce", "nonce", "randomnonce123", true},
		{"password", "password", "hunter2", true},
		{"secret", "client_secret", "s3cr3t", true},
		{"credential", "credential_file", "/home/.git-credentials", true},
		{"sha256", "sha256", "deadbeef12345678", true},
		{"safe field", "vibespace", "myproject", false},
		{"safe addr", "addr", "10.100.0.1:7780", false},
		{"safe status", "status", "ok", false},
		{"safe error", "error", "connection refused", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			logger.Info("test", tt.key, tt.value)

			output := buf.String()
			if tt.redacted {
				if !bytes.Contains(buf.Bytes(), []byte("[REDACTED]")) {
					t.Errorf("expected %q value to be redacted, got: %s", tt.key, output)
				}
				// Full original value must not appear
				if bytes.Contains(buf.Bytes(), []byte(tt.value)) {
					t.Errorf("expected full value %q to be absent for key %q, got: %s", tt.value, tt.key, output)
				}
			} else {
				if bytes.Contains(buf.Bytes(), []byte("[REDACTED]")) {
					t.Errorf("expected %q value NOT to be redacted, got: %s", tt.key, output)
				}
			}
		})
	}
}

func TestRedactingHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler := &redactingHandler{inner: inner}
	logger := slog.New(handler).With("serverPublicKey", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")

	logger.Info("test msg")
	output := buf.String()

	if !bytes.Contains(buf.Bytes(), []byte("[REDACTED]")) {
		t.Errorf("expected WithAttrs key to be redacted, got: %s", output)
	}
	if bytes.Contains(buf.Bytes(), []byte("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=")) {
		t.Errorf("full key value should not appear in output: %s", output)
	}
}

func TestRedactingHandlerEnabled(t *testing.T) {
	inner := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	handler := &redactingHandler{inner: inner}

	if handler.Enabled(context.Background(), slog.LevelDebug) {
		t.Error("expected debug to be disabled when inner level is warn")
	}
	if !handler.Enabled(context.Background(), slog.LevelError) {
		t.Error("expected error to be enabled when inner level is warn")
	}
}

func TestRedactValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ab", "[REDACTED]"},
		{"abcd", "[REDACTED]"},
		{"abcde", "abcd…[REDACTED]"},
		{"sha256:deadbeef1234567890", "sha2…[REDACTED]"},
	}
	for _, tt := range tests {
		got := redactValue(tt.input)
		if got != tt.expected {
			t.Errorf("redactValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestIsSensitiveKey(t *testing.T) {
	sensitive := []string{"publicKey", "PrivateKeyPath", "access_token", "fingerprint", "nonce", "password", "sha256", "credential", "secret"}
	safe := []string{"vibespace", "addr", "status", "error", "mode", "agent", "name", "port", "attempt"}

	for _, k := range sensitive {
		if !isSensitiveKey(k) {
			t.Errorf("expected %q to be sensitive", k)
		}
	}
	for _, k := range safe {
		if isSensitiveKey(k) {
			t.Errorf("expected %q to NOT be sensitive", k)
		}
	}
}
