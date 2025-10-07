FROM ubuntu:22.04

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

# Install code-server
RUN curl -fsSL https://code-server.dev/install.sh | sh -s -- --version=4.20.0

# Create non-root user
RUN useradd -m -s /bin/bash -u 1000 coder && \
    mkdir -p /home/coder/.local/share/code-server && \
    chown -R coder:coder /home/coder

USER coder
WORKDIR /workspace

# Pre-configure code-server
RUN mkdir -p ~/.config/code-server && \
    echo "bind-addr: 0.0.0.0:8080" > ~/.config/code-server/config.yaml && \
    echo "auth: none" >> ~/.config/code-server/config.yaml && \
    echo "cert: false" >> ~/.config/code-server/config.yaml

EXPOSE 8080

CMD ["code-server", "/workspace"]
