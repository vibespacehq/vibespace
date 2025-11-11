#!/bin/bash
# Next.js Production Server
# Runs: pnpm build && pnpm start on VIBESPACE_PROD_PORT (default: 3001)

set -e

# Use environment variable or default port
PORT=${VIBESPACE_PROD_PORT:-3001}

echo "[Prod] Starting Next.js production server on port $PORT..."

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

# Build the Next.js app if .next doesn't exist or is outdated
if [ ! -d ".next" ] || [ ".next" -ot "app" ] || [ ".next" -ot "pages" ]; then
    echo "[Prod] Building Next.js application..."
    pnpm build
fi

# Start production server
echo "[Prod] Starting Next.js production server..."
exec pnpm start --port "$PORT"
