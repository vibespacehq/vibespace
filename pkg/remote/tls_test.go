package remote

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"strings"
	"testing"
)

func TestGenerateSelfSignedCert(t *testing.T) {
	certPEM, keyPEM, fingerprint, err := GenerateSelfSignedCert("192.168.1.1")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() error: %v", err)
	}

	// Check cert PEM
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	if block.Type != "CERTIFICATE" {
		t.Errorf("cert PEM type = %q, want %q", block.Type, "CERTIFICATE")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	// Check CN
	if cert.Subject.CommonName != "vibespace registration" {
		t.Errorf("CN = %q, want %q", cert.Subject.CommonName, "vibespace registration")
	}

	// Check key PEM
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		t.Fatal("failed to decode key PEM")
	}
	if keyBlock.Type != "EC PRIVATE KEY" {
		t.Errorf("key PEM type = %q, want %q", keyBlock.Type, "EC PRIVATE KEY")
	}

	// Check fingerprint format
	if !strings.HasPrefix(fingerprint, "sha256:") {
		t.Errorf("fingerprint should start with 'sha256:', got %q", fingerprint)
	}

	// Verify fingerprint matches cert
	hash := sha256.Sum256(block.Bytes)
	expectedFP := "sha256:" + hex.EncodeToString(hash[:])
	if fingerprint != expectedFP {
		t.Errorf("fingerprint = %q, want %q", fingerprint, expectedFP)
	}
}

func TestGenerateSelfSignedCertDNS(t *testing.T) {
	certPEM, _, _, err := GenerateSelfSignedCert("example.com")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert(hostname) error: %v", err)
	}

	block, _ := pem.Decode(certPEM)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	if len(cert.DNSNames) == 0 {
		t.Error("cert should have DNS SANs for hostname input")
	}
}

func TestPinningTLSConfig(t *testing.T) {
	certPEM, _, fingerprint, err := GenerateSelfSignedCert("127.0.0.1")
	if err != nil {
		t.Fatalf("GenerateSelfSignedCert() error: %v", err)
	}

	block, _ := pem.Decode(certPEM)

	// Matching fingerprint should pass
	config := PinningTLSConfig(fingerprint)
	err = config.VerifyPeerCertificate([][]byte{block.Bytes}, nil)
	if err != nil {
		t.Errorf("matching fingerprint should pass, got error: %v", err)
	}

	// Wrong fingerprint should fail
	wrongConfig := PinningTLSConfig("sha256:0000000000000000000000000000000000000000000000000000000000000000")
	err = wrongConfig.VerifyPeerCertificate([][]byte{block.Bytes}, nil)
	if err == nil {
		t.Error("wrong fingerprint should fail")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("error should mention fingerprint mismatch, got: %v", err)
	}

	// No certs should fail
	err = config.VerifyPeerCertificate(nil, nil)
	if err == nil {
		t.Error("no certificate should fail")
	}
}
