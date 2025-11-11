#!/bin/bash
# Vibespace Multi-Process Entrypoint
# Runs code-server, preview server, and production server simultaneously via supervisord

set -e

echo "=== Vibespace Multi-Process Container ==="
echo ""

# Set default port values if not provided by Knative
export VIBESPACE_CODE_PORT=${VIBESPACE_CODE_PORT:-8080}
export VIBESPACE_PREVIEW_PORT=${VIBESPACE_PREVIEW_PORT:-3000}
export VIBESPACE_PROD_PORT=${VIBESPACE_PROD_PORT:-3001}

echo "Port Configuration:"
echo "  Code Server: $VIBESPACE_CODE_PORT"
echo "  Preview Server: $VIBESPACE_PREVIEW_PORT"
echo "  Production Server: $VIBESPACE_PROD_PORT"
echo ""

# Ensure scripts directory exists
mkdir -p /home/workspace/scripts

# Copy preview and prod scripts if they exist in the template
if [ -f /opt/vibespace/preview.sh ]; then
    cp /opt/vibespace/preview.sh /home/workspace/scripts/preview.sh
    chmod +x /home/workspace/scripts/preview.sh
    echo "✓ Preview script installed"
fi

if [ -f /opt/vibespace/prod.sh ]; then
    cp /opt/vibespace/prod.sh /home/workspace/scripts/prod.sh
    chmod +x /home/workspace/scripts/prod.sh
    echo "✓ Production script installed"
fi

# Copy supervisord config
if [ -f /opt/vibespace/supervisord.conf ]; then
    cp /opt/vibespace/supervisord.conf /home/workspace/supervisord.conf
    echo "✓ Supervisord config installed"
fi

echo ""

# Run vibespace initialization if it exists
if [ -f /home/workspace/init-workspace.sh ]; then
    echo "Running vibespace initialization..."
    bash /home/workspace/init-workspace.sh
    echo "✓ Vibespace initialized"
    echo ""
fi

# Display environment info
echo "Vibespace Environment:"
echo "  Node.js: $(node --version 2>/dev/null || echo 'not installed')"
echo "  npm: $(npm --version 2>/dev/null || echo 'not installed')"
echo "  code-server: $(code-server --version 2>/dev/null | head -1 || echo 'not installed')"

if [ -n "$ANTHROPIC_API_KEY" ]; then
    echo "  ✓ Claude Code CLI available"
fi

if [ -n "$OPENAI_API_KEY" ]; then
    echo "  ✓ OpenAI API available"
fi

echo ""
echo "Starting services via supervisord..."
echo ""

# Start supervisord in foreground
exec /usr/bin/supervisord -c /home/workspace/supervisord.conf
