#!/bin/bash
# Jupyter Preview Server (Development Mode with autoreload)
# Runs: jupyter lab on VIBESPACE_PREVIEW_PORT (default: 3000)

set -e

# Use environment variable or default port
PORT=${VIBESPACE_PREVIEW_PORT:-3000}

echo "[Preview] Starting Jupyter Lab (dev mode) on port $PORT..."

# Wait for vibespace to be initialized
while [ ! -f /workspace/.initialized ]; do
    echo "[Preview] Waiting for vibespace initialization..."
    sleep 2
done

cd /workspace

# Start Jupyter Lab in development mode
echo "[Preview] Starting Jupyter Lab development server..."
exec jupyter lab \
    --ip=0.0.0.0 \
    --port="$PORT" \
    --no-browser \
    --allow-root \
    --NotebookApp.token='' \
    --NotebookApp.password='' \
    --NotebookApp.allow_origin='*' \
    --NotebookApp.disable_check_xsrf=True \
    --autoreload
