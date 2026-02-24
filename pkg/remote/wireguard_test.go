package remote

import (
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
