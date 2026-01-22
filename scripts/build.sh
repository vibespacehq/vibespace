#!/usr/bin/env bash
set -euo pipefail

VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}

go build -ldflags "-X main.Version=${VERSION}" -o vibespace ./cmd/vibespace
