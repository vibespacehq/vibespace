# Workspace

Local Kubernetes workspace manager for AI-assisted development.

## What is Workspace?

Workspace is a desktop application that manages isolated development environments running locally in k3s. Each workspace includes VS Code (code-server) and can be configured with AI coding agents like Claude Code and OpenAI Codex.

### Key Features

- 🚀 **Isolated Environments**: Spin up project-specific workspaces with custom configurations
- 🤖 **AI-Ready**: Pre-configured for coding agents with seamless authentication
- 💻 **Local-First**: All workspaces run on local k3s cluster, no cloud dependency
- 📦 **Template-Based**: Quick start with Next.js, Vue, Jupyter, or custom templates
- 🔄 **Scale-to-Zero**: Workspaces auto-stop when idle to save resources
- 🌐 **Local DNS**: Access workspaces via `workspace-{id}.local`

## Quick Start

### Prerequisites

- macOS or Linux (Windows via WSL2)
- 8GB+ RAM
- 50GB+ disk space
- Docker (for building images)

### Installation

```bash
# Clone the repository
git clone https://github.com/your-org/workspace
cd workspace

# Install k3s cluster
./script/install_k3s.sh

# Build and run the desktop app
cd app
npm install
npm run dev
```

### Usage

1. Launch Workspace app
2. Click "New Workspace"
3. Choose a template (Next.js, Vue, Jupyter)
4. Configure resources and AI agents
5. Click "Create"
6. Open in embedded VS Code or browser

## Architecture

```
┌─────────────────┐
│  Desktop App    │  (Tauri + React)
└────────┬────────┘
         │ HTTP
┌────────▼────────┐
│   API Server    │  (Go)
└────────┬────────┘
         │ Kubernetes API
┌────────▼────────┐
│   k3s Cluster   │
│  ┌───────────┐  │
│  │ Workspace │  │  (Knative + code-server)
│  └───────────┘  │
└─────────────────┘
```

## Documentation

- [Technical Specification](SPEC.md) - Complete architecture and design
- [Contributing Guide](docs/CONTRIBUTING.md) - Development workflow
- [AI Assistant Context](.claude/claude.md) - For AI code assistants

## Project Structure

```
workspace/
├── app/           # Desktop application (Tauri + React)
├── api/           # API server (Go)
├── images/        # Container image Dockerfiles
├── k8s/           # Kubernetes manifests
├── script/        # Utility scripts
└── docs/          # Documentation
```

## Development

### Running Locally

**Desktop App**:
```bash
cd app
npm run dev
```

**API Server**:
```bash
cd api
go run cmd/server/main.go
```

**Build Images**:
```bash
cd images/base
docker build -t workspace-base:latest .
```

## Roadmap

- [x] **Phase 1**: Foundation (Weeks 1-2)
- [ ] **Phase 2**: Workspaces (Weeks 2-3)
- [ ] **Phase 3**: Networking (Weeks 3-4)
- [ ] **Phase 4**: AI Integration (Week 4)
- [ ] **Phase 5**: Custom Templates (Week 4-5)
- [ ] **Phase 6**: Polish & Testing (Week 5)

See [SPEC.md](SPEC.md) for detailed roadmap.

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](docs/CONTRIBUTING.md) first.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [k3s](https://k3s.io/) - Lightweight Kubernetes
- [Knative](https://knative.dev/) - Serverless on Kubernetes
- [code-server](https://github.com/coder/code-server) - VS Code in browser
- [Tauri](https://tauri.app/) - Desktop app framework
- [BuildKit](https://github.com/moby/buildkit) - Container image builder

---

**Status**: MVP Development | **License**: MIT | **Made with**: ☕ + 🤖
