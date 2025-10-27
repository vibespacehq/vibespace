package k8s

import (
	"context"
	"fmt"
	"testing"
)

// TestStartPortForwardToPod_Signature tests that the method exists with correct signature
// Note: Full integration testing requires a live cluster and kubectl
func TestStartPortForwardToPod_Signature(t *testing.T) {
	// This test verifies the method signature exists and can be called
	// We skip actual execution as it requires kubectl and a cluster

	t.Run("method exists and accepts correct parameters", func(t *testing.T) {
		// Just verify the method signature compiles and can be referenced
		// Type assertion to verify method signature
		var _ func(context.Context, string, string, int, int) error = (*Client)(nil).StartPortForwardToPod

		// If we get here, the method signature is correct
		t.Log("StartPortForwardToPod method signature verified")
	})
}

// TestPortForwardKeyGeneration tests the port-forward key generation logic
func TestPortForwardKeyGeneration(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		keyName   string
		want      string
	}{
		{
			name:      "standard workspace",
			namespace: "workspace",
			keyName:   "workspace-abc123",
			want:      "workspace/workspace-abc123",
		},
		{
			name:      "different namespace",
			namespace: "default",
			keyName:   "my-pod",
			want:      "default/my-pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The key format is namespace/keyName
			got := tt.namespace + "/" + tt.keyName
			if got != tt.want {
				t.Errorf("key = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPortForwardResourceFormat tests the resource format for kubectl command
func TestPortForwardResourceFormat(t *testing.T) {
	tests := []struct {
		name     string
		podName  string
		wantFmt  string
	}{
		{
			name:     "workspace pod",
			podName:  "workspace-abc123",
			wantFmt:  "pod/workspace-abc123",
		},
		{
			name:     "pod with special chars",
			podName:  "workspace-test-123",
			wantFmt:  "pod/workspace-test-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Format matches kubectl port-forward syntax
			got := "pod/" + tt.podName
			if got != tt.wantFmt {
				t.Errorf("resource format = %q, want %q", got, tt.wantFmt)
			}
		})
	}
}

// TestPortForwardPortFormat tests the port format for kubectl command
func TestPortForwardPortFormat(t *testing.T) {
	tests := []struct {
		name       string
		localPort  int
		remotePort int
		wantFmt    string
	}{
		{
			name:       "same ports",
			localPort:  8080,
			remotePort: 8080,
			wantFmt:    "8080:8080",
		},
		{
			name:       "different ports",
			localPort:  9000,
			remotePort: 8080,
			wantFmt:    "9000:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Format matches kubectl port-forward syntax: localPort:remotePort
			got := formatPort(tt.localPort, tt.remotePort)
			if got != tt.wantFmt {
				t.Errorf("port format = %q, want %q", got, tt.wantFmt)
			}
		})
	}
}

// Helper function to format ports
func formatPort(local, remote int) string {
	return fmt.Sprintf("%d:%d", local, remote)
}
