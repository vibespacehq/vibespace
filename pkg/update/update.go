// Package update provides version checking and self-update functionality
// for the vibespace CLI using GitHub Releases.
package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	// githubOwner is the GitHub repository owner.
	githubOwner = "vibespacehq"
	// githubRepo is the GitHub repository name.
	githubRepo = "vibespace"
	// cacheFileName is the name of the update check cache file.
	cacheFileName = "update-check.json"
	// cacheTTL is how long a cached check remains valid.
	cacheTTL = 24 * time.Hour
	// apiTimeout is the timeout for GitHub API calls during background checks.
	apiTimeout = 5 * time.Second
	// downloadTimeout is the timeout for downloading release assets.
	downloadTimeout = 5 * time.Minute
)

// UpdateInfo contains information about an available update.
type UpdateInfo struct {
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
}

// updateCache is the on-disk cache structure.
type updateCache struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

// gitHubRelease represents a GitHub release API response.
type gitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []gitHubAsset `json:"assets"`
}

// gitHubAsset represents a release asset.
type gitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckForUpdate checks if a newer version is available.
// Returns nil if current version is up-to-date or if the check cannot be performed.
// This uses a 24-hour cache to avoid hitting the GitHub API on every invocation.
func CheckForUpdate(currentVersion string) *UpdateInfo {
	if !isReleaseBuild(currentVersion) {
		return nil
	}

	cache, err := loadCache()
	if err == nil && time.Since(cache.CheckedAt) < cacheTTL {
		// Cache is fresh
		if IsNewer(cache.LatestVersion, currentVersion) {
			return &UpdateInfo{
				CurrentVersion: currentVersion,
				LatestVersion:  cache.LatestVersion,
			}
		}
		return nil
	}

	// Cache is stale or missing — fetch from GitHub
	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	release, err := getLatestRelease(ctx)
	if err != nil {
		slog.Debug("update check failed", "error", err)
		return nil
	}

	// Save to cache
	if err := saveCache(&updateCache{
		LatestVersion: release.TagName,
		CheckedAt:     time.Now(),
	}); err != nil {
		slog.Debug("failed to save update cache", "error", err)
	}

	if IsNewer(release.TagName, currentVersion) {
		return &UpdateInfo{
			CurrentVersion: currentVersion,
			LatestVersion:  release.TagName,
		}
	}
	return nil
}

// GetLatestVersion fetches the latest release version from GitHub.
// Unlike CheckForUpdate, this always hits the API (used by the upgrade command).
func GetLatestVersion(ctx context.Context) (string, error) {
	release, err := getLatestRelease(ctx)
	if err != nil {
		return "", err
	}
	return release.TagName, nil
}

