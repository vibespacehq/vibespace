package template

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MockK8sClient implements K8sClient interface for testing
type MockK8sClient struct {
	portForwards map[string]bool
	shouldFail   bool
}

func (m *MockK8sClient) StartPortForward(ctx context.Context, namespace, service string, localPort, remotePort int) error {
	if m.shouldFail {
		return context.DeadlineExceeded
	}
	key := namespace + "/" + service
	m.portForwards[key] = true
	return nil
}

func (m *MockK8sClient) StopPortForward(namespace, service string) error {
	key := namespace + "/" + service
	delete(m.portForwards, key)
	return nil
}

func TestNewBuilder(t *testing.T) {
	mockClient := &MockK8sClient{portForwards: make(map[string]bool)}
	builder := NewBuilder("localhost:5000", mockClient)

	if builder == nil {
		t.Fatal("NewBuilder returned nil")
	}

	if builder.registryURL != "localhost:5000" {
		t.Errorf("Expected registryURL 'localhost:5000', got '%s'", builder.registryURL)
	}

	expectedAddr := "tcp://127.0.0.1:1234"
	if builder.buildkitAddr != expectedAddr {
		t.Errorf("Expected buildkitAddr '%s', got '%s'", expectedAddr, builder.buildkitAddr)
	}
}

func TestCleanupOldTempDirs(t *testing.T) {
	// Create old temp directory
	tempDir, err := os.MkdirTemp("", "vibespace-build-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Make it look old by changing mod time
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(tempDir, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to change temp dir time: %v", err)
	}

	// Create builder which should clean up old dirs
	mockClient := &MockK8sClient{portForwards: make(map[string]bool)}
	_ = NewBuilder("localhost:5000", mockClient)

	// Check if old temp dir was cleaned up
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("Old temp directory was not cleaned up: %s", tempDir)
		os.RemoveAll(tempDir) // Cleanup for test
	}
}

func TestBuildImageCreatesTempDir(t *testing.T) {
	mockClient := &MockK8sClient{portForwards: make(map[string]bool)}
	builder := NewBuilder("localhost:5000", mockClient)

	// We can't fully test BuildImage without BuildKit, but we can test
	// that it creates temp directories correctly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// This will fail to connect to BuildKit, but temp dir should be created and cleaned
	progressCalled := false
	progressFn := func(progress BuildProgress) {
		progressCalled = true
		// Should receive error about BuildKit connection
		if progress.Status == "error" && !strings.Contains(progress.Error, "BuildKit") {
			t.Logf("Progress update: %+v", progress)
		}
	}

	err := builder.BuildImage(ctx, "base", "claude", progressFn)
	if err == nil {
		t.Error("Expected error when BuildKit is not available")
	}

	if !progressCalled {
		t.Error("Progress function was not called")
	}

	// Verify no temp dirs left behind
	tmpDir := os.TempDir()
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "vibespace-build-base-claude") {
			t.Errorf("Temp directory not cleaned up: %s", entry.Name())
			os.RemoveAll(filepath.Join(tmpDir, entry.Name()))
		}
	}
}

func TestConstants(t *testing.T) {
	// Verify constants are set correctly
	if BuildKitPort != 1234 {
		t.Errorf("BuildKitPort = %d, want 1234", BuildKitPort)
	}

	if RegistryPort != 5000 {
		t.Errorf("RegistryPort = %d, want 5000", RegistryPort)
	}

	if CodeServerPort != 8080 {
		t.Errorf("CodeServerPort = %d, want 8080", CodeServerPort)
	}

	if JupyterLabPort != 8888 {
		t.Errorf("JupyterLabPort = %d, want 8888", JupyterLabPort)
	}

	// Verify default timeout values
	if DefaultBuildKitReadyTimeout != 30*time.Second {
		t.Errorf("DefaultBuildKitReadyTimeout = %v, want 30s", DefaultBuildKitReadyTimeout)
	}

	if DefaultTempDirCleanupAge != time.Hour {
		t.Errorf("DefaultTempDirCleanupAge = %v, want 1h", DefaultTempDirCleanupAge)
	}

	// Verify that timeout variables are set (they may differ from defaults if env vars are set)
	if BuildKitReadyTimeout <= 0 {
		t.Errorf("BuildKitReadyTimeout must be positive, got %v", BuildKitReadyTimeout)
	}

	if TempDirCleanupAge <= 0 {
		t.Errorf("TempDirCleanupAge must be positive, got %v", TempDirCleanupAge)
	}
}

func TestGetTimeoutFromEnv(t *testing.T) {
	tests := []struct {
		name         string
		envVar       string
		envValue     string
		defaultValue time.Duration
		expected     time.Duration
	}{
		{
			name:         "no env var set",
			envVar:       "TEST_TIMEOUT_NONE",
			envValue:     "",
			defaultValue: 30 * time.Second,
			expected:     30 * time.Second,
		},
		{
			name:         "valid env var",
			envVar:       "TEST_TIMEOUT_VALID",
			envValue:     "1m",
			defaultValue: 30 * time.Second,
			expected:     time.Minute,
		},
		{
			name:         "invalid env var",
			envVar:       "TEST_TIMEOUT_INVALID",
			envValue:     "not-a-duration",
			defaultValue: 30 * time.Second,
			expected:     30 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env var if specified
			if tt.envValue != "" {
				os.Setenv(tt.envVar, tt.envValue)
				defer os.Unsetenv(tt.envVar)
			}

			got := getTimeoutFromEnv(tt.envVar, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("getTimeoutFromEnv() = %v, want %v", got, tt.expected)
			}
		})
	}
}
