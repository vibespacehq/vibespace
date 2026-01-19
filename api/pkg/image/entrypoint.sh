#!/bin/bash
# vibespace container entrypoint
# Sets up environment and starts supervisor

set -e

# Ensure log directory exists
mkdir -p /var/log/supervisor

# Ensure /vibespace is writable
chown -R user:user /vibespace 2>/dev/null || true

# Set up Claude Code configuration if ANTHROPIC_API_KEY is set
if [ -n "$ANTHROPIC_API_KEY" ]; then
    su - user -c "mkdir -p /home/user/.config/claude-code"
    cat > /home/user/.config/claude-code/config.json <<EOF
{
  "autoApprove": true,
  "telemetry": false,
  "apiKey": "$ANTHROPIC_API_KEY"
}
EOF
    chown user:user /home/user/.config/claude-code/config.json
fi

# Create .bashrc additions
cat >> /home/user/.bashrc <<'EOF'

# vibespace environment
export PS1='\[\e[36m\]vibespace\[\e[0m\]:\[\e[33m\]\w\[\e[0m\]$ '
alias ll='ls -la'
alias la='ls -a'

# Welcome message
echo ""
echo "  Welcome to vibespace"
echo "  --------------------"
echo "  Project: ${VIBESPACE_PROJECT:-unknown}"
echo "  Claude ID: ${VIBESPACE_CLAUDE_ID:-1}"
echo ""
echo "  Commands:"
echo "    claude        - Start Claude Code AI assistant"
echo "    claude /login - Authenticate with your Anthropic account"
echo ""
EOF

# Log startup info
echo "vibespace container starting..."
echo "  VIBESPACE_ID: ${VIBESPACE_ID:-not set}"
echo "  VIBESPACE_PROJECT: ${VIBESPACE_PROJECT:-not set}"
echo "  VIBESPACE_CLAUDE_ID: ${VIBESPACE_CLAUDE_ID:-not set}"

exec "$@"
