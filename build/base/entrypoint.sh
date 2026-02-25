#!/bin/bash
# vibespace container entrypoint
set -e

log() { echo "[vibespace] $*"; }

mkdir -p /var/log/supervisor
chown -R user:user /vibespace 2>/dev/null || true

# Determine home directory (may change if credential sharing enabled)
USER_HOME="/home/user"

# ============================================================================
# Credential persistence
# ============================================================================
# Agent name is required for persistent storage
AGENT_NAME="${VIBESPACE_AGENT:-agent}"

if [ "$VIBESPACE_SHARE_CREDENTIALS" = "true" ]; then
    # Shared mode: all agents share credentials via common home directory
    log "Credential sharing enabled"
    USER_HOME="/vibespace/.home"
else
    # Isolated mode: each agent has its own persistent home on PVC
    # Credentials persist across pod restarts but are not shared between agents
    log "Isolated credentials mode"
    USER_HOME="/vibespace/.agents/$AGENT_NAME"
fi

# Create persistent home if it doesn't exist
if [ ! -d "$USER_HOME" ]; then
    log "Creating persistent home at $USER_HOME"
    mkdir -p "$USER_HOME"
    cp -a /home/user/. "$USER_HOME/" 2>/dev/null || true
fi

# Handle SSH authorized_keys
mkdir -p "$USER_HOME/.ssh"
chmod 700 "$USER_HOME/.ssh"
if [ -n "$AUTHORIZED_KEYS" ]; then
    if [ "$VIBESPACE_SHARE_CREDENTIALS" = "true" ]; then
        # Shared mode: append and deduplicate
        echo "$AUTHORIZED_KEYS" >> "$USER_HOME/.ssh/authorized_keys"
        sort -u "$USER_HOME/.ssh/authorized_keys" -o "$USER_HOME/.ssh/authorized_keys" 2>/dev/null || true
    else
        # Isolated mode: overwrite
        echo "$AUTHORIZED_KEYS" > "$USER_HOME/.ssh/authorized_keys"
    fi
    chmod 600 "$USER_HOME/.ssh/authorized_keys"
fi

chown -R user:user "$USER_HOME"

# Update /etc/passwd to use persistent home
sed -i "s|/home/user|$USER_HOME|g" /etc/passwd

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
export PATH="/home/user/.local/bin:/home/user/.npm-global/bin:\$PATH"
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
# Agent-specific settings
# ============================================================================
case "$VIBESPACE_AGENT_TYPE" in
    claude-code)
        CLAUDE_CONFIG_DIR="$USER_HOME/.claude"
        mkdir -p "$CLAUDE_CONFIG_DIR"
        # Note: permission hook settings are injected per-session by the TUI
        # into project-level /vibespace/.claude/settings.json, not global settings.
        # This ensures direct SSH sessions aren't blocked by the hook.
        chown -R user:user "$CLAUDE_CONFIG_DIR"
        ;;
    codex)
        CODEX_CONFIG_DIR="$USER_HOME/.codex"
        mkdir -p "$CODEX_CONFIG_DIR"
        # Copy Codex config (file-based credential storage, tool settings, etc.)
        cp /etc/vibespace/codex/config.toml "$CODEX_CONFIG_DIR/config.toml"
        chown -R user:user "$CODEX_CONFIG_DIR"
        log "Codex configuration applied"
        ;;
    *)
        log "Unknown agent type: ${VIBESPACE_AGENT_TYPE:-unset}, skipping agent setup"
        ;;
esac

# ============================================================================
# Startup
# ============================================================================
log "Starting (name=${VIBESPACE_NAME:-?}, agent=${VIBESPACE_AGENT:-?}, shared=${VIBESPACE_SHARE_CREDENTIALS:-false})"

# Export environment variables for supervisord (with defaults for optional ones)
export USER_HOME
export VIBESPACE_SHARE_CREDENTIALS="${VIBESPACE_SHARE_CREDENTIALS:-false}"

exec "$@"
