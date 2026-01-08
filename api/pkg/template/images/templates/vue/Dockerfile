ARG AGENT=claude
ARG BASE_REGISTRY=ghcr.io/yagizdagabak/vibespace
FROM ${BASE_REGISTRY}/vibespace-base-${AGENT}:latest

USER root

# Install supervisord for multi-process management
RUN apt-get update && apt-get install -y supervisor && rm -rf /var/lib/apt/lists/*

# Install pnpm 10.18.3
RUN npm install -g pnpm@10.18.3

# Create vibespace scripts directory
RUN mkdir -p /opt/vibespace

# Copy multi-process scripts
COPY supervisord.conf /opt/vibespace/supervisord.conf
COPY entrypoint.sh /opt/vibespace/entrypoint.sh
COPY preview.sh /opt/vibespace/preview.sh
COPY prod.sh /opt/vibespace/prod.sh
RUN chmod +x /opt/vibespace/*.sh

USER workspace

# Install Vue VS Code extensions (non-fatal - continue even if some extensions fail)
RUN (code-server --install-extension Vue.volar || true) && \
    (code-server --install-extension Vue.vscode-typescript-vue-plugin || true) && \
    (code-server --install-extension dbaeumer.vscode-eslint || true) && \
    (code-server --install-extension esbenp.prettier-vscode || true)

# Copy Vue-specific agent instructions
# Copy to all agent directories (only the relevant one will exist based on base image)
COPY CLAUDE.md /home/workspace/VUE-GUIDE.md
RUN mkdir -p ~/.claude ~/.openai ~/.gemini && \
    cp /home/workspace/VUE-GUIDE.md ~/.claude/VUE-GUIDE.md 2>/dev/null || true && \
    cp /home/workspace/VUE-GUIDE.md ~/.openai/VUE-GUIDE.md 2>/dev/null || true && \
    cp /home/workspace/VUE-GUIDE.md ~/.gemini/VUE-GUIDE.md 2>/dev/null || true

# Create vibespace initialization script - installs Vue 3.5.22 + Vite 7.1.10
RUN echo '#!/bin/bash\n\
if [ ! -f /workspace/.initialized ]; then\n\
  echo "Initializing Vue 3 vibespace..."\n\
  cd /workspace\n\
  # Create Vite + Vue app with latest versions\n\
  pnpm create vite@latest . --template vue-ts\n\
  pnpm install\n\
  echo ""\n\
  echo "Installed versions:"\n\
  echo "  Vue: $(pnpm list vue --depth=0 | grep -oP \"vue@\\K[^\\s]+\")"\n\
  echo "  Vite: $(pnpm list vite --depth=0 | grep -oP \"vite@\\K[^\\s]+\")"\n\
  echo "  TypeScript: $(pnpm list typescript --depth=0 | grep -oP \"typescript@\\K[^\\s]+\")"\n\
  echo ""\n\
  touch /workspace/.initialized\n\
fi' > /home/workspace/init-workspace.sh && \
    chmod +x /home/workspace/init-workspace.sh

# Expose only port 8080 (Caddy proxy - single Knative port)
# Internal services run on 8081 (code-server), 3000 (preview), 3001 (prod)
# Caddy routes based on Host header
EXPOSE 8080

# Use multi-process entrypoint
CMD ["/bin/bash", "/opt/vibespace/entrypoint.sh"]
