#!/bin/bash
# Download kubectl binary (shared across macOS and Linux)
# Run during Tauri build process if binary is missing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Versions
KUBECTL_VERSION="v1.27.16"

# Platform and architecture detection (can be overridden by env vars)
if [ -z "$KUBECTL_OS" ]; then
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    if [ "$OS" = "darwin" ]; then
        KUBECTL_OS="darwin"
    elif [ "$OS" = "linux" ]; then
        KUBECTL_OS="linux"
    else
        echo "Unsupported OS: $OS"
        exit 1
    fi
fi

if [ -z "$KUBECTL_ARCH" ]; then
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then
        KUBECTL_ARCH="amd64"
    elif [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
        KUBECTL_ARCH="arm64"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi
fi

echo "Downloading kubectl for $KUBECTL_OS/$KUBECTL_ARCH..."

# Download kubectl
KUBECTL_URL="https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/${KUBECTL_OS}/${KUBECTL_ARCH}/kubectl"
KUBECTL_FILE="kubectl-${KUBECTL_OS}-${KUBECTL_ARCH}"

if [ ! -f "$KUBECTL_FILE" ]; then
    echo "Downloading kubectl ${KUBECTL_VERSION}..."
    curl -L "$KUBECTL_URL" -o "$KUBECTL_FILE"
    chmod +x "$KUBECTL_FILE"
    echo "✓ kubectl downloaded"
else
    echo "✓ kubectl already exists"
fi

# Verify binary
echo ""
echo "Verifying binary..."
./"$KUBECTL_FILE" version --client || echo "Warning: kubectl version check failed"

echo ""
echo "✓ kubectl downloaded successfully"
echo "  - kubectl: $(du -h $KUBECTL_FILE | cut -f1)"
