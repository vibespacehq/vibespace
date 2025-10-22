ARG AGENT=claude
FROM registry.default.svc.cluster.local:5000/workspace-base-${AGENT}:latest

USER root

# Install pnpm 10.18.3
RUN npm install -g pnpm@10.18.3

USER coder

# Install VS Code extensions for Next.js (non-fatal - continue even if some extensions fail)
RUN (code-server --install-extension dbaeumer.vscode-eslint || true) && \
    (code-server --install-extension esbenp.prettier-vscode || true) && \
    (code-server --install-extension bradlc.vscode-tailwindcss || true) && \
    (code-server --install-extension formulahendry.auto-rename-tag || true) && \
    (code-server --install-extension csstools.postcss || true)

# Copy Next.js-specific agent instructions
# Copy to all agent directories (only the relevant one will exist based on base image)
COPY CLAUDE.md /home/coder/NEXTJS-GUIDE.md
RUN mkdir -p ~/.claude ~/.openai ~/.gemini && \
    cp /home/coder/NEXTJS-GUIDE.md ~/.claude/NEXTJS-GUIDE.md 2>/dev/null || true && \
    cp /home/coder/NEXTJS-GUIDE.md ~/.openai/NEXTJS-GUIDE.md 2>/dev/null || true && \
    cp /home/coder/NEXTJS-GUIDE.md ~/.gemini/NEXTJS-GUIDE.md 2>/dev/null || true

# Create init script - installs Next.js 15.5.5
RUN echo '#!/bin/bash\n\
if [ ! -f /workspace/.initialized ]; then\n\
  echo "Initializing Next.js workspace..."\n\
  cd /workspace\n\
  # Create Next.js 15.5 app with latest stable version\n\
  pnpm create next-app@15.5.5 . \\\n\
    --typescript \\\n\
    --tailwind \\\n\
    --app \\\n\
    --no-src-dir \\\n\
    --import-alias "@/*" \\\n\
    --turbopack \\\n\
    --use-pnpm\n\
  pnpm install\n\
  echo ""\n\
  echo "Installed versions:"\n\
  echo "  Next.js: $(pnpm list next --depth=0 | grep -oP \"next@\\K[^\\s]+\")"\n\
  echo "  React: $(pnpm list react --depth=0 | grep -oP \"react@\\K[^\\s]+\")"\n\
  echo "  TypeScript: $(pnpm list typescript --depth=0 | grep -oP \"typescript@\\K[^\\s]+\")"\n\
  echo "  Tailwind: $(pnpm list tailwindcss --depth=0 | grep -oP \"tailwindcss@\\K[^\\s]+\")"\n\
  touch /workspace/.initialized\n\
fi\n\
~/init-agents.sh\n\
exec code-server /workspace' > /home/coder/init.sh && \
    chmod +x /home/coder/init.sh

EXPOSE 8080 3000

CMD ["/home/coder/init.sh"]
