# workspaces

Desktop app for managing containerized development environments in k3s.

## What is workspaces?

workspaces is a Tauri desktop app that manages isolated dev environments running in local k3s. Each workspace runs code-server (VS Code in browser) and supports AI coding agents like Claude Code and OpenAI Codex.

### Key Features

- 🚀 **Isolated Environments**: Spin up project-specific workspaces with custom configurations
- 🤖 **AI-Ready**: Pre-configured for coding agents with seamless authentication
- 💻 **Local-First**: All workspaces run on local k3s cluster, no cloud dependency
- 📦 **Template-Based**: Quick start with Next.js, Vue, Jupyter, or custom templates
- 🔄 **Scale-to-Zero**: Workspaces auto-stop when idle to save resources
- 🌐 **Local DNS**: Access workspaces via `workspace-{id}.local`

## Quick Start

### Prerequisites

**System Requirements**:
- macOS, Linux, or Windows (via WSL2)
- 8GB+ RAM
- 50GB+ disk space

**Kubernetes Required**:

workspaces needs Kubernetes to run. Choose one option:

#### Option 1: Rancher Desktop (Recommended) ⭐

**Easiest for beginners** - GUI-based k3s management.

1. Download from [rancherdesktop.io](https://rancherdesktop.io/)
2. Install and launch Rancher Desktop
3. Enable Kubernetes in settings
4. Done! ✅

#### Option 2: k3d (k3s in Docker)

**Lightweight k3s cluster in Docker containers**.

**macOS/Linux/Windows**:
```bash
# Install k3d
brew install k3d  # macOS/Linux with Homebrew
# or download from https://k3d.io/

# Create a cluster
k3d cluster create mycluster
```

#### Option 3: Native k3s (Linux only)

**For Linux users who prefer native installation**.

```bash
curl -sfL https://get.k3s.io | sh -s - \
  --write-kubeconfig-mode 644 \
  --disable traefik
```

#### Option 4: Existing Cluster

If you already have a Kubernetes cluster (k3d, minikube, Docker Desktop), workspaces will detect it.

### Installation

```bash
# Clone the repository
git clone https://github.com/yagizdagabak/workspaces
cd workspaces

# Install dependencies and run the desktop app
cd app
npm install
npm run tauri:dev
```

The app will detect your Kubernetes installation on startup. If Kubernetes is not found, the app will show installation instructions.

### Usage

1. Launch workspaces app
2. Click "New workspace"
3. Choose a template (Next.js, Vue, Jupyter)
4. Configure resources and AI agents
5. Click "Create"
6. Open in embedded VS Code or browser

### Troubleshooting Installation

**Issue**: "Kubernetes not detected" after installing k3s

**Solution**: Ensure k3s is running and kubeconfig is accessible
```bash
# Check if k3s is running (Linux)
sudo systemctl status k3s

# Start k3s if stopped (Linux)
sudo systemctl start k3s

# macOS (Rancher Desktop): Ensure Kubernetes is enabled in settings
# macOS (native k3s): Check if k3s process is running
ps aux | grep k3s
```

**Issue**: "Permission denied" when accessing kubeconfig

**Solution**: Fix kubeconfig permissions
```bash
# For native k3s
sudo chmod 644 /etc/rancher/k3s/k3s.yaml

# Or copy to user directory
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown $USER:$USER ~/.kube/config
chmod 600 ~/.kube/config
```

**Issue**: kubectl not found in PATH

**Solution**: Install kubectl or ensure it's in your PATH
```bash
# macOS
brew install kubectl

# Linux
curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl

# Verify
kubectl version --client
```

**Issue**: Rancher Desktop installed but not detected

**Solution**: Ensure Kubernetes is enabled in Rancher Desktop settings
1. Open Rancher Desktop
2. Go to Preferences → Kubernetes
3. Check "Enable Kubernetes"
4. Wait for cluster to start (green indicator)
5. Click "Verify installation" in workspaces app

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

- [Product Roadmap](docs/ROADMAP.md) - Feature timeline and release strategy
- [Technical Specification](docs/SPEC.md) - Complete architecture and design
- [Architecture Decisions](docs/adr/) - Records of key architectural decisions
- [Contributing Guide](docs/CONTRIBUTING.md) - Development workflow
- [AI Assistant Context](.claude/CLAUDE.md) - For AI code assistants
- [API Documentation](api/README.md) - API server guide

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

**Current Phase**: Phase 1 - MVP (Week 2 of 3)

- [x] **Phase 1**: MVP - Foundation
  - [x] Tauri app scaffold
  - [x] Functional tests
  - [x] Go API server
  - [ ] Kubernetes detection & setup guide
  - [ ] Basic workspace management
- [ ] **Phase 2**: Polish & Integration
  - [ ] Rancher Desktop integration
  - [ ] More templates (Vue, Go, Jupyter)
  - [ ] Knative scale-to-zero
- [ ] **Phase 3**: Zero-Config Installation
  - [ ] Bundled Kubernetes runtime
  - [ ] One-click setup experience
- [ ] **Phase 4**: Cloud Mode
  - [ ] AWS/GCP/DigitalOcean support
  - [ ] TLS & custom domains
- [ ] **Phase 5**: Enterprise Features
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
- [Knative](https://knative.dev/) - Serverless on Kubernetes
- [code-server](https://github.com/coder/code-server) - VS Code in browser
- [Tauri](https://tauri.app/) - Desktop app framework
- [BuildKit](https://github.com/moby/buildkit) - Container image builder

---

**Status**: MVP Development | **License**: MIT | **Made with**: ☕ + 🤖