// DownloadAndReplace downloads the specified version and replaces the current binary.
func DownloadAndReplace(ctx context.Context, version string) (string, error) {
	// Resolve current binary path
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to determine executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	// Construct asset names
	tarballName := AssetName(version, runtime.GOOS, runtime.GOARCH)
	checksumsName := fmt.Sprintf("vibespace-%s-checksums.txt", version)

	// Fetch release to get download URLs
	release, err := getReleaseByTag(ctx, version)
	if err != nil {
		return "", fmt.Errorf("failed to get release %s: %w", version, err)
	}

	tarballURL := findAssetURL(release, tarballName)
	if tarballURL == "" {
		return "", fmt.Errorf("asset %s not found in release %s", tarballName, version)
	}

	checksumsURL := findAssetURL(release, checksumsName)
	if checksumsURL == "" {
		return "", fmt.Errorf("checksums file %s not found in release %s", checksumsName, version)
	}

	// Download and parse checksums
	checksumsBody, err := fetchText(ctx, checksumsURL)
	if err != nil {
		return "", fmt.Errorf("failed to download checksums: %w", err)
	}
	expectedHash, err := parseSHA256SUMS(checksumsBody, tarballName)
	if err != nil {
		return "", err
	}

	// Download tarball to temp file
	tmpTarball, err := os.CreateTemp("", "vibespace-upgrade-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpTarballPath := tmpTarball.Name()
	defer os.Remove(tmpTarballPath)

	if err := downloadToFile(ctx, tarballURL, tmpTarball); err != nil {
		tmpTarball.Close()
		return "", fmt.Errorf("failed to download %s: %w", tarballName, err)
	}
	tmpTarball.Close()

	// Verify SHA256
	if err := verifySHA256(tmpTarballPath, expectedHash); err != nil {
		return "", fmt.Errorf("integrity verification failed: %w", err)
	}
	slog.Debug("SHA256 verified", "asset", tarballName, "sha256", expectedHash[:12]+"...")

	// Extract binary from tarball
	tmpBinDir, err := os.MkdirTemp("", "vibespace-extract-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpBinDir)

	if err := extractTarGz(tmpTarballPath, tmpBinDir); err != nil {
		return "", fmt.Errorf("failed to extract tarball: %w", err)
	}

	extractedBinary := filepath.Join(tmpBinDir, "vibespace")
	if _, err := os.Stat(extractedBinary); err != nil {
		return "", fmt.Errorf("vibespace binary not found in tarball")
	}

	// Replace current binary: write to temp in same dir, then atomic rename
	tmpNewBin, err := os.CreateTemp(filepath.Dir(execPath), ".vibespace-upgrade-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file for replacement (permission denied? try: sudo vibespace upgrade): %w", err)
	}
	tmpNewBinPath := tmpNewBin.Name()

	src, err := os.Open(extractedBinary)
	if err != nil {
		tmpNewBin.Close()
		os.Remove(tmpNewBinPath)
		return "", fmt.Errorf("failed to open extracted binary: %w", err)
	}

	if _, err := io.Copy(tmpNewBin, src); err != nil {
		src.Close()
		tmpNewBin.Close()
		os.Remove(tmpNewBinPath)
		return "", fmt.Errorf("failed to copy binary: %w", err)
	}
	src.Close()
	tmpNewBin.Close()

	if err := os.Chmod(tmpNewBinPath, 0755); err != nil {
		os.Remove(tmpNewBinPath)
		return "", fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := os.Rename(tmpNewBinPath, execPath); err != nil {
		os.Remove(tmpNewBinPath)
		return "", fmt.Errorf("failed to replace binary (permission denied? try: sudo vibespace upgrade): %w", err)
	}

	// Clear the update check cache
	ClearCache()

	return execPath, nil
}

// ClearCache removes the update check cache file.
func ClearCache() {
	path, err := cachePath()
	if err != nil {
		return
	}
	os.Remove(path)
}

// AssetName returns the tarball asset name for a given version, OS, and architecture.
func AssetName(version, goos, goarch string) string {
	return fmt.Sprintf("vibespace-%s-%s-%s.tar.gz", version, goos, goarch)
}

// --- internal helpers ---

// isReleaseBuild returns true if the version looks like a release (not "dev" or dirty).
func isReleaseBuild(version string) bool {
	if version == "dev" || version == "" {
		return false
	}
	// Skip dirty or snapshot builds
	if strings.Contains(version, "-dirty") {
		return false
	}
	// Must be valid semver
	v := version
	if v[0] != 'v' {
		v = "v" + v
	}
	return semver.IsValid(v)
}

// IsNewer returns true if latest is newer than current.
func IsNewer(latest, current string) bool {
	l := normalize(latest)
	c := normalize(current)
	if !semver.IsValid(l) || !semver.IsValid(c) {
		return false
	}
	return semver.Compare(l, c) > 0
}

// normalize ensures a version string has a "v" prefix.
func normalize(version string) string {
	if version == "" {
		return ""
	}
	if version[0] != 'v' {
		return "v" + version
	}
	return version
}

func getLatestRelease(ctx context.Context) (*gitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubOwner, githubRepo)
	return fetchRelease(ctx, url)
}

func getReleaseByTag(ctx context.Context, tag string) (*gitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", githubOwner, githubRepo, tag)
	return fetchRelease(ctx, url)
}

func fetchRelease(ctx context.Context, url string) (*gitHubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %s", resp.Status)
	}

	var release gitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release: %w", err)
	}
	return &release, nil
}

func findAssetURL(release *gitHubRelease, name string) string {
	for _, a := range release.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func fetchText(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %s", resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func downloadToFile(ctx context.Context, url string, f *os.File) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	_, err = io.Copy(f, resp.Body)
	return err
}

func parseSHA256SUMS(content, assetName string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("hash not found for %s in checksums", assetName)
}

func verifySHA256(filePath, expectedHex string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	actual := hex.EncodeToString(h.Sum(nil))
	if actual != expectedHex {
		return fmt.Errorf("SHA256 mismatch: expected %s, got %s", expectedHex, actual)
	}
	return nil
}

func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Security: prevent path traversal
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("invalid path in tarball: %s", header.Name)
		}

		target := filepath.Join(destDir, filepath.Base(cleanName))
		out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return err
		}
		out.Close()

		if header.Mode&0111 != 0 {
			os.Chmod(target, 0755)
		}
	}
	return nil
}

// --- cache helpers ---

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".vibespace", cacheFileName), nil
}

func loadCache() (*updateCache, error) {
	path, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache updateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func saveCache(cache *updateCache) error {
	path, err := cachePath()
	if err != nil {
		return err
	}
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
