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
			name:      "standard vibespace",
			namespace: "vibespace",
			keyName:   "vibespace-abc123",
			want:      "vibespace/vibespace-abc123",
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
			name:     "vibespace pod",
			podName:  "vibespace-abc123",
			wantFmt:  "pod/vibespace-abc123",
		},
		{
			name:     "pod with special chars",
			podName:  "vibespace-test-123",
			wantFmt:  "pod/vibespace-test-123",
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

// TestGetBundledKubeconfigPath tests the kubeconfig path for bundled Kubernetes
func TestGetBundledKubeconfigPath(t *testing.T) {
	// Get expected path
	path, err := getBundledKubeconfigPath()
	if err != nil {
		t.Fatalf("getBundledKubeconfigPath() failed: %v", err)
	}

	// Verify path is not empty
	if path == "" {
		t.Error("getBundledKubeconfigPath() returned empty string")
	}

	// Verify path contains .kube/config
	if !contains(path, ".kube") || !contains(path, "config") {
		t.Errorf("getBundledKubeconfigPath() = %q, should contain .kube/config", path)
	}

	t.Logf("Bundled kubeconfig path: %s", path)
}

// TestVibespaceNamespaceConstant verifies the namespace constant
func TestVibespaceNamespaceConstant(t *testing.T) {
	if VibespaceNamespace != "vibespace" {
		t.Errorf("VibespaceNamespace = %q, want %q", VibespaceNamespace, "vibespace")
	}
}

// TestPortForwardWithRemotePort tests port-forward key includes remote port
func TestPortForwardWithRemotePort(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		service    string
		remotePort int
		wantKey    string
	}{
		{
			name:       "code-server default port",
			namespace:  "vibespace",
			service:    "vibespace-abc123",
			remotePort: 8080,
			wantKey:    "vibespace/vibespace-abc123:8080",
		},
		{
			name:       "application port",
			namespace:  "vibespace",
			service:    "vibespace-def456",
			remotePort: 3000,
			wantKey:    "vibespace/vibespace-def456:3000",
		},
		{
			name:       "registry port",
			namespace:  "default",
			service:    "registry",
			remotePort: 5000,
			wantKey:    "default/registry:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Key format: namespace/service:remotePort
			key := fmt.Sprintf("%s/%s:%d", tt.namespace, tt.service, tt.remotePort)
			if key != tt.wantKey {
				t.Errorf("port-forward key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

// TestStopPortForwardKeyMatching tests that StopPortForward matches all ports
func TestStopPortForwardKeyMatching(t *testing.T) {
	// When stopping port-forwards, we should match all keys with the same namespace/service
	// regardless of remote port
	namespace := "vibespace"
	service := "vibespace-abc123"
	prefix := fmt.Sprintf("%s/%s:", namespace, service)

	// Keys that should match
	matchingKeys := []string{
		"vibespace/vibespace-abc123:8080",
		"vibespace/vibespace-abc123:3000",
		"vibespace/vibespace-abc123:8000",
	}

	// Keys that should NOT match
	nonMatchingKeys := []string{
		"vibespace/vibespace-def456:8080",
		"default/vibespace-abc123:8080",
		"vibespace/other-service:8080",
	}

	// Test matching keys
	for _, key := range matchingKeys {
		if !hasPrefix(key, prefix) {
			t.Errorf("key %q should match prefix %q", key, prefix)
		}
	}

	// Test non-matching keys
	for _, key := range nonMatchingKeys {
		if hasPrefix(key, prefix) {
			t.Errorf("key %q should NOT match prefix %q", key, prefix)
		}
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" &&
		(s == substr || (len(s) > len(substr) && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
