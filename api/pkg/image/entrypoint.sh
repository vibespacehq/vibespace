#!/bin/bash
# vibespace container entrypoint
# Sets up environment, credentials, and starts supervisor

set -e

log() { echo "[vibespace] $*"; }

# ============================================================================
# Directory setup
# ============================================================================
mkdir -p /var/log/supervisor
chown -R user:user /vibespace 2>/dev/null || true

# ============================================================================
# Credential sharing (--share-credentials mode)
# ============================================================================
if [ "$VIBESPACE_SHARE_CREDENTIALS" = "true" ]; then
    log "Credential sharing enabled"

    SHARED_HOME="/vibespace/.home"

    # Create shared home if it doesn't exist (first agent)
    if [ ! -d "$SHARED_HOME" ]; then
        log "Creating shared home directory"
        mkdir -p "$SHARED_HOME"
        cp -a /home/user/. "$SHARED_HOME/" 2>/dev/null || true
    fi

    # Handle SSH authorized_keys
    mkdir -p "$SHARED_HOME/.ssh"
    chmod 700 "$SHARED_HOME/.ssh"
    if [ -n "$AUTHORIZED_KEYS" ]; then
        echo "$AUTHORIZED_KEYS" >> "$SHARED_HOME/.ssh/authorized_keys"
        sort -u "$SHARED_HOME/.ssh/authorized_keys" -o "$SHARED_HOME/.ssh/authorized_keys" 2>/dev/null || true
        chmod 600 "$SHARED_HOME/.ssh/authorized_keys"
    fi

    chown -R user:user "$SHARED_HOME"

    # Update /etc/passwd to use shared home
    sed -i "s|/home/user|$SHARED_HOME|g" /etc/passwd
else
    # Non-shared mode: set up SSH in regular home
    if [ -n "$AUTHORIZED_KEYS" ]; then
        mkdir -p /home/user/.ssh
        echo "$AUTHORIZED_KEYS" > /home/user/.ssh/authorized_keys
        chmod 700 /home/user/.ssh
        chmod 600 /home/user/.ssh/authorized_keys
        chown -R user:user /home/user/.ssh
    fi
fi

# ============================================================================
# Git config
# ============================================================================
if [ ! -f /home/user/.gitconfig ]; then
    cat > /home/user/.gitconfig <<EOF
[user]
    name = Claude (vibespace)
    email = claude@vibespace.local
[init]
    defaultBranch = main
[safe]
    directory = /vibespace
EOF
    chown user:user /home/user/.gitconfig
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
EOF
chmod 644 /etc/profile.d/vibespace.sh

# Create .profile to source .bashrc for SSH login shells
if [ ! -f /home/user/.profile ]; then
    cat > /home/user/.profile <<'PROFILE'
if [ -n "$BASH_VERSION" ] && [ -f "$HOME/.bashrc" ]; then
    . "$HOME/.bashrc"
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
# Startup
# ============================================================================
log "Starting (name=${VIBESPACE_NAME:-?}, agent=${VIBESPACE_AGENT:-?}, shared=${VIBESPACE_SHARE_CREDENTIALS:-false})"

exec "$@"
