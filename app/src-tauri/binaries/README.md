# Bundled Kubernetes Binaries

This directory contains scripts to download Kubernetes runtime binaries that are bundled with the vibespace application.

## Purpose

Instead of requiring users to install Kubernetes manually (Rancher Desktop, k3d, k3s, etc.), vibespace bundles the necessary Kubernetes runtime to provide a zero-configuration installation experience.

**See [ADR 0006: Bundle Kubernetes Runtime](../../../docs/adr/0006-bundled-kubernetes-runtime.md) for the architectural decision.**

## Directory Structure

```
binaries/
├── macos/
│   ├── download.sh       # Downloads Colima and Lima for macOS
│   ├── colima            # Colima binary (~20MB, not in git)
│   └── lima              # Lima binary (~30MB, not in git)
├── linux/
│   ├── download.sh       # Downloads k3s for Linux
│   └── k3s               # k3s binary (~50MB, not in git)
├── download-kubectl.sh   # Downloads kubectl (shared)
├── kubectl-darwin-amd64  # kubectl for macOS Intel (~50MB, not in git)
├── kubectl-darwin-arm64  # kubectl for macOS ARM (~50MB, not in git)
├── kubectl-linux-amd64   # kubectl for Linux x64 (~50MB, not in git)
├── kubectl-linux-arm64   # kubectl for Linux ARM (~50MB, not in git)
└── README.md             # This file
```

## Binaries

### macOS
- **Colima** (v0.8.2): Container runtime for macOS with Kubernetes support
  - Manages Lima VM lifecycle
  - Provides `colima start --kubernetes` command
  - Source: https://github.com/abiosoft/colima
- **Lima** (v1.0.3): Lightweight VM runtime for macOS
  - Used by Colima to run Linux VMs
  - Source: https://github.com/lima-vm/lima

### Linux
- **k3s** (v1.27.16): Lightweight Kubernetes distribution
  - Single binary, no external dependencies
  - Runs as user process (no sudo required)
  - Source: https://k3s.io

### Shared
- **kubectl** (v1.27.16): Kubernetes CLI
  - Used by API server to interact with cluster
  - Platform-specific binaries for macOS (Intel + ARM) and Linux (x64 + ARM)
  - Source: https://kubernetes.io

## Usage

### During Development

**Download binaries manually** (required for local development):

```bash
# From vibespace root
cd app/src-tauri/binaries

# macOS
./macos/download.sh

# Linux
./linux/download.sh

# kubectl (auto-detects platform)
./download-kubectl.sh
```

### During Build

**Binaries are downloaded automatically** by `build.rs` if missing. See `app/src-tauri/build.rs`.

The build script:
1. Checks if binaries exist
2. If missing, runs download scripts
3. Verifies checksums (future enhancement)
4. Copies to Tauri bundle directory

### In Production

**Binaries are extracted at runtime**:
1. User installs vibespace app
2. User clicks "Install Kubernetes" in setup wizard
3. App extracts bundled binaries to `~/.vibespace/bin/`
4. App runs installation:
   - **macOS**: `colima start --kubernetes --cpu 2 --memory 4`
   - **Linux**: `k3s server --write-kubeconfig-mode 644 --disable traefik`
5. App waits for cluster to be healthy
6. App installs components (Knative, Traefik, Registry, BuildKit)

## Security

### Checksums (Future Enhancement)

Currently, binaries are downloaded over HTTPS without checksum verification. Future improvement:

```
checksums.txt:
colima-darwin-amd64: sha256:abc123...
colima-darwin-arm64: sha256:def456...
lima-darwin-amd64: sha256:ghi789...
...
```

Verify during build with:
```bash
shasum -a 256 -c checksums.txt
```

### Code Signing (macOS)

Binaries must be **code-signed** for macOS Gatekeeper:

```bash
# Sign each binary
codesign --sign "Developer ID Application: Your Name" colima
codesign --sign "Developer ID Application: Your Name" lima
codesign --sign "Developer ID Application: Your Name" kubectl-darwin-amd64

# Verify
codesign --verify --verbose colima
```

**Requirements**:
- Apple Developer account ($99/year)
- Developer ID Application certificate
- Notarization for macOS 10.15+

## Updating Binaries

### Process

1. **Update version numbers** in download scripts:
   ```bash
   # macos/download.sh
   COLIMA_VERSION="0.9.0"  # Change here

   # linux/download.sh
   K3S_VERSION="v1.28.0+k3s1"  # Change here

   # download-kubectl.sh
   KUBECTL_VERSION="v1.28.0"  # Change here
   ```

2. **Delete old binaries**:
   ```bash
   rm -f macos/colima macos/lima
   rm -f linux/k3s
   rm -f kubectl-*
   ```

3. **Download new binaries**:
   ```bash
   ./macos/download.sh
   ./linux/download.sh
   ./download-kubectl.sh
   ```

4. **Test locally**:
   ```bash
   # macOS
   ./macos/colima version

   # Linux
   ./linux/k3s --version

   # kubectl
   ./kubectl-darwin-amd64 version --client
   ```

5. **Update checksums** (when implemented)

6. **Commit version changes** (not binaries):
   ```bash
   git add */download.sh download-kubectl.sh README.md
   git commit -m "chore: update bundled k8s binaries to vX.Y.Z"
   ```

## .gitignore

**Binaries are NOT committed to git** (too large). They are:
- Downloaded during development via scripts
- Downloaded during CI/CD builds
- Bundled in release artifacts (DMG, DEB, RPM)

See `app/src-tauri/binaries/.gitignore`:
```
# Binaries (downloaded by scripts, not committed)
colima
lima
limactl
k3s
kubectl-*
*.tar.gz
*.zip
```

## Troubleshooting

### Download Fails

**Error**: `curl: (56) OpenSSL SSL_read: error:0A000126`

**Solution**: GitHub rate limiting or network issue. Retry with:
```bash
# Use token for higher rate limit
GITHUB_TOKEN=your_token ./macos/download.sh
```

### Binary Won't Execute

**Error**: `Permission denied`

**Solution**: Ensure executable bit is set:
```bash
chmod +x macos/colima
chmod +x linux/k3s
```

### macOS Gatekeeper Blocks

**Error**: "Cannot be opened because the developer cannot be verified"

**Solution**: Code signing required. See "Code Signing (macOS)" section above.

### Wrong Architecture

**Error**: `cannot execute binary file: Exec format error`

**Solution**: Download script auto-detects architecture. If incorrect:
```bash
# Check your architecture
uname -m

# macOS: x86_64 or arm64
# Linux: x86_64, aarch64, or armv7l
```

## Development Notes

- **Binary size**: ~150MB total (acceptable for desktop app)
- **Download time**: ~2-3 minutes on slow connections
- **Startup time**: ~60 seconds (macOS), ~30 seconds (Linux)
- **Resource usage**: 2 CPU, 4GB RAM, 10GB disk (default)

## References

- **ADR 0006**: [Bundle Kubernetes Runtime](../../../docs/adr/0006-bundled-kubernetes-runtime.md)
- **Colima**: https://github.com/abiosoft/colima
- **Lima**: https://github.com/lima-vm/lima
- **k3s**: https://k3s.io
- **kubectl**: https://kubernetes.io/docs/tasks/tools/

## License

Bundled binaries retain their original licenses:
- Colima: MIT License
- Lima: Apache License 2.0
- k3s: Apache License 2.0
- kubectl: Apache License 2.0
