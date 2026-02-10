package remote

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	vserrors "github.com/vibespacehq/vibespace/pkg/errors"
)

func TestGenerateSigningKey(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error: %v", err)
	}

	if pub == "" {
		t.Fatal("public key should not be empty")
	}

	if len(priv) != ed25519.PrivateKeySize {
		t.Errorf("private key length = %d, want %d", len(priv), ed25519.PrivateKeySize)
	}

	// Verify public key is valid base64url
	decoded, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("public key is not valid base64url: %v", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		t.Errorf("decoded public key length = %d, want %d", len(decoded), ed25519.PublicKeySize)
	}
}

func TestEncodeDecodeToken(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error: %v", err)
	}

	token := &InviteToken{
		ServerPublicKey:  "server-pub-key",
		Endpoint:         "example.com:51820",
		ServerIP:         "10.100.0.1",
		ExpiresAt:        time.Now().Add(30 * time.Minute).Unix(),
		Nonce:            "test-nonce-123",
		SigningPublicKey: pub,
		CertFingerprint:  "sha256:abc123",
		Host:             "example.com",
	}

	if err := SignInviteToken(token, priv); err != nil {
		t.Fatalf("SignInviteToken() error: %v", err)
	}

	encoded, err := EncodeInviteToken(token)
	if err != nil {
		t.Fatalf("EncodeInviteToken() error: %v", err)
	}

	decoded, err := DecodeInviteToken(encoded)
	if err != nil {
		t.Fatalf("DecodeInviteToken() error: %v", err)
	}

	if decoded.ServerPublicKey != token.ServerPublicKey {
		t.Errorf("ServerPublicKey = %q, want %q", decoded.ServerPublicKey, token.ServerPublicKey)
	}
	if decoded.Endpoint != token.Endpoint {
		t.Errorf("Endpoint = %q, want %q", decoded.Endpoint, token.Endpoint)
	}
	if decoded.ServerIP != token.ServerIP {
		t.Errorf("ServerIP = %q, want %q", decoded.ServerIP, token.ServerIP)
	}
	if decoded.CertFingerprint != token.CertFingerprint {
		t.Errorf("CertFingerprint = %q, want %q", decoded.CertFingerprint, token.CertFingerprint)
	}
	if decoded.Host != token.Host {
		t.Errorf("Host = %q, want %q", decoded.Host, token.Host)
	}
}

func TestSignVerifyToken(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error: %v", err)
	}

	token := &InviteToken{
		ServerPublicKey:  "test-key",
		Endpoint:         "host:51820",
		ServerIP:         "10.100.0.1",
		ExpiresAt:        time.Now().Add(30 * time.Minute).Unix(),
		Nonce:            "nonce",
		SigningPublicKey: pub,
	}

	if err := SignInviteToken(token, priv); err != nil {
		t.Fatalf("SignInviteToken() error: %v", err)
	}

	// Valid signature should pass
	if err := VerifyInviteToken(token, time.Now()); err != nil {
		t.Errorf("VerifyInviteToken() with valid signature returned error: %v", err)
	}

	// Tampered payload should fail
	tampered := *token
	tampered.ServerIP = "10.200.0.1"
	err = VerifyInviteToken(&tampered, time.Now())
	if err == nil {
		t.Error("VerifyInviteToken() with tampered payload should return error")
	}
	if !strings.Contains(err.Error(), "signature") {
		t.Errorf("tampered token error should mention signature, got: %v", err)
	}
}

func TestExpiredToken(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error: %v", err)
	}

	token := &InviteToken{
		ServerPublicKey:  "test-key",
		Endpoint:         "host:51820",
		ServerIP:         "10.100.0.1",
		ExpiresAt:        time.Now().Add(-1 * time.Hour).Unix(), // expired
		Nonce:            "nonce",
		SigningPublicKey: pub,
	}

	if err := SignInviteToken(token, priv); err != nil {
		t.Fatalf("SignInviteToken() error: %v", err)
	}

	err = VerifyInviteToken(token, time.Now())
	if err == nil {
		t.Fatal("VerifyInviteToken() with expired token should return error")
	}
	if !strings.Contains(err.Error(), vserrors.ErrInviteTokenExpired.Error()) {
		t.Errorf("expired token error should wrap ErrInviteTokenExpired, got: %v", err)
	}
}

func TestTokenPrefix(t *testing.T) {
	pub, priv, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error: %v", err)
	}

	token := &InviteToken{
		ServerPublicKey:  "test-key",
		Endpoint:         "host:51820",
		ServerIP:         "10.100.0.1",
		ExpiresAt:        time.Now().Add(30 * time.Minute).Unix(),
		Nonce:            "nonce",
		SigningPublicKey: pub,
	}

	if err := SignInviteToken(token, priv); err != nil {
		t.Fatalf("SignInviteToken() error: %v", err)
	}

	encoded, err := EncodeInviteToken(token)
	if err != nil {
		t.Fatalf("EncodeInviteToken() error: %v", err)
	}

	if !strings.HasPrefix(encoded, "vs-") {
		t.Errorf("encoded token should start with 'vs-', got prefix: %q", encoded[:10])
	}
}

func TestVerifyTokenMissingFields(t *testing.T) {
	pub, _, err := GenerateSigningKey()
	if err != nil {
		t.Fatalf("GenerateSigningKey() error: %v", err)
	}

	tests := []struct {
		name  string
		token InviteToken
	}{
		{"missing ServerPublicKey", InviteToken{
			Endpoint: "host:51820", ServerIP: "10.100.0.1",
			ExpiresAt: time.Now().Add(30 * time.Minute).Unix(), Nonce: "n",
			SigningPublicKey: pub, Signature: "sig",
		}},
		{"missing Endpoint", InviteToken{
			ServerPublicKey: "key", ServerIP: "10.100.0.1",
			ExpiresAt: time.Now().Add(30 * time.Minute).Unix(), Nonce: "n",
			SigningPublicKey: pub, Signature: "sig",
		}},
		{"missing Signature", InviteToken{
			ServerPublicKey: "key", Endpoint: "host:51820", ServerIP: "10.100.0.1",
			ExpiresAt: time.Now().Add(30 * time.Minute).Unix(), Nonce: "n",
			SigningPublicKey: pub,
		}},
		{"missing Nonce", InviteToken{
			ServerPublicKey: "key", Endpoint: "host:51820", ServerIP: "10.100.0.1",
			ExpiresAt:        time.Now().Add(30 * time.Minute).Unix(),
			SigningPublicKey: pub, Signature: "sig",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifyInviteToken(&tt.token, time.Now())
			if err == nil {
				t.Error("VerifyInviteToken() with missing fields should return error")
			}
			if !strings.Contains(err.Error(), vserrors.ErrInviteTokenInvalid.Error()) {
				t.Errorf("error should wrap ErrInviteTokenInvalid, got: %v", err)
			}
		})
	}
}

func TestEncodeUnsignedToken(t *testing.T) {
	token := &InviteToken{
		ServerPublicKey: "key",
		Endpoint:        "host:51820",
		ServerIP:        "10.100.0.1",
	}

	_, err := EncodeInviteToken(token)
	if err == nil {
		t.Error("EncodeInviteToken() with unsigned token should return error")
	}
}
