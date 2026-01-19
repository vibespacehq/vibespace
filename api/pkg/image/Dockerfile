# vibespace Container Image
# Minimal image: SSH + ttyd web terminal + Claude Code CLI
#
# Components:
# - SSH: Primary terminal access (port 22) - fast, native terminal
# - ttyd: Web terminal fallback (port 7681) - browser access
# - Claude Code CLI: AI coding agent (user authenticates via /login)
#
# Environment variables (set by Knative):
# - VIBESPACE_ID: Vibespace identifier
# - VIBESPACE_PROJECT: Project name
# - VIBESPACE_CLAUDE_ID: Claude instance ID (1, 2, 3, ...)
# - AUTHORIZED_KEYS: SSH public key for authentication (from secret)

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/user \
    PATH="/home/user/.local/bin:/home/user/.npm-global/bin:${PATH}"

# Install system dependencies
RUN apt-get update && \
    apt-get install -y --no-install-recommends --fix-missing \
    # Core tools
    curl wget git vim nano less \
    # Build essentials for npm packages
    build-essential ca-certificates openssh-client \
    # SSH server for terminal access
    openssh-server \
    # Node.js (LTS)
    nodejs npm \
    # Python (for misc tooling)
    python3 python3-pip \
    # Process manager
    supervisor \
    # Network tools for debugging
    net-tools procps \
    && rm -rf /var/lib/apt/lists/* \
    # Configure sshd
    && mkdir -p /run/sshd \
    && sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config \
    && sed -i 's/#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config \
    && echo "AllowUsers user" >> /etc/ssh/sshd_config

# Upgrade Node.js to v20 LTS via n
RUN npm install -g n && n lts && hash -r

# Install ttyd for web terminal (browser access fallback)
ARG TARGETARCH
RUN TTYD_VERSION="1.7.7" && \
    case "${TARGETARCH:-amd64}" in \
        amd64) TTYD_ARCH="x86_64" ;; \
        arm64) TTYD_ARCH="aarch64" ;; \
        *) echo "Unsupported architecture" && exit 1 ;; \
    esac && \
    curl -sL "https://github.com/tsl0922/ttyd/releases/download/${TTYD_VERSION}/ttyd.${TTYD_ARCH}" -o /usr/bin/ttyd && \
    chmod +x /usr/bin/ttyd

# Install Claude Code CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user (UID 1000 matches typical host user)
RUN useradd -m -s /bin/bash -u 1000 user && \
    mkdir -p /vibespace && \
    chown user:user /vibespace && \
    # Create npm global directory and config (as root, then chown)
    mkdir -p /home/user/.npm-global /home/user/.config/claude-code && \
    # Create .ssh directory for SSH key auth
    mkdir -p /home/user/.ssh && \
    chmod 700 /home/user/.ssh && \
    chown -R user:user /home/user

# Copy supervisor configuration and create log directory (writable by user 1000)
COPY supervisord.conf /etc/supervisor/conf.d/vibespace.conf
RUN mkdir -p /var/log/supervisor && chown 1000:1000 /var/log/supervisor && chmod 755 /var/log/supervisor

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch to non-root user
USER user
WORKDIR /vibespace

# Configure npm global directory and Claude Code
RUN npm config set prefix /home/user/.npm-global && \
    echo '{"autoApprove": true, "telemetry": false}' > /home/user/.config/claude-code/config.json

# Expose SSH and ttyd ports
EXPOSE 22 7681

# Switch back to root for supervisor
USER root

ENTRYPOINT ["/entrypoint.sh"]
CMD ["supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
