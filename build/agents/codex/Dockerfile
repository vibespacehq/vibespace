# vibespace Codex Agent
# AI coding agent powered by OpenAI's Codex CLI
#
# Extends the vibespace base image with:
# - Codex CLI (@openai/codex)
# - Codex-specific settings

ARG REGISTRY=ghcr.io/yagizdagabak/vibespace
ARG VERSION=latest

FROM ${REGISTRY}/base:${VERSION}

# Install Codex CLI globally
RUN npm install -g @openai/codex

# Create Codex config directory
RUN mkdir -p /home/user/.codex && \
    chown -R user:user /home/user/.codex

# Copy Codex-specific configuration (if any)
# COPY codex-settings.toml /etc/vibespace/codex-settings.toml

# Set agent type environment variable
ENV VIBESPACE_AGENT_TYPE=codex
