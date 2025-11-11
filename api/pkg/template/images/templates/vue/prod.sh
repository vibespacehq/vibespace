#!/bin/bash
# Vue Production Server
# Runs: pnpm build && pnpm preview on VIBESPACE_PROD_PORT (default: 3001)

set -e

# Use environment variable or default port
PORT=${VIBESPACE_PROD_PORT:-3001}

echo "[Prod] Starting Vue production server on port $PORT..."

# Wait for vibespace to be initialized
while [ ! -f /workspace/.initialized ]; do
    echo "[Prod] Waiting for vibespace initialization..."
    sleep 2
done

cd /workspace

# Check if package.json exists
if [ ! -f "package.json" ]; then
    echo "[Prod] Error: package.json not found"
    sleep infinity
    exit 1
fi

# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
    echo "[Prod] Installing dependencies..."
    pnpm install
fi

# Build the Vue app if dist doesn't exist or is outdated
if [ ! -d "dist" ] || [ "dist" -ot "src" ]; then
    echo "[Prod] Building Vue application..."
    pnpm build
fi

# Start production preview server
echo "[Prod] Starting Vue production server (Vite preview)..."
exec pnpm preview --port "$PORT" --host 0.0.0.0
