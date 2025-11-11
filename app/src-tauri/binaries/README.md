# Bundled Binaries

This directory contains platform-specific binaries bundled with the vibespace Tauri application.

## Structure

```
binaries/
├── macos/
│   └── colima/             # Colima binaries for macOS (existing)
├── linux/
│   └── k3s                 # k3s binary for Linux (existing)
├── kubectl-darwin-amd64    # kubectl for macOS Intel (existing)
├── kubectl-darwin-arm64    # kubectl for macOS Apple Silicon (existing)
├── kubectl-linux-amd64     # kubectl for Linux x86_64 (existing)
├── kubectl-linux-arm64     # kubectl for Linux ARM64 (existing)
├── dnsd-darwin-amd64       # DNS server for macOS Intel
├── dnsd-darwin-arm64       # DNS server for macOS Apple Silicon
├── dnsd-linux-amd64        # DNS server for Linux x86_64
└── dnsd-linux-arm64        # DNS server for Linux ARM64
```

## DNS Binaries

DNS binaries are built via GitHub Actions workflow (`.github/workflows/build-dns.yml`) and should be downloaded as artifacts and placed in the appropriate directories before building the Tauri app.

**Build locally**:
```bash
cd ../../api
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../app/src-tauri/binaries/dnsd-darwin-amd64 ./cmd/dnsd
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../app/src-tauri/binaries/dnsd-darwin-arm64 ./cmd/dnsd
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../app/src-tauri/binaries/dnsd-linux-amd64 ./cmd/dnsd
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o ../app/src-tauri/binaries/dnsd-linux-arm64 ./cmd/dnsd
```

**Download from CI**:
```bash
# Download artifacts from GitHub Actions
gh run download <run-id> -n dnsd-darwin-amd64 -D binaries/
gh run download <run-id> -n dnsd-darwin-arm64 -D binaries/
gh run download <run-id> -n dnsd-linux-amd64 -D binaries/
gh run download <run-id> -n dnsd-linux-arm64 -D binaries/
```

## Usage

These binaries are extracted to `~/.vibespace/bin/` during application installation and managed by the Rust DNS manager (`src-tauri/src/dns_manager.rs`).

## .gitignore

DNS binaries are gitignored to keep the repository size small. They must be built or downloaded before creating release builds.
