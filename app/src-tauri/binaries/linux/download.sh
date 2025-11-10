#!/bin/bash
# Download k3s binary for Linux bundling
# Run during Tauri build process if binaries are missing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Versions
K3S_VERSION="v1.27.16+k3s1"

# Architecture detection
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    K3S_ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    K3S_ARCH="arm64"
elif [ "$ARCH" = "armv7l" ]; then
    K3S_ARCH="armhf"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

echo "Downloading binaries for Linux ($ARCH)..."

# Download k3s
K3S_URL="https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s"
if [ "$K3S_ARCH" != "amd64" ]; then
    K3S_URL="https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s-${K3S_ARCH}"
fi
K3S_FILE="k3s"

if [ ! -f "$K3S_FILE" ]; then
    echo "Downloading k3s ${K3S_VERSION} for ${K3S_ARCH}..."
    curl -L "$K3S_URL" -o "$K3S_FILE"
    chmod +x "$K3S_FILE"
    echo "✓ k3s downloaded"
else
    echo "✓ k3s already exists"
fi

# Verify binary
echo ""
echo "Verifying binary..."
./"$K3S_FILE" --version || echo "Warning: k3s version check failed"

echo ""
echo "✓ All Linux binaries downloaded successfully"
echo "  - k3s: $(du -h k3s | cut -f1)"
