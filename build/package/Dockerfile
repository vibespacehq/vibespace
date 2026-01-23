# vibespace Container Image
# AI-powered development environment with Claude Code
#
# Features:
# - SSH (port 22): Primary terminal access
# - ttyd (port 7681): Web terminal fallback
# - Claude Code CLI: AI coding agent
# - Modern dev tools: ripgrep, fzf, jq, etc.
# - Passwordless sudo for installing packages
#
# Environment variables (set by Kubernetes):
# - VIBESPACE_ID: Vibespace identifier
# - VIBESPACE_NAME: Vibespace name
# - VIBESPACE_AGENT: Agent name (e.g., claude-1 or custom name)
# - VIBESPACE_CLAUDE_ID: Claude instance ID (1, 2, 3, ...)
# - AUTHORIZED_KEYS: SSH public key for authentication (from secret)
# - VIBESPACE_SHARE_CREDENTIALS: Enable credential sharing across agents

FROM ubuntu:22.04

ARG TARGETARCH

ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/user \
    LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8 \
    LANGUAGE=en_US:en \
    TERM=xterm-256color \
    PATH="/home/user/.local/bin:/home/user/.npm-global/bin:${PATH}"

# ============================================================================
# System setup: locale, sudo, SSH
# ============================================================================
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Locale
    locales \
    # Sudo for package installation
    sudo \
    # SSH server
    openssh-server openssh-client \
    # Process manager
    supervisor \
    && rm -rf /var/lib/apt/lists/* \
    # Generate locale
    && sed -i '/en_US.UTF-8/s/^# //g' /etc/locale.gen \
    && locale-gen en_US.UTF-8 \
    && update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 \
    # Configure sshd
    && mkdir -p /run/sshd \
    && sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config \
    && sed -i 's/#PubkeyAuthentication yes/PubkeyAuthentication yes/' /etc/ssh/sshd_config \
    && echo "AllowUsers user" >> /etc/ssh/sshd_config

# ============================================================================
# Development tools
# ============================================================================
RUN apt-get update && apt-get install -y --no-install-recommends \
    # Core tools
    curl wget git vim nano less \
    # Build essentials
    build-essential ca-certificates pkg-config \
    # Modern CLI tools
    ripgrep fzf jq tree htop tmux \
    # Network tools
    net-tools procps dnsutils iputils-ping \
    # Archive tools
    zip unzip tar gzip \
    # Python
    python3 python3-pip python3-venv \
    # Node.js
    nodejs npm \
    && rm -rf /var/lib/apt/lists/*

# Upgrade Node.js to v20 LTS via n
RUN npm install -g n && n lts && hash -r

# Install ttyd for web terminal (browser access fallback)
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

# ============================================================================
# User setup
# ============================================================================
RUN useradd -m -s /bin/bash -u 1000 user && \
    # Add to sudo group with NOPASSWD
    usermod -aG sudo user && \
    echo "user ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/user && \
    chmod 0440 /etc/sudoers.d/user && \
    # Create directories
    mkdir -p /vibespace && \
    chown user:user /vibespace && \
    mkdir -p /home/user/.npm-global \
             /home/user/.config/claude-code \
             /home/user/.ssh \
             /home/user/.local/bin && \
    chmod 700 /home/user/.ssh && \
    chown -R user:user /home/user

# Suppress default Ubuntu MOTD
RUN rm -f /etc/update-motd.d/* && \
    touch /home/user/.hushlogin && \
    chown user:user /home/user/.hushlogin

# Copy configuration files
COPY supervisord.conf /etc/supervisor/conf.d/vibespace.conf
COPY bashrc.vibespace /home/user/.bashrc.vibespace
COPY banner.txt /home/user/.banner.txt
COPY entrypoint.sh /entrypoint.sh

RUN mkdir -p /var/log/supervisor && \
    chown 1000:1000 /var/log/supervisor && \
    chmod 755 /var/log/supervisor && \
    chmod +x /entrypoint.sh && \
    chown user:user /home/user/.bashrc.vibespace

# Switch to user for npm config
USER user
WORKDIR /vibespace

RUN npm config set prefix /home/user/.npm-global

# Back to root for supervisor
USER root

EXPOSE 22 7681

ENTRYPOINT ["/entrypoint.sh"]
CMD ["supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
