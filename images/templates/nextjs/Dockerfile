FROM localhost:5000/workspace-base:latest

USER root

# Install Node.js 20 LTS
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Install pnpm
RUN npm install -g pnpm

USER coder

# Pre-install VS Code extensions
RUN code-server --install-extension dbaeumer.vscode-eslint && \
    code-server --install-extension esbenp.prettier-vscode && \
    code-server --install-extension bradlc.vscode-tailwindcss && \
    code-server --install-extension formulahendry.auto-rename-tag && \
    code-server --install-extension csstools.postcss

# Create init script to scaffold on first run
RUN echo '#!/bin/bash\n\
if [ ! -f /workspace/.initialized ]; then\n\
  echo "Initializing Next.js workspace..."\n\
  cd /workspace\n\
  pnpm create next-app@latest . --typescript --tailwind --app --no-src-dir --import-alias "@/*"\n\
  pnpm install\n\
  touch /workspace/.initialized\n\
fi\n\
exec code-server /workspace' > /home/coder/init.sh && \
    chmod +x /home/coder/init.sh

CMD ["/home/coder/init.sh"]
