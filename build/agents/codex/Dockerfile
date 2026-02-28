# vibespace Codex Agent
# AI coding agent powered by OpenAI's Codex CLI
#
# Extends the vibespace base image with:
# - Codex CLI (@openai/codex)
# - Codex-specific settings

ARG REGISTRY=ghcr.io/vibespacehq/vibespace
ARG VERSION=latest

FROM ${REGISTRY}/base:${VERSION}

# Install Codex CLI globally
RUN npm install -g @openai/codex

# Create Codex config directory
RUN mkdir -p /home/user/.codex && \
    chown -R user:user /home/user/.codex

# Copy Codex configuration (file-based credential storage for shared home)
COPY config.toml /etc/vibespace/codex/config.toml

# Wrapper script to log session starts (shadows npm-global/bin/codex via PATH priority)
COPY codex-wrapper.sh /home/user/.local/bin/codex
RUN chmod +x /home/user/.local/bin/codex && \
    chown user:user /home/user/.local/bin/codex

# Set agent type environment variable
ENV VIBESPACE_AGENT_TYPE=codex
