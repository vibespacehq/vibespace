#!/usr/bin/env bash
set -euo pipefail

# Get version from git tags (e.g., v0.1.0, v0.1.0-3-g1234567, or dev)
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
COMMIT=${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}
BUILD_DATE=${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}

# Package path for ldflags
PKG="github.com/vibespacehq/vibespace/internal/cli"

echo "Building vibespace..."
echo "  Version:    ${VERSION}"
echo "  Commit:     ${COMMIT}"
echo "  Build Date: ${BUILD_DATE}"

go build \
  -ldflags "-X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.BuildDate=${BUILD_DATE}" \
  -o vibespace \
  ./cmd/vibespace

echo "Built: ./vibespace"
