# vibespace Container Image
# Single minimal image: ttyd + Claude Code + port detector
#
# Components:
# - ttyd: Web terminal (port 7681)
# - Claude Code CLI: AI coding agent
# - port-detector: Monitors listening ports, publishes to NATS
# - nats-bridge: Routes messages between Claude CLI and NATS
#
# Environment variables (set by Knative):
# - VIBESPACE_PROJECT: Project identifier
# - VIBESPACE_CLAUDE_ID: Claude instance ID
# - NATS_URL: NATS server URL (nats://nats.default.svc.cluster.local:4222)
# - ANTHROPIC_API_KEY: Claude API key (from Kubernetes Secret)

FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive \
    HOME=/home/user \
    PATH="/home/user/.local/bin:/home/user/.npm-global/bin:${PATH}"

# Install system dependencies
RUN apt-get update && apt-get install -y \
    # Core tools
    curl wget git vim nano less \
    # Build essentials for npm packages
    build-essential ca-certificates openssh-client \
    # Node.js (LTS)
    nodejs npm \
    # Python (for misc tooling)
    python3 python3-pip \
    # Process manager
    supervisor \
    # Network tools for debugging
    net-tools procps \
    && rm -rf /var/lib/apt/lists/*

# Upgrade Node.js to v20 LTS via n
RUN npm install -g n && n lts && hash -r

# Install ttyd for web terminal
# Detect architecture and download appropriate binary
ARG TARGETARCH
RUN case "${TARGETARCH:-amd64}" in \
        amd64) TTYD_ARCH="x86_64" ;; \
        arm64) TTYD_ARCH="aarch64" ;; \
        *) echo "Unsupported architecture" && exit 1 ;; \
    esac && \
    curl -L "https://github.com/tsl0922/ttyd/releases/download/1.7.4/ttyd.${TTYD_ARCH}" \
        -o /usr/bin/ttyd && \
    chmod +x /usr/bin/ttyd

# Install Claude Code CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user (UID 1000 matches typical host user)
RUN useradd -m -s /bin/bash -u 1000 user && \
    mkdir -p /vibespace && \
    chown user:user /vibespace && \
    # Create npm global directory and config (as root, then chown)
    mkdir -p /home/user/.npm-global /home/user/.config/claude-code && \
    chown -R user:user /home/user

# Copy port detector and nats-bridge binaries (built separately)
COPY --chown=root:root port-detector /usr/bin/port-detector
COPY --chown=root:root nats-bridge /usr/bin/nats-bridge
RUN chmod +x /usr/bin/port-detector /usr/bin/nats-bridge

# Copy supervisor configuration
COPY supervisord.conf /etc/supervisor/conf.d/vibespace.conf

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Switch to non-root user
USER user
WORKDIR /vibespace

# Configure npm global directory and Claude Code
RUN npm config set prefix /home/user/.npm-global && \
    echo '{"autoApprove": true, "telemetry": false}' > /home/user/.config/claude-code/config.json

# Expose ttyd port
EXPOSE 7681

# Switch back to root for supervisor
USER root

ENTRYPOINT ["/entrypoint.sh"]
CMD ["supervisord", "-n", "-c", "/etc/supervisor/supervisord.conf"]
