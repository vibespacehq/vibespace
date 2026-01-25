# vibespace Claude Code Agent
# AI coding agent powered by Anthropic's Claude Code CLI
#
# Extends the vibespace base image with:
# - Claude Code CLI (@anthropic-ai/claude-code)
# - Permission hooks for TUI integration
# - Claude-specific settings

ARG REGISTRY=ghcr.io/yagizdagabak/vibespace
ARG VERSION=latest

FROM ${REGISTRY}/base:${VERSION}

# Install Claude Code CLI globally
RUN npm install -g @anthropic-ai/claude-code

# Create Claude config directory
RUN mkdir -p /home/user/.config/claude-code && \
    chown -R user:user /home/user/.config/claude-code

# Copy Claude-specific configuration
COPY vibespace-permission-hook /home/user/.local/bin/vibespace-permission-hook
COPY claude-settings.json /etc/vibespace/claude-settings.json

RUN chmod +x /home/user/.local/bin/vibespace-permission-hook && \
    chown user:user /home/user/.local/bin/vibespace-permission-hook

# Set agent type environment variable
ENV VIBESPACE_AGENT_TYPE=claude-code
