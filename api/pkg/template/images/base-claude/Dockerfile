FROM ubuntu:24.04

# Prevent interactive prompts
ENV DEBIAN_FRONTEND=noninteractive

# Install system dependencies
RUN apt-get update && apt-get install -y \
    curl \
    wget \
    git \
    vim \
    nano \
    build-essential \
    ca-certificates \
    openssh-client \
    && rm -rf /var/lib/apt/lists/*

# Install code-server (latest 4.104.3+)
RUN curl -fsSL https://code-server.dev/install.sh | sh

# Install Node.js 24.x LTS (entering LTS October 2025)
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Upgrade npm to 11.6.2
RUN npm install -g npm@11.6.2

# Install Claude Code CLI (latest)
RUN npm install -g @anthropic-ai/claude-code

# Create non-root user (allow duplicate UID in case 1000 is taken)
RUN useradd -m -s /bin/bash -u 1000 -o coder && \
    mkdir -p /home/coder/.local/share/code-server && \
    mkdir -p /home/coder/.claude && \
    chown -R coder:coder /home/coder

USER coder
WORKDIR /workspace

# Pre-configure code-server
RUN mkdir -p ~/.config/code-server && \
    echo "bind-addr: 0.0.0.0:8080" > ~/.config/code-server/config.yaml && \
    echo "auth: none" >> ~/.config/code-server/config.yaml && \
    echo "cert: false" >> ~/.config/code-server/config.yaml

# Copy base CLAUDE.md
COPY CLAUDE.md /home/coder/.claude/CLAUDE.md

# Agent initialization script
RUN echo '#!/bin/bash\n\
echo "Workspace Environment:"\n\
echo "  Node.js: $(node --version)"\n\
echo "  npm: $(npm --version)"\n\
echo "  code-server: $(code-server --version | head -1)"\n\
if [ -n "$ANTHROPIC_API_KEY" ]; then\n\
  echo "  ✓ Claude Code CLI available"\n\
fi\n\
if [ -n "$OPENAI_API_KEY" ]; then\n\
  echo "  ✓ OpenAI API available"\n\
fi' > /home/coder/init-agents.sh && chmod +x /home/coder/init-agents.sh

EXPOSE 8080

CMD ["/bin/bash", "-c", "~/init-agents.sh && exec code-server /workspace"]
