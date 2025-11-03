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
    python3 \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

# Install code-server (latest 4.104.3+)
RUN curl -fsSL https://code-server.dev/install.sh | sh

# Install Node.js 24.x LTS (entering LTS October 2025)
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Upgrade npm to 11.6.2
RUN npm install -g npm@11.6.2

# Install Google Gemini CLI (Python-based)
# Using google-generativeai package (official Google Gemini SDK)
RUN pip3 install --break-system-packages google-generativeai

# Create workspace user (UID 1001 to avoid conflict with ubuntu user at 1000)
RUN useradd -m -s /bin/bash -u 1001 workspace && \
    mkdir -p /home/workspace/.local/share/code-server && \
    mkdir -p /home/workspace/.gemini && \
    chown -R workspace:workspace /home/workspace

USER workspace
WORKDIR /workspace

# Pre-configure code-server
RUN mkdir -p ~/.config/code-server && \
    echo "bind-addr: 0.0.0.0:8080" > ~/.config/code-server/config.yaml && \
    echo "auth: none" >> ~/.config/code-server/config.yaml && \
    echo "cert: false" >> ~/.config/code-server/config.yaml

# Copy base AGENT.md
COPY AGENT.md /home/workspace/.gemini/AGENT.md

# Agent initialization script
RUN echo '#!/bin/bash\n\
echo "Workspace Environment:"\n\
echo "  Node.js: $(node --version)"\n\
echo "  npm: $(npm --version)"\n\
echo "  Python: $(python3 --version)"\n\
echo "  code-server: $(code-server --version | head -1)"\n\
if [ -n "$GOOGLE_API_KEY" ]; then\n\
  echo "  ✓ Google Gemini API available"\n\
fi' > /home/workspace/init-agents.sh && chmod +x /home/workspace/init-agents.sh

EXPOSE 8080

CMD ["/bin/bash", "-c", "~/init-agents.sh && exec code-server /workspace"]
