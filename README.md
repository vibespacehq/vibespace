# vibespace

Desktop app for managing containerized development environments in k3s.

## What is vibespace?

vibespace is a Tauri desktop app that manages isolated dev environments running in k3s. Each vibespace runs code-server (VS Code in browser) and supports AI coding agents like Claude Code and OpenAI Codex.

### Key Features

- 🚀 **Isolated Environments**: Spin up project-specific vibespaces with custom configurations
- 🤖 **AI-Ready**: Pre-configured for coding agents with seamless authentication
- 💻 **Local-First**: Zero-configuration bundled Kubernetes (Colima/k3s)
- 📦 **Template-Based**: Quick start with Next.js, Vue, Jupyter, or custom templates
- 🔄 **Scale-to-Zero**: Vibespaces auto-stop when idle to save resources
- 🌐 **Local DNS**: Access vibespaces via `vibespace-{id}.local`

## Quick Start

### Prerequisites

**System Requirements**:
- macOS or Linux (Windows via WSL2)
- 8GB+ RAM
- 50GB+ disk space

**No External Dependencies!** vibespace bundles everything you need:
- Kubernetes runtime (Colima on macOS, k3s on Linux)
- kubectl CLI tool
- All required components (Knative, Traefik, Registry, BuildKit)

### Installation

```bash
# Clone the repository
git clone https://github.com/yagizdagabak/vibespace
cd vibespace

# Install dependencies and run the desktop app
cd app
npm install
npm run tauri:dev
```

The app will guide you through one-click Kubernetes installation. No external setup required!

### Usage

1. Launch vibespace app
2. Click "Install Kubernetes" (one-click setup, ~3-5 minutes)
3. Click "New vibespace"
4. Choose a template (Next.js, Vue, Jupyter)
5. Configure resources and AI agents
6. Click "Create"
7. Open in embedded VS Code or browser

## Deployment Modes

vibespace supports two deployment architectures:

### Local Mode (Current Implementation)

All components run on your machine:
- Tauri desktop app (UI)
- Go API server
- Bundled Kubernetes (Colima/k3s)
- All vibespaces run locally

**Zero-configuration**: One-click installation, works immediately. Perfect for local development.

### Remote Mode (Planned for Post-MVP)

Control plane on your machine, infrastructure on VPS:
- Tauri desktop app on your machine (UI only)
- Go API server on VPS
- Kubernetes on VPS
- Vibespaces run on VPS

**Use case**: Remote development, team collaboration, cloud resources.

See [ADR 0006](docs/adr/0006-bundled-kubernetes-runtime.md) for architectural details.

## Architecture

### Local Mode Architecture

```
┌─────────────────┐
│  Desktop App    │  (Tauri + React)
└────────┬────────┘
         │ HTTP (localhost)
┌────────▼────────┐
│   API Server    │  (Go)
└────────┬────────┘
         │ Kubernetes API
┌────────▼────────┐
│  Bundled k3s    │  (Colima on macOS, k3s on Linux)
│  ┌───────────┐  │
│  │ Vibespace │  │  (Knative + code-server)
│  └───────────┘  │
└─────────────────┘

All on same machine
```

### Remote Mode Architecture (Future)

```
User's Machine                  VPS (Cloud)
┌─────────────────┐            ┌────────────────┐
│  Desktop App    │            │  API Server    │
│  (Tauri+React)  │            │  (Go)          │
└────────┬────────┘            └────────┬───────┘
         │                              │
         │ HTTPS + WireGuard            │ K8s API
         │                              │
         └──────────────────────────────▼
                                ┌────────────────┐
                                │   k3s Cluster  │
                                │  ┌───────────┐ │
                                │  │Vibespace  │ │
                                │  └───────────┘ │
                                └────────────────┘
```

## Documentation

- [Product Roadmap](docs/ROADMAP.md) - Feature timeline and release strategy
- [Technical Specification](docs/SPEC.md) - Complete architecture and design
- [Architecture Decisions](docs/adr/) - Records of key architectural decisions
  - [ADR 0006: Bundled Kubernetes Runtime](docs/adr/0006-bundled-kubernetes-runtime.md)
- [Contributing Guide](docs/CONTRIBUTING.md) - Development workflow
- [AI Assistant Context](.claude/CLAUDE.md) - For AI code assistants
- [API Documentation](api/README.md) - API server guide

## Project Structure

```
vibespace/
├── app/           # Desktop application (Tauri + React)
│   └── src-tauri/ # Rust backend (bundled k8s manager)
├── api/           # API server (Go)
├── images/        # Container image Dockerfiles
├── k8s/           # Kubernetes manifests
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
docker build -t vibespace-base:latest .
```

## Roadmap

**Current Phase**: MVP Phase 1 - Foundation (95% complete)

- [x] **MVP Phase 1**: Foundation
  - [x] Tauri app scaffold
  - [x] Functional tests
  - [x] Go API server
  - [x] Bundled Kubernetes (Local Mode)
  - [x] Component installation (Knative, Traefik, Registry, BuildKit)
  - [x] Basic vibespace management
  - [x] Docker images with AI agents
  - [ ] Credential management (in progress)
- [ ] **MVP Phase 2**: Polish & Integration
  - [ ] End-to-end testing
  - [ ] More templates (Go, Rust, custom)
  - [ ] Performance optimization
- [ ] **Post-MVP**: Advanced Features
  - [ ] Remote Mode (VPS deployment)
  - [ ] Cloud provider integration
  - [ ] Template marketplace
  - [ ] Team collaboration
  - [ ] SSO/SAML

See [docs/ROADMAP.md](docs/ROADMAP.md) for detailed product roadmap and [docs/SPEC.md](docs/SPEC.md) for technical specifications.

## Contributing

Contributions are welcome! Please read [CONTRIBUTING.md](docs/CONTRIBUTING.md) first.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- [k3s](https://k3s.io/) - Lightweight Kubernetes
- [Colima](https://github.com/abiosoft/colima) - Container runtime for macOS
- [Knative](https://knative.dev/) - Serverless on Kubernetes
- [code-server](https://github.com/coder/code-server) - VS Code in browser
- [Tauri](https://tauri.app/) - Desktop app framework
- [BuildKit](https://github.com/moby/buildkit) - Container image builder

---

**Status**: MVP Development | **License**: MIT | **Made with**: ☕ + 🤖
