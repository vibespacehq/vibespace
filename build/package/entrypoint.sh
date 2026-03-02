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
# GitHub repository clone
# ============================================================================
if [ -n "$VIBESPACE_GITHUB_REPO" ] && [ -n "$GITHUB_ACCESS_TOKEN" ]; then
    # Configure git credentials (shared by both modes)
    log "Configuring git credentials"
    CRED_FILE="$USER_HOME/.git-credentials-vibespace"
    REPO_HOST=$(echo "$VIBESPACE_GITHUB_REPO" | sed -n 's|https://\([^/]*\).*|\1|p')
    echo "https://x-access-token:${GITHUB_ACCESS_TOKEN}@${REPO_HOST}" > "$CRED_FILE"
    chmod 600 "$CRED_FILE"
    chown user:user "$CRED_FILE"
    su -s /bin/bash user -c "git config --global credential.helper 'store --file=$CRED_FILE'"

    if [ "$VIBESPACE_GIT_WORKTREE" = "true" ]; then
        # ── Worktree mode ──
        AGENT_NAME="${VIBESPACE_AGENT:-agent}"
        BARE_DIR="/vibespace/.bare-repo"
        BRANCH="${VIBESPACE_GIT_BRANCH:-$AGENT_NAME}"
        WORKTREE_DIR="/vibespace/worktrees/$AGENT_NAME"

        # Bare clone (first agent only, idempotent)
        if [ ! -f "$BARE_DIR/HEAD" ]; then
            log "Creating bare clone of $VIBESPACE_GITHUB_REPO"
            su -s /bin/bash user -c "git clone --bare '$VIBESPACE_GITHUB_REPO' '$BARE_DIR'"
            su -s /bin/bash user -c "git -C '$BARE_DIR' config remote.origin.fetch '+refs/heads/*:refs/remotes/origin/*'"
        fi

        # Create worktree (per-agent, idempotent)
        if [ -f "$BARE_DIR/HEAD" ] && [ ! -e "$WORKTREE_DIR/.git" ]; then
            log "Creating worktree at $WORKTREE_DIR (branch: $BRANCH)"
            mkdir -p /vibespace/worktrees
            DEFAULT_BRANCH=$(su -s /bin/bash user -c "git -C '$BARE_DIR' symbolic-ref HEAD" | sed 's|refs/heads/||')
            su -s /bin/bash user -c "git -C '$BARE_DIR' worktree add -b '$BRANCH' '$WORKTREE_DIR' 'refs/heads/${DEFAULT_BRANCH:-main}'"
        fi

        # Safe directories
        su -s /bin/bash user -c "git config --global --add safe.directory '$WORKTREE_DIR'"
        su -s /bin/bash user -c "git config --global --add safe.directory '$BARE_DIR'"

        export VIBESPACE_WORKDIR="$WORKTREE_DIR"
    else
        # ── Normal clone mode (existing behavior) ──
        REPO_NAME=$(basename "$VIBESPACE_GITHUB_REPO" .git)
        CLONE_DIR="/vibespace/$REPO_NAME"

        if [ ! -d "$CLONE_DIR/.git" ]; then
            log "Cloning $VIBESPACE_GITHUB_REPO into $CLONE_DIR"
            if su -s /bin/bash user -c "git clone '$VIBESPACE_GITHUB_REPO' '$CLONE_DIR'"; then
                log "Clone successful"
                su -s /bin/bash user -c "git config --global --add safe.directory '$CLONE_DIR'"
            else
                log "ERROR: Clone failed"
            fi
        else
            log "Repository already present at $CLONE_DIR"
        fi
        export VIBESPACE_WORKDIR="$CLONE_DIR"
    fi
fi

# Default workdir fallback
export VIBESPACE_WORKDIR="${VIBESPACE_WORKDIR:-/vibespace}"

# ============================================================================
# GitHub token refresh
# ============================================================================
if [ -n "$GITHUB_REFRESH_TOKEN" ] && [ -n "$VIBESPACE_GITHUB_CLIENT_ID" ]; then
    REFRESH_SCRIPT="/usr/local/bin/vibespace-git-refresh.sh"
    cat > "$REFRESH_SCRIPT" <<'RSCRIPT'
#!/bin/bash
# Refresh GitHub OAuth token every 7 hours (token lifetime: 8 hours)
INTERVAL=25200
sleep "$INTERVAL"
while true; do
    RESP=$(curl -sS -X POST "https://github.com/login/oauth/access_token" \
        -H "Accept: application/json" \
        -d "client_id=$VIBESPACE_GITHUB_CLIENT_ID" \
        -d "grant_type=refresh_token" \
        -d "refresh_token=$GITHUB_REFRESH_TOKEN")

    NEW_AT=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('access_token',''))" 2>/dev/null)
    NEW_RT=$(echo "$RESP" | python3 -c "import sys,json; print(json.load(sys.stdin).get('refresh_token',''))" 2>/dev/null)

    if [ -n "$NEW_AT" ] && [ -n "$NEW_RT" ]; then
        export GITHUB_ACCESS_TOKEN="$NEW_AT"
        export GITHUB_REFRESH_TOKEN="$NEW_RT"
        REPO_HOST=$(echo "$VIBESPACE_GITHUB_REPO" | sed -n 's|https://\([^/]*\).*|\1|p')
        echo "https://x-access-token:${NEW_AT}@${REPO_HOST}" > "$HOME/.git-credentials-vibespace"
        echo "[github-refresh] $(date +%H:%M) Token refreshed"
    else
        echo "[github-refresh] $(date +%H:%M) Refresh failed: $RESP" >&2
    fi
    sleep "$INTERVAL"
done
RSCRIPT
    chmod +x "$REFRESH_SCRIPT"
    nohup "$REFRESH_SCRIPT" >> /var/log/github-token-refresh.log 2>&1 &
    log "GitHub token refresh daemon started (PID $!)"
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
export VIBESPACE_GIT_WORKTREE="${VIBESPACE_GIT_WORKTREE:-}"
export VIBESPACE_WORKDIR="${VIBESPACE_WORKDIR:-/vibespace}"
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
# Claude Code settings
# ============================================================================
CLAUDE_CONFIG_DIR="$USER_HOME/.claude"
mkdir -p "$CLAUDE_CONFIG_DIR"
# Note: permission hook settings are injected per-session by the TUI
# into project-level /vibespace/.claude/settings.json, not global settings.
# This ensures direct SSH sessions aren't blocked by the hook.
chown -R user:user "$CLAUDE_CONFIG_DIR"

# ============================================================================
# Startup
# ============================================================================
log "Starting (name=${VIBESPACE_NAME:-?}, agent=${VIBESPACE_AGENT:-?}, shared=${VIBESPACE_SHARE_CREDENTIALS:-false})"

# Export environment variables for supervisord (with defaults for optional ones)
export USER_HOME
export VIBESPACE_SHARE_CREDENTIALS="${VIBESPACE_SHARE_CREDENTIALS:-false}"
export VIBESPACE_GIT_WORKTREE="${VIBESPACE_GIT_WORKTREE:-}"
export VIBESPACE_WORKDIR="${VIBESPACE_WORKDIR:-/vibespace}"

exec "$@"
