#!/bin/bash
# Download mkcert binary for TLS certificate generation
# Run during Tauri build process if binary is missing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Version
MKCERT_VERSION="v1.4.4"

# Platform and architecture detection (can be overridden by env vars)
if [ -z "$MKCERT_OS" ]; then
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    if [ "$OS" = "darwin" ]; then
        MKCERT_OS="darwin"
    elif [ "$OS" = "linux" ]; then
        MKCERT_OS="linux"
    else
        echo "Unsupported OS: $OS"
        exit 1
    fi
fi

if [ -z "$MKCERT_ARCH" ]; then
    ARCH=$(uname -m)
    if [ "$ARCH" = "x86_64" ]; then
        MKCERT_ARCH="amd64"
    elif [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
        MKCERT_ARCH="arm64"
    else
        echo "Unsupported architecture: $ARCH"
        exit 1
    fi
fi

echo "Downloading mkcert for $MKCERT_OS/$MKCERT_ARCH..."

# Download mkcert
MKCERT_URL="https://github.com/FiloSottile/mkcert/releases/download/${MKCERT_VERSION}/mkcert-${MKCERT_VERSION}-${MKCERT_OS}-${MKCERT_ARCH}"
MKCERT_FILE="mkcert-${MKCERT_OS}-${MKCERT_ARCH}"

if [ ! -f "$MKCERT_FILE" ]; then
    echo "Downloading mkcert ${MKCERT_VERSION}..."
    curl -sL "$MKCERT_URL" -o "$MKCERT_FILE"
    chmod +x "$MKCERT_FILE"
    echo "✓ mkcert downloaded"
else
    echo "✓ mkcert already exists"
fi

# Verify binary
echo ""
echo "Verifying binary..."
./"$MKCERT_FILE" -version || echo "Warning: mkcert version check failed"

echo ""
echo "✓ mkcert downloaded successfully"
echo "  - mkcert: $(du -h $MKCERT_FILE | cut -f1)"
