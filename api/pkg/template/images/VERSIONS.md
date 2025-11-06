# Template Image Versions

**Build Date**: October 16, 2025
**Node.js LTS Transition**: Node.js 24.x enters LTS in October 2025
**Multi-Agent Architecture**: Each template supports multiple AI coding agents

## Image Architecture

### Multi-Agent Support (MVP Phase 1)

All templates support three AI coding agents:
- **Claude** (Anthropic Claude Code CLI)
- **Codex** (OpenAI Codex CLI)
- **Gemini** (Google Gemini SDK)

Users select their preferred agent during vibespace creation. The agent is baked into the container image.

### Image Naming Convention

```
Base images:     localhost:5000/vibespace-base-{agent}:latest
Template images: localhost:5000/vibespace-{template}-{agent}:latest

Examples:
- localhost:5000/vibespace-base-claude:latest
- localhost:5000/vibespace-base-codex:latest
- localhost:5000/vibespace-base-gemini:latest
- localhost:5000/vibespace-nextjs-claude:latest
- localhost:5000/vibespace-vue-codex:latest
- localhost:5000/vibespace-jupyter-gemini:latest
```

### Total Images Built

- **3 base images** (one per agent)
- **9 template images** (3 templates × 3 agents)
- **12 total images** built during cluster setup

---

## Base Images

### vibespace-base-claude:latest
- **Ubuntu**: 24.04 LTS
- **code-server**: 4.104.3+ (VS Code 1.104.0+)
- **Node.js**: 24.x (entering LTS October 2025)
- **npm**: 11.6.2
- **pnpm**: 10.18.3
- **AI Agent**: Claude Code CLI (@anthropic-ai/claude-code)
- **API Key**: ANTHROPIC_API_KEY (injected via Kubernetes Secret)

### vibespace-base-codex:latest
- **Ubuntu**: 24.04 LTS
- **code-server**: 4.104.3+ (VS Code 1.104.0+)
- **Node.js**: 24.x (entering LTS October 2025)
- **npm**: 11.6.2
- **pnpm**: 10.18.3
- **AI Agent**: OpenAI CLI (openai-cli)
- **API Key**: OPENAI_API_KEY (injected via Kubernetes Secret)

### vibespace-base-gemini:latest
- **Ubuntu**: 24.04 LTS
- **code-server**: 4.104.3+ (VS Code 1.104.0+)
- **Node.js**: 24.x (entering LTS October 2025)
- **npm**: 11.6.2
- **pnpm**: 10.18.3
- **Python**: 3.12+ (for Gemini SDK)
- **AI Agent**: Google Gemini SDK (google-generativeai)
- **API Key**: GOOGLE_API_KEY (injected via Kubernetes Secret)

---

## Template Images

### Next.js Templates (3 variants)
- **vibespace-nextjs-claude:latest**
- **vibespace-nextjs-codex:latest**
- **vibespace-nextjs-gemini:latest**

**Versions**:
- **Next.js**: 15.5.5 (stable)
- **React**: 19.x
- **TypeScript**: 5.9.3
- **Tailwind CSS**: 4.1.14
- **Turbopack**: Stable (default bundler)
- **pnpm**: 10.18.3

**Ports**:
- code-server: 8080
- dev server: 3000

### Vue Templates (3 variants)
- **vibespace-vue-claude:latest**
- **vibespace-vue-codex:latest**
- **vibespace-vue-gemini:latest**

**Versions**:
- **Vue**: 3.5.22
- **Vite**: 7.1.10 (requires Node 20.19+ or 22.12+)
- **TypeScript**: 5.9.3
- **pnpm**: 10.18.3

**Ports**:
- code-server: 8080
- dev server: 5173

### Jupyter Templates (3 variants)
- **vibespace-jupyter-claude:latest**
- **vibespace-jupyter-codex:latest**
- **vibespace-jupyter-gemini:latest**

**Versions**:
- **Python**: 3.12.3 (Ubuntu 24.04 default)
- **Jupyter Lab**: 4.4.9
- **NumPy**: latest stable
- **Pandas**: latest stable
- **Matplotlib**: latest stable
- **Seaborn**: latest stable
- **Scikit-learn**: latest stable
- **SciPy**: latest stable
- **Plotly**: latest stable

**Ports**:
- code-server: 8080
- Jupyter Lab: 8888

---

## Build Process

Images are built automatically during cluster setup using BuildKit:

1. **Build all base images** (3 images: claude, codex, gemini)
2. **Build all template images** (9 images: 3 templates × 3 agents)
3. **Push to local registry** (localhost:5000)

Build order:
```
1. vibespace-base-claude:latest
2. vibespace-base-codex:latest
3. vibespace-base-gemini:latest
4. vibespace-nextjs-claude:latest
5. vibespace-nextjs-codex:latest
6. vibespace-nextjs-gemini:latest
7. vibespace-vue-claude:latest
8. vibespace-vue-codex:latest
9. vibespace-vue-gemini:latest
10. vibespace-jupyter-claude:latest
11. vibespace-jupyter-codex:latest
12. vibespace-jupyter-gemini:latest
```

Total build time: ~5-10 minutes (depending on system)

---

## Update Strategy

Images are rebuilt during cluster setup and pull exact pinned versions above.

**To update to newer versions**:
1. Modify Dockerfiles in `images/base-*/` or `images/templates/*/`
2. Update version numbers in this file
3. Trigger rebuild via cluster setup process

**Version pinning philosophy**:
- All versions are pinned and documented for reproducibility
- Latest stable versions as of October 2025
- Compatibility verified across all combinations

---

## Version Compatibility Notes

- **Vite 7**: Requires Node.js 20.19+, 22.12+, or 24.x
- **Next.js 15.5**: Works with Node.js 18.18+, optimized for 20.x/22.x/24.x
- **Python 3.12**: Ubuntu 24.04 default (3.12.3)
- **Tailwind CSS 4.1**: New mask utilities and text-shadow support
- **All agents**: Compatible with code-server 4.104.3+ and Ubuntu 24.04

---

## Agent Selection

Users choose their AI agent during vibespace creation:

**Frontend (vibespace creation form)**:
```json
{
  "name": "my-vibespace",
  "template": "nextjs",
  "agent": "claude"
}
```

**Backend (image selection)**:
```
Image: localhost:5000/vibespace-nextjs-claude:latest
Env: ANTHROPIC_API_KEY (from credential store)
```

**Default**: If no agent specified, defaults to `claude`

---

## References

- [Node.js Releases](https://nodejs.org/en/about/previous-releases)
- [Next.js Releases](https://github.com/vercel/next.js/releases)
- [Vue Releases](https://github.com/vuejs/core/releases)
- [Vite Releases](https://github.com/vitejs/vite/releases)
- [Python Releases](https://www.python.org/downloads/)
- [Jupyter Lab Releases](https://github.com/jupyterlab/jupyterlab/releases)
- [Claude Code CLI](https://www.npmjs.com/package/@anthropic-ai/claude-code)
- [OpenAI CLI](https://www.npmjs.com/package/openai-cli)
- [Google Gemini SDK](https://pypi.org/project/google-generativeai/)
