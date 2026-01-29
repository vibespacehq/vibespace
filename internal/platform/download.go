package platform

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// downloadBinary downloads a file from URL and saves it as an executable
func downloadBinary(ctx context.Context, url, destPath string) error {
	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Make the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "vibespace-download-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Copy content
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write file: %w", err)
	}
	tmpFile.Close()

	// Make executable
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to chmod: %w", err)
	}

	// Move to final location
	if err := os.Rename(tmpPath, destPath); err != nil {
		// Rename might fail across filesystems, try copy instead
		return copyFile(tmpPath, destPath)
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return os.Chmod(dst, 0755)
}

// GitHubRelease represents a GitHub release response
type GitHubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []GitHubAsset `json:"assets"`
}

// GitHubAsset represents a release asset
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// getGitHubReleaseAssetURL fetches a release from GitHub and returns the download URL for the specified asset
// If tag is empty, fetches the latest release; otherwise fetches the specific tag
func getGitHubReleaseAssetURL(ctx context.Context, owner, repo, tag, assetName string) (string, error) {
	var apiURL string
	if tag == "" {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	} else {
		apiURL = fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned status: %s", resp.Status)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release info: %w", err)
	}

	// Find the matching asset
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			return asset.BrowserDownloadURL, nil
		}
	}

	// List available assets for debugging
	available := make([]string, 0, len(release.Assets))
	for _, asset := range release.Assets {
		available = append(available, asset.Name)
	}

	return "", fmt.Errorf("asset '%s' not found in release %s. Available: %v", assetName, release.TagName, available)
}

// getLatestKubectlVersion fetches the latest stable kubectl version
func getLatestKubectlVersion(ctx context.Context) (string, error) {
	url := "https://dl.k8s.io/release/stable.txt"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch kubectl version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get kubectl version: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// downloadAndExtractTarGz downloads a tar.gz file and extracts it to destDir
func downloadAndExtractTarGz(ctx context.Context, url, destDir string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	return extractTarGz(resp.Body, destDir)
}

// extractTarGz extracts a tar.gz stream to destDir
func extractTarGz(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		// Skip empty or root-only entries
		if header.Name == "" || header.Name == "./" || header.Name == "." {
			continue
		}

		// Security: prevent path traversal
		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("invalid file path (path traversal): %s", header.Name)
		}
		target := filepath.Join(destDir, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			f.Close()

			// Ensure executables are executable
			if header.Mode&0111 != 0 {
				if err := os.Chmod(target, 0755); err != nil {
					return fmt.Errorf("failed to chmod: %w", err)
				}
			}
		case tar.TypeSymlink:
			// Handle symlinks
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				// Ignore if symlink already exists
				if !os.IsExist(err) {
					return fmt.Errorf("failed to create symlink: %w", err)
				}
			}
		}
	}

	return nil
}
