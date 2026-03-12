package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsReleaseBuild(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"dev", false},
		{"", false},
		{"v0.4.0", true},
		{"0.4.0", true},
		{"v1.0.0", true},
		{"v0.4.0-dirty", false},
		{"v0.4.0-alpha.1", true},
		{"notaversion", false},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := isReleaseBuild(tt.version)
			if got != tt.want {
				t.Errorf("isReleaseBuild(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v0.5.0", "v0.4.0", true},
		{"v0.4.0", "v0.4.0", false},
		{"v0.3.0", "v0.4.0", false},
		{"v1.0.0", "v0.9.9", true},
		{"0.5.0", "0.4.0", true},       // without v prefix
		{"v0.5.0", "0.4.0", true},       // mixed prefix
		{"v0.5.0-alpha.1", "v0.4.0", true},
		{"invalid", "v0.4.0", false},
		{"v0.4.0", "invalid", false},
	}
	for _, tt := range tests {
		t.Run(tt.latest+"_vs_"+tt.current, func(t *testing.T) {
			got := IsNewer(tt.latest, tt.current)
			if got != tt.want {
				t.Errorf("IsNewer(%q, %q) = %v, want %v", tt.latest, tt.current, got, tt.want)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v0.4.0", "v0.4.0"},
		{"0.4.0", "v0.4.0"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalize(tt.input)
			if got != tt.want {
				t.Errorf("normalize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestAssetName(t *testing.T) {
	tests := []struct {
		version string
		goos    string
		goarch  string
		want    string
	}{
		{"v0.4.0", "darwin", "arm64", "vibespace-v0.4.0-darwin-arm64.tar.gz"},
		{"v0.4.0", "linux", "amd64", "vibespace-v0.4.0-linux-amd64.tar.gz"},
		{"v1.0.0", "darwin", "amd64", "vibespace-v1.0.0-darwin-amd64.tar.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := AssetName(tt.version, tt.goos, tt.goarch)
			if got != tt.want {
				t.Errorf("AssetName(%q, %q, %q) = %q, want %q", tt.version, tt.goos, tt.goarch, got, tt.want)
			}
		})
	}
}

func TestCacheLoadSave(t *testing.T) {
	// Override home directory for test
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Ensure .vibespace directory exists
	os.MkdirAll(filepath.Join(tmpDir, ".vibespace"), 0755)

	// Should fail to load when no cache exists
	_, err := loadCache()
	if err == nil {
		t.Fatal("expected error loading non-existent cache")
	}

	// Save and reload
	now := time.Now().Truncate(time.Second)
	cache := &updateCache{
		LatestVersion: "v0.5.0",
		CheckedAt:     now,
	}
	if err := saveCache(cache); err != nil {
		t.Fatalf("saveCache: %v", err)
	}

	loaded, err := loadCache()
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if loaded.LatestVersion != "v0.5.0" {
		t.Errorf("LatestVersion = %q, want %q", loaded.LatestVersion, "v0.5.0")
	}
	if !loaded.CheckedAt.Equal(now) {
		t.Errorf("CheckedAt = %v, want %v", loaded.CheckedAt, now)
	}
}

func TestCacheTTL(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	os.MkdirAll(filepath.Join(tmpDir, ".vibespace"), 0755)

	// Save a fresh cache
	cache := &updateCache{
		LatestVersion: "v0.5.0",
		CheckedAt:     time.Now(),
	}
	saveCache(cache)

	loaded, err := loadCache()
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}

	// Fresh cache should be within TTL
	if time.Since(loaded.CheckedAt) >= cacheTTL {
		t.Error("fresh cache should be within TTL")
	}

	// Save an expired cache
	cache.CheckedAt = time.Now().Add(-25 * time.Hour)
	saveCache(cache)

	loaded, err = loadCache()
	if err != nil {
		t.Fatalf("loadCache: %v", err)
	}
	if time.Since(loaded.CheckedAt) < cacheTTL {
		t.Error("expired cache should be past TTL")
	}
}

func TestParseSHA256SUMS(t *testing.T) {
	content := `abc123def456  vibespace-v0.4.0-darwin-arm64.tar.gz
789xyz  vibespace-v0.4.0-linux-amd64.tar.gz`

	hash, err := parseSHA256SUMS(content, "vibespace-v0.4.0-darwin-arm64.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hash != "abc123def456" {
		t.Errorf("got %q, want %q", hash, "abc123def456")
	}

	_, err = parseSHA256SUMS(content, "nonexistent.tar.gz")
	if err == nil {
		t.Fatal("expected error for missing asset")
	}
}

func TestClearCache(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	vsDir := filepath.Join(tmpDir, ".vibespace")
	os.MkdirAll(vsDir, 0755)

	// Write a cache file
	data, _ := json.Marshal(&updateCache{LatestVersion: "v1.0.0", CheckedAt: time.Now()})
	os.WriteFile(filepath.Join(vsDir, cacheFileName), data, 0644)

	// Clear it
	ClearCache()

	// Should be gone
	_, err := os.Stat(filepath.Join(vsDir, cacheFileName))
	if err == nil {
		t.Error("cache file should have been removed")
	}
}
