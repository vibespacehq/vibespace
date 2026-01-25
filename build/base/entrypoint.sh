#!/bin/bash
# vibespace container entrypoint
set -e

log() { echo "[vibespace] $*"; }

mkdir -p /var/log/supervisor
chown -R user:user /vibespace 2>/dev/null || true

# Determine home directory (may change if credential sharing enabled)
USER_HOME="/home/user"

# ============================================================================
# Credential sharing (--share-credentials mode)
# ============================================================================
if [ "$VIBESPACE_SHARE_CREDENTIALS" = "true" ]; then
    log "Credential sharing enabled"

    USER_HOME="/vibespace/.home"

    # Create shared home if it doesn't exist (first agent)
    if [ ! -d "$USER_HOME" ]; then
        log "Creating shared home directory"
        mkdir -p "$USER_HOME"
        cp -a /home/user/. "$USER_HOME/" 2>/dev/null || true
    fi

    # Handle SSH authorized_keys
    mkdir -p "$USER_HOME/.ssh"
    chmod 700 "$USER_HOME/.ssh"
    if [ -n "$AUTHORIZED_KEYS" ]; then
        echo "$AUTHORIZED_KEYS" >> "$USER_HOME/.ssh/authorized_keys"
        sort -u "$USER_HOME/.ssh/authorized_keys" -o "$USER_HOME/.ssh/authorized_keys" 2>/dev/null || true
        chmod 600 "$USER_HOME/.ssh/authorized_keys"
    fi

    chown -R user:user "$USER_HOME"

    # Update /etc/passwd to use shared home
    sed -i "s|/home/user|$USER_HOME|g" /etc/passwd
else
    # Non-shared mode: set up SSH in regular home
    if [ -n "$AUTHORIZED_KEYS" ]; then
        mkdir -p "$USER_HOME/.ssh"
        echo "$AUTHORIZED_KEYS" > "$USER_HOME/.ssh/authorized_keys"
        chmod 700 "$USER_HOME/.ssh"
        chmod 600 "$USER_HOME/.ssh/authorized_keys"
        chown -R user:user "$USER_HOME/.ssh"
    fi
fi

# ============================================================================
# Git config
# ============================================================================
if [ ! -f "$USER_HOME/.gitconfig" ]; then
    cat > "$USER_HOME/.gitconfig" <<EOF
[user]
    name = Claude (vibespace)
    email = claude@vibespace.local
[init]
    defaultBranch = main
[safe]
    directory = /vibespace
EOF
    chown user:user "$USER_HOME/.gitconfig"
fi

# ============================================================================
# Shell configuration
# ============================================================================
cat > /etc/profile.d/vibespace.sh <<EOF
export VIBESPACE_ID="${VIBESPACE_ID}"
export VIBESPACE_NAME="${VIBESPACE_NAME}"
export VIBESPACE_AGENT="${VIBESPACE_AGENT}"
export VIBESPACE_CLAUDE_ID="${VIBESPACE_CLAUDE_ID}"
export VIBESPACE_SHARE_CREDENTIALS="${VIBESPACE_SHARE_CREDENTIALS:-false}"
export VIBESPACE_SKIP_PERMISSIONS="${VIBESPACE_SKIP_PERMISSIONS:-false}"
export VIBESPACE_ALLOWED_TOOLS="${VIBESPACE_ALLOWED_TOOLS:-}"
export VIBESPACE_DISALLOWED_TOOLS="${VIBESPACE_DISALLOWED_TOOLS:-}"
EOF
chmod 644 /etc/profile.d/vibespace.sh

# Create .profile to source .bashrc for SSH login shells
if [ ! -f "$USER_HOME/.profile" ]; then
    cat > "$USER_HOME/.profile" <<'PROFILE'
if [ -n "$BASH_VERSION" ] && [ -f "$HOME/.bashrc" ]; then
    . "$HOME/.bashrc"
fi
PROFILE
    chown user:user "$USER_HOME/.profile"
fi

# Append vibespace config to bashrc if not already present
if ! grep -q "vibespace shell configuration" "$USER_HOME/.bashrc" 2>/dev/null; then
    cat "$USER_HOME/.bashrc.vibespace" >> "$USER_HOME/.bashrc"
fi
chown user:user "$USER_HOME/.bashrc"

# ============================================================================
# Claude Code settings (permission hooks)
# ============================================================================
CLAUDE_CONFIG_DIR="$USER_HOME/.claude"
mkdir -p "$CLAUDE_CONFIG_DIR"
# Always update settings to ensure PreToolUse hooks are configured
# User customizations should go in project-level .claude/settings.json
cp /etc/vibespace/claude-settings.json "$CLAUDE_CONFIG_DIR/settings.json"
chown -R user:user "$CLAUDE_CONFIG_DIR"
log "Claude Code permission hooks configured"

# ============================================================================
# Startup
# ============================================================================
log "Starting (name=${VIBESPACE_NAME:-?}, agent=${VIBESPACE_AGENT:-?}, shared=${VIBESPACE_SHARE_CREDENTIALS:-false})"

# Export environment variables for supervisord (with defaults for optional ones)
export USER_HOME
export VIBESPACE_SHARE_CREDENTIALS="${VIBESPACE_SHARE_CREDENTIALS:-false}"

exec "$@"
