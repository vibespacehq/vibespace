ARG AGENT=claude
FROM registry.default.svc.cluster.local:5000/vibespace-base-${AGENT}:latest

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

# Install VS Code extensions for Next.js (non-fatal - continue even if some extensions fail)
RUN (code-server --install-extension dbaeumer.vscode-eslint || true) && \
    (code-server --install-extension esbenp.prettier-vscode || true) && \
    (code-server --install-extension bradlc.vscode-tailwindcss || true) && \
    (code-server --install-extension formulahendry.auto-rename-tag || true) && \
    (code-server --install-extension csstools.postcss || true)

# Copy Next.js-specific agent instructions
# Copy to all agent directories (only the relevant one will exist based on base image)
COPY CLAUDE.md /home/workspace/NEXTJS-GUIDE.md
RUN mkdir -p ~/.claude ~/.openai ~/.gemini && \
    cp /home/workspace/NEXTJS-GUIDE.md ~/.claude/NEXTJS-GUIDE.md 2>/dev/null || true && \
    cp /home/workspace/NEXTJS-GUIDE.md ~/.openai/NEXTJS-GUIDE.md 2>/dev/null || true && \
    cp /home/workspace/NEXTJS-GUIDE.md ~/.gemini/NEXTJS-GUIDE.md 2>/dev/null || true

# Create vibespace initialization script - installs Next.js 15.5.5
RUN echo '#!/bin/bash\n\
if [ ! -f /workspace/.initialized ]; then\n\
  echo "Initializing Next.js vibespace..."\n\
  # Create Next.js app in temp directory (create-next-app has issues with mounted volumes)\n\
  # Use lowercase directory name to avoid npm naming restrictions\n\
  TEMP_DIR="/tmp/nextjs-init-$(date +%s)"\n\
  mkdir -p "$TEMP_DIR"\n\
  cd "$TEMP_DIR"\n\
  pnpm create next-app@15.5.5 . \\\n\
    --typescript \\\n\
    --tailwind \\\n\
    --eslint \\\n\
    --app \\\n\
    --no-src-dir \\\n\
    --import-alias "@/*" \\\n\
    --turbopack \\\n\
    --use-pnpm\n\
  # Move all files (including hidden files) to /workspace\n\
  shopt -s dotglob\n\
  mv "$TEMP_DIR"/* /workspace/ 2>/dev/null || true\n\
  cd /workspace\n\
  rm -rf "$TEMP_DIR"\n\
  pnpm install\n\
  echo ""\n\
  echo "Installed versions:"\n\
  echo "  Next.js: $(pnpm list next --depth=0 | grep -oP \"next@\\K[^\\s]+\")"\n\
  echo "  React: $(pnpm list react --depth=0 | grep -oP \"react@\\K[^\\s]+\")"\n\
  echo "  TypeScript: $(pnpm list typescript --depth=0 | grep -oP \"typescript@\\K[^\\s]+\")"\n\
  echo "  Tailwind: $(pnpm list tailwindcss --depth=0 | grep -oP \"tailwindcss@\\K[^\\s]+\")"\n\
  echo ""\n\
  touch /workspace/.initialized\n\
fi' > /home/workspace/init-workspace.sh && \
    chmod +x /home/workspace/init-workspace.sh

# Expose all 3 service ports
EXPOSE 8080 3000 3001

# Use multi-process entrypoint
CMD ["/bin/bash", "/opt/vibespace/entrypoint.sh"]
