#!/usr/bin/env bash
set -euo pipefail

# vibespace installer
# Usage: curl -fsSL https://raw.githubusercontent.com/vibespacehq/vibespace/main/install.sh | bash

REPO="vibespacehq/vibespace"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

info() { printf '\033[1;34m==>\033[0m %s\n' "$*"; }
error() { printf '\033[1;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)      error "Unsupported OS: $OS" ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)             error "Unsupported architecture: $ARCH" ;;
esac

# Get latest version
info "Fetching latest version..."
VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
[ -n "$VERSION" ] || error "Could not determine latest version"
info "Latest version: $VERSION"

# Download
TARBALL="vibespace-${VERSION}-${OS}-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/vibespace-${VERSION}-checksums.txt"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

info "Downloading ${TARBALL}..."
curl -fsSL -o "${TMPDIR}/${TARBALL}" "$URL" || error "Download failed. Check https://github.com/${REPO}/releases for available binaries."

# Verify checksum
info "Verifying checksum..."
curl -fsSL -o "${TMPDIR}/checksums.txt" "$CHECKSUM_URL" || error "Could not download checksums"
EXPECTED="$(grep "${TARBALL}" "${TMPDIR}/checksums.txt" | awk '{print $1}')"
if command -v sha256sum &>/dev/null; then
  ACTUAL="$(sha256sum "${TMPDIR}/${TARBALL}" | awk '{print $1}')"
elif command -v shasum &>/dev/null; then
  ACTUAL="$(shasum -a 256 "${TMPDIR}/${TARBALL}" | awk '{print $1}')"
else
  error "No sha256sum or shasum found"
fi
[ "$EXPECTED" = "$ACTUAL" ] || error "Checksum mismatch: expected $EXPECTED, got $ACTUAL"

# Extract and install
info "Installing to ${INSTALL_DIR}..."
tar xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/vibespace" "${INSTALL_DIR}/vibespace"
else
  sudo mv "${TMPDIR}/vibespace" "${INSTALL_DIR}/vibespace"
fi

chmod +x "${INSTALL_DIR}/vibespace"

info "vibespace ${VERSION} installed to ${INSTALL_DIR}/vibespace"
info "Run 'vibespace --help' to get started"
