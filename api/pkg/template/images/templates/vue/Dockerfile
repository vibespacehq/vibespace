ARG AGENT=claude
FROM registry.default.svc.cluster.local:5000/vibespace-base-${AGENT}:latest

USER root

# Install pnpm 10.18.3
RUN npm install -g pnpm@10.18.3

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

# Create init script - installs Vue 3.5.22 + Vite 7.1.10
RUN echo '#!/bin/bash\n\
if [ ! -f /workspace/.initialized ]; then\n\
  echo "Initializing Vue 3 workspace..."\n\
  cd /workspace\n\
  # Create Vite + Vue app with latest versions\n\
  pnpm create vite@latest . --template vue-ts\n\
  pnpm install\n\
  echo ""\n\
  echo "Installed versions:"\n\
  echo "  Vue: $(pnpm list vue --depth=0 | grep -oP \"vue@\\K[^\\s]+\")"\n\
  echo "  Vite: $(pnpm list vite --depth=0 | grep -oP \"vite@\\K[^\\s]+\")"\n\
  echo "  TypeScript: $(pnpm list typescript --depth=0 | grep -oP \"typescript@\\K[^\\s]+\")"\n\
  # Start dev server in background\n\
  echo "Starting Vite dev server..."\n\
  pnpm dev &>/dev/null &\n\
  echo "  ✓ Dev server running on port 5173"\n\
  echo ""\n\
  touch /workspace/.initialized\n\
fi\n\
~/init-agents.sh\n\
exec code-server /workspace' > /home/workspace/init.sh && \
    chmod +x /home/workspace/init.sh

EXPOSE 8080 5173

CMD ["/home/workspace/init.sh"]
