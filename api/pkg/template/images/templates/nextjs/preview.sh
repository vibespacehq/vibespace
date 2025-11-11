#!/bin/bash
# Next.js Preview Server (Development Mode)
# Runs: pnpm dev on VIBESPACE_PREVIEW_PORT (default: 3000)

set -e

# Use environment variable or default port
PORT=${VIBESPACE_PREVIEW_PORT:-3000}

echo "[Preview] Starting Next.js dev server on port $PORT..."

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

# Start dev server with turbopack
echo "[Preview] Starting Next.js development server..."
exec pnpm dev --port "$PORT" --turbopack
