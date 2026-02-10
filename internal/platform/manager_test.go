package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadClusterState(t *testing.T) {
	dir := t.TempDir()

	if err := SaveClusterState(dir, ClusterModeBareMetal); err != nil {
		t.Fatalf("SaveClusterState error: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, "cluster.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cluster.json should exist: %v", err)
	}

	state, err := LoadClusterState(dir)
	if err != nil {
		t.Fatalf("LoadClusterState error: %v", err)
	}

	if state.Mode != ClusterModeBareMetal {
		t.Errorf("Mode = %q, want %q", state.Mode, ClusterModeBareMetal)
	}
}

func TestLoadClusterStateMissing(t *testing.T) {
	dir := t.TempDir()

	_, err := LoadClusterState(dir)
	if err == nil {
		t.Error("LoadClusterState on missing file should return error")
	}
}

func TestSaveLoadClusterStateModes(t *testing.T) {
	modes := []ClusterMode{ClusterModeColima, ClusterModeLima, ClusterModeBareMetal}

	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			dir := t.TempDir()
			if err := SaveClusterState(dir, mode); err != nil {
				t.Fatalf("SaveClusterState error: %v", err)
			}
			state, err := LoadClusterState(dir)
			if err != nil {
				t.Fatalf("LoadClusterState error: %v", err)
			}
			if state.Mode != mode {
				t.Errorf("Mode = %q, want %q", state.Mode, mode)
			}
		})
	}
}

func TestNewClusterManagerDarwin(t *testing.T) {
	dir := t.TempDir()
	p := Platform{OS: "darwin", Arch: "arm64"}

	mgr, err := NewClusterManager(p, dir, ClusterManagerOptions{})
	if err != nil {
		t.Fatalf("NewClusterManager error: %v", err)
	}

	if _, ok := mgr.(*ColimaManager); !ok {
		t.Errorf("darwin should create ColimaManager, got %T", mgr)
	}
}

func TestNewClusterManagerLinux(t *testing.T) {
	dir := t.TempDir()
	p := Platform{OS: "linux", Arch: "amd64"}

	mgr, err := NewClusterManager(p, dir, ClusterManagerOptions{})
	if err != nil {
		t.Fatalf("NewClusterManager error: %v", err)
	}

	if _, ok := mgr.(*LimaManager); !ok {
		t.Errorf("linux should create LimaManager, got %T", mgr)
	}
}

func TestNewClusterManagerLinuxBareMetal(t *testing.T) {
	dir := t.TempDir()
	p := Platform{OS: "linux", Arch: "amd64"}

	mgr, err := NewClusterManager(p, dir, ClusterManagerOptions{BareMetal: true})
	if err != nil {
		t.Fatalf("NewClusterManager error: %v", err)
	}

	if _, ok := mgr.(*BareMetalManager); !ok {
		t.Errorf("linux+BareMetal should create BareMetalManager, got %T", mgr)
	}
}

func TestNewClusterManagerUnsupported(t *testing.T) {
	dir := t.TempDir()
	p := Platform{OS: "windows", Arch: "amd64"}

	_, err := NewClusterManager(p, dir, ClusterManagerOptions{})
	if err == nil {
		t.Error("unsupported platform should return error")
	}
}

func TestNewClusterManagerFromPersistedState(t *testing.T) {
	dir := t.TempDir()

	// Persist bare metal mode
	if err := SaveClusterState(dir, ClusterModeBareMetal); err != nil {
		t.Fatalf("SaveClusterState error: %v", err)
	}

	// Even on darwin, persisted state should override
	p := Platform{OS: "darwin", Arch: "arm64"}
	mgr, err := NewClusterManager(p, dir, ClusterManagerOptions{})
	if err != nil {
		t.Fatalf("NewClusterManager error: %v", err)
	}

	if _, ok := mgr.(*BareMetalManager); !ok {
		t.Errorf("persisted BareMetal should create BareMetalManager, got %T", mgr)
	}
}
