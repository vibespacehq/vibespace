#!/bin/bash
# Download Colima and Lima binaries for macOS bundling
# Run during Tauri build process if binaries are missing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Versions
COLIMA_VERSION="0.9.1"
LIMA_VERSION="1.2.1"

# Architecture detection
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    COLIMA_ARCH="x86_64"
    LIMA_ARCH="x86_64"
elif [ "$ARCH" = "arm64" ]; then
    COLIMA_ARCH="arm64"  # Colima uses "arm64" for Darwin
    LIMA_ARCH="arm64"    # Lima uses "arm64" not "aarch64" for Darwin
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

echo "Downloading binaries for macOS ($ARCH)..."

# Download Colima
COLIMA_URL="https://github.com/abiosoft/colima/releases/download/v${COLIMA_VERSION}/colima-Darwin-${COLIMA_ARCH}"
COLIMA_FILE="colima"

if [ ! -f "$COLIMA_FILE" ] || [ ! -s "$COLIMA_FILE" ]; then
    echo "Downloading Colima ${COLIMA_VERSION}..."
    curl -L "$COLIMA_URL" -o "$COLIMA_FILE"
    if [ ! -s "$COLIMA_FILE" ]; then
        echo "Error: Colima download failed or file is empty"
        cat "$COLIMA_FILE"
        exit 1
    fi
    chmod +x "$COLIMA_FILE"
    echo "✓ Colima downloaded"
else
    echo "✓ Colima already exists"
fi

# Download Lima
LIMA_URL="https://github.com/lima-vm/lima/releases/download/v${LIMA_VERSION}/lima-${LIMA_VERSION}-Darwin-${LIMA_ARCH}.tar.gz"
LIMA_DIR="lima-dist"

if [ ! -d "$LIMA_DIR" ]; then
    echo "Downloading Lima ${LIMA_VERSION}..."
    curl -L "$LIMA_URL" -o lima.tar.gz
    # Extract full Lima distribution (includes limactl, guest agents, and support files)
    mkdir -p "$LIMA_DIR"
    tar -xzf lima.tar.gz -C "$LIMA_DIR" --strip-components=1
    rm -f lima.tar.gz
    echo "✓ Lima downloaded and extracted"
else
    echo "✓ Lima already exists"
fi

# Verify binaries
echo ""
echo "Verifying binaries..."
./"$COLIMA_FILE" version || echo "Warning: Colima version check failed"
./"$LIMA_DIR/bin/limactl" --version || echo "Warning: Lima version check failed"

echo ""
echo "✓ All macOS binaries downloaded successfully"
echo "  - colima: $(du -h colima | cut -f1)"
echo "  - lima-dist: $(du -sh lima-dist | cut -f1)"
