#!/bin/bash
# Build portfwd for all platforms
set -e

cd "$(dirname "$0")"

echo "Building portfwd for all platforms..."

# macOS
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ../portfwd-darwin-amd64 main.go
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ../portfwd-darwin-arm64 main.go

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ../portfwd-linux-amd64 main.go
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o ../portfwd-linux-arm64 main.go

echo "Done! Binaries:"
ls -la ../portfwd-*
