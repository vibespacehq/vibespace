#!/bin/bash
# vibespace container entrypoint
# Sets up environment, credentials, and starts supervisor

set -e

# ============================================================================
# Logging
# ============================================================================
log() { echo "[vibespace] $*"; }

# ============================================================================
# Directory setup
# ============================================================================
mkdir -p /var/log/supervisor
chown -R user:user /vibespace 2>/dev/null || true

# ============================================================================
# SSH setup
# ============================================================================
if [ -n "$AUTHORIZED_KEYS" ]; then
    log "Setting up SSH authorized_keys..."
    mkdir -p /home/user/.ssh
    echo "$AUTHORIZED_KEYS" > /home/user/.ssh/authorized_keys
    chmod 700 /home/user/.ssh
    chmod 600 /home/user/.ssh/authorized_keys
    chown -R user:user /home/user/.ssh
fi

# ============================================================================
# Credential sharing (--share-credentials mode)
# ============================================================================
SHARED_CONFIG="/vibespace/.vibespace"

if [ "$VIBESPACE_SHARE_CREDENTIALS" = "true" ]; then
    log "Credential sharing enabled, using $SHARED_CONFIG"

    # Create shared config directory structure
    mkdir -p "$SHARED_CONFIG/.claude" "$SHARED_CONFIG/.ssh"
    chown -R user:user "$SHARED_CONFIG"
    chmod 700 "$SHARED_CONFIG/.ssh"

    # Remove existing Claude config and symlink to shared location
    rm -rf /home/user/.config/claude-code
    mkdir -p /home/user/.config
    ln -sf "$SHARED_CONFIG/.claude" /home/user/.config/claude-code
    chown -h user:user /home/user/.config/claude-code

    # Symlink git config if it exists in shared location
    if [ -f "$SHARED_CONFIG/.gitconfig" ]; then
        rm -f /home/user/.gitconfig
        ln -sf "$SHARED_CONFIG/.gitconfig" /home/user/.gitconfig
        chown -h user:user /home/user/.gitconfig
    fi

    # Handle SSH keys for shared mode
    if [ -d "$SHARED_CONFIG/.ssh" ]; then
        # Merge authorized_keys if we have new ones
        if [ -n "$AUTHORIZED_KEYS" ]; then
            echo "$AUTHORIZED_KEYS" >> "$SHARED_CONFIG/.ssh/authorized_keys"
            sort -u "$SHARED_CONFIG/.ssh/authorized_keys" -o "$SHARED_CONFIG/.ssh/authorized_keys" 2>/dev/null || true
            chmod 600 "$SHARED_CONFIG/.ssh/authorized_keys"
        fi

        # Copy any existing SSH keys to shared location (first agent only)
        if [ -f /home/user/.ssh/authorized_keys ] && [ ! -f "$SHARED_CONFIG/.ssh/authorized_keys" ]; then
            cp /home/user/.ssh/authorized_keys "$SHARED_CONFIG/.ssh/"
        fi

        # Symlink SSH directory
        rm -rf /home/user/.ssh
        ln -sf "$SHARED_CONFIG/.ssh" /home/user/.ssh
        chown -h user:user /home/user/.ssh
    fi

    log "Shared config directory: $SHARED_CONFIG"
fi

# ============================================================================
# Claude Code setup
# ============================================================================
if [ -n "$ANTHROPIC_API_KEY" ]; then
    log "Setting up Claude Code with API key..."
    # Determine config location (direct or via symlink)
    CONFIG_DIR="/home/user/.config/claude-code"
    # Follow symlink if it exists
    if [ -L "$CONFIG_DIR" ]; then
        CONFIG_DIR=$(readlink -f "$CONFIG_DIR")
    fi
    mkdir -p "$CONFIG_DIR"
    cat > "$CONFIG_DIR/config.json" <<EOF
{
  "autoApprove": true,
  "telemetry": false,
  "apiKey": "$ANTHROPIC_API_KEY"
}
EOF
    chown user:user "$CONFIG_DIR/config.json"
    chmod 600 "$CONFIG_DIR/config.json"
fi

# ============================================================================
# Git config (default identity for Claude)
# ============================================================================
setup_git_config() {
    local config_path="$1"
    cat > "$config_path" <<EOF
[user]
    name = Claude (vibespace)
    email = claude@vibespace.local
[init]
    defaultBranch = main
[core]
    editor = vim
[safe]
    directory = /vibespace
EOF
    chown user:user "$config_path"
}

# Set up git config in appropriate location
if [ "$VIBESPACE_SHARE_CREDENTIALS" = "true" ]; then
    # For shared mode, create in shared location if not exists
    if [ ! -f "$SHARED_CONFIG/.gitconfig" ]; then
        setup_git_config "$SHARED_CONFIG/.gitconfig"
        rm -f /home/user/.gitconfig
        ln -sf "$SHARED_CONFIG/.gitconfig" /home/user/.gitconfig
        chown -h user:user /home/user/.gitconfig
    fi
else
    # For non-shared mode, create in home if not exists
    if [ ! -f /home/user/.gitconfig ]; then
        setup_git_config /home/user/.gitconfig
    fi
fi

# ============================================================================
# Shell configuration
# ============================================================================
# Create .profile to source .bashrc for SSH login shells
if [ ! -f /home/user/.profile ]; then
    cat > /home/user/.profile <<'PROFILE'
# Source .bashrc for interactive shells
if [ -n "$BASH_VERSION" ]; then
    if [ -f "$HOME/.bashrc" ]; then
        . "$HOME/.bashrc"
    fi
fi
PROFILE
    chown user:user /home/user/.profile
fi

# Append vibespace config to bashrc if not already present
if ! grep -q "vibespace shell configuration" /home/user/.bashrc 2>/dev/null; then
    cat /home/user/.bashrc.vibespace >> /home/user/.bashrc
fi
chown user:user /home/user/.bashrc

# ============================================================================
# Startup info
# ============================================================================
log "Container starting..."
log "  VIBESPACE_NAME: ${VIBESPACE_NAME:-not set}"
log "  VIBESPACE_AGENT: ${VIBESPACE_AGENT:-not set}"
log "  SHARE_CREDENTIALS: ${VIBESPACE_SHARE_CREDENTIALS:-false}"

exec "$@"
