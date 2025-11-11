#!/bin/bash
# Jupyter Production Server
# Runs: jupyter lab on VIBESPACE_PROD_PORT (default: 3001)

set -e

# Use environment variable or default port
PORT=${VIBESPACE_PROD_PORT:-3001}

echo "[Prod] Starting Jupyter Lab (prod mode) on port $PORT..."

# Wait for vibespace to be initialized
while [ ! -f /workspace/.initialized ]; do
    echo "[Prod] Waiting for vibespace initialization..."
    sleep 2
done

cd /workspace

# Start Jupyter Lab in production mode (no autoreload)
echo "[Prod] Starting Jupyter Lab production server..."
exec jupyter lab \
    --ip=0.0.0.0 \
    --port="$PORT" \
    --no-browser \
    --allow-root \
    --NotebookApp.token='' \
    --NotebookApp.password='' \
    --NotebookApp.allow_origin='*' \
    --NotebookApp.disable_check_xsrf=True
