#!/bin/bash
# Vue Preview Server (Development Mode)
# Runs: pnpm dev on VIBESPACE_PREVIEW_PORT (default: 3000)

set -e

# Use environment variable or default port
PORT=${VIBESPACE_PREVIEW_PORT:-3000}

echo "[Preview] Starting Vue dev server on port $PORT..."

# Wait for vibespace to be initialized
while [ ! -f /workspace/.initialized ]; do
    echo "[Preview] Waiting for vibespace initialization..."
    sleep 2
done

cd /workspace

# Check if package.json exists
if [ ! -f "package.json" ]; then
    echo "[Preview] Error: package.json not found"
    sleep infinity
    exit 1
fi

# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
    echo "[Preview] Installing dependencies..."
    pnpm install
fi

# Start Vite dev server
echo "[Preview] Starting Vue development server (Vite)..."
exec pnpm dev --port "$PORT" --host 0.0.0.0
