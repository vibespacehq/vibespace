# workspaces - Technical Specification

**Version:** 1.0.0
**Date:** 2025-10-07
**Status:** MVP Specification

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture](#architecture)
3. [Technical Stack](#technical-stack)
4. [Core Components](#core-components)
5. [Workspace Specifications](#workspace-specifications)
6. [API Documentation](#api-documentation)
7. [Security & Credentials](#security--credentials)
8. [Networking](#networking)
9. [Template System](#template-system)
10. [Implementation Phases](#implementation-phases)
11. [User Flows](#user-flows)
12. [Future Enhancements](#future-enhancements)

---

## 1. Project Overview

**Project Name**: `workspaces`

### 1.1 Naming Conventions

**Project Structure**:
```
workspace/                  # Root (singular)
├── app/                   # Desktop application
├── api/                   # Backend API server
├── images/                # Container images
├── k8s/                   # Kubernetes manifests
├── script/                # Utility scripts
└── docs/                  # Documentation
```

**Kubernetes Resources**:
- Namespace: `workspace` (singular)
- Labels: `workspace.dev/*`
- Resources: `workspace-{id}`, `workspace-{id}-pvc`, `workspace-{id}-secrets`

**Domains**:
- Pattern: `workspace-{id}.local`
- App ports: `workspace-{id}-3000.local`

**Go Packages**:
- Singular names: `workspace/`, `template/`, `credential/`
- Standard layout: `cmd/`, `pkg/`, `config/`, `script/`

**API Paths**:
- Collections: `/api/v1/workspaces`
- Single resource: `/api/v1/workspaces/{id}`

### 1.2 Vision

An open-source Tauri desktop app for managing isolated dev environments running in local k3s. Each workspace is a containerized environment with code-server (VS Code in browser), supports AI coding agents like Claude Code and OpenAI Codex.

### 1.3 Goals

- **Isolated Environments**: Spin up project-specific workspaces with custom configurations
- **AI-Ready**: Pre-configured for coding agents with seamless authentication
- **Local-First**: All workspaces run on local k3s cluster, no cloud dependency
- **Developer UX**: Simple desktop UI abstracting Kubernetes complexity
- **Template-Based**: Quick start with pre-built templates (Next.js, Vue, Jupyter, etc.)
- **Accessible**: VS Code available both embedded in app and via browser

### 1.4 Target Users

- Developers working on multiple projects simultaneously
- AI-assisted development practitioners
- Teams needing reproducible development environments
- Data scientists requiring isolated Jupyter environments
- Developers wanting to experiment without polluting their host system

### 1.5 Key Features (MVP)

✅ Desktop app (Tauri) with embedded VS Code
✅ Local k3s cluster management (abstracted)
✅ On-demand and persistent workspaces
✅ Scale-to-zero with Knative
✅ Local DNS (*.local)
✅ Host credential mounting (SSH, Git, AI agents)
✅ Custom template builder with BuildKit
✅ 3 built-in templates (Next.js, Vue, Jupyter)
✅ AI agent configuration (Claude Code, OpenAI Codex)
✅ Inter-workspace networking (configurable)
✅ Port forwarding to host

### 1.6 Key Features (Extended)

🎯 **Cloud Deployment Mode**
- Desktop app runs locally
- Backend API + workspaces run in cloud (AWS, GCP, DigitalOcean, etc.)
- Connect to remote workspaces via embedded code-server
- Managed k3s cluster in cloud
- Secure tunnel for workspace access

🎯 **Certificate Management**
- Automatic TLS certificate provisioning (Let's Encrypt via cert-manager)
- Per-workspace HTTPS endpoints
- Custom domain support (`myproject.example.com`)
- Wildcard certificates for subdomains
- Certificate auto-renewal

🎯 **Custom Domains**
- Map workspaces to custom domains
- DNS provider integration (Cloudflare, Route53, etc.)
- Automatic DNS record creation
- Support for multiple domains per workspace
- CNAME and A record management

---

## 2. Architecture

### 2.1 System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Desktop App (Tauri)                     │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │  Workspace List │  │  Template       │  │  Settings   │ │
│  │  + Control      │  │  Builder        │  │  Panel      │ │
│  └─────────────────┘  └─────────────────┘  └─────────────┘ │
│  ┌───────────────────────────────────────────────────────┐  │
│  │         Embedded VS Code (WebView)                    │  │
│  │         Multiple Tabs for Workspaces                  │  │
│  └───────────────────────────────────────────────────────┘  │
└──────────────────────┬──────────────────────────────────────┘
                       │ HTTP (localhost:8090)
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                    API Server (Go)                          │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │  Workspace   │  │  Template    │  │  k3s Manager    │   │
│  │  Controller  │  │  Builder     │  │                 │   │
│  └──────────────┘  └──────────────┘  └─────────────────┘   │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │  Credential  │  │  Registry    │  │  DNS Manager    │   │
│  │  Manager     │  │  Client      │  │                 │   │
│  └──────────────┘  └──────────────┘  └─────────────────┘   │
└──────────────────────┬──────────────────────────────────────┘
                       │ Kubernetes API
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                    k3s Cluster (local)                      │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐ │
│  │  Knative Serving (Workspaces)                         │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐  │ │
│  │  │ workspace-1 │  │ workspace-2 │  │ workspace-3  │  │ │
│  │  │  (Next.js)  │  │  (Vue)      │  │  (Jupyter)   │  │ │
│  │  │ code-server │  │ code-server │  │ jupyter-lab  │  │ │
│  │  │ PVC mount   │  │ PVC mount   │  │ PVC mount    │  │ │
│  │  │ Host creds  │  │ Host creds  │  │ Host creds   │  │ │
│  │  └─────────────┘  └─────────────┘  └──────────────┘  │ │
│  └───────────────────────────────────────────────────────┘ │
│                                                             │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │  Traefik    │  │  BuildKit    │  │  Local Registry  │  │
│  │  Ingress    │  │  Daemon      │  │  (registry:2)    │  │
│  │  (DNS)      │  │  (Builder)   │  │  (:5000)         │  │
│  └─────────────┘  └──────────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                       │
                       ▼
                Host Machine
         ┌───────────────────────────┐
         │  /etc/hosts               │
         │  *.local        │
         │                           │
         │  ~/.ssh (mounted)         │
         │  ~/.gitconfig (mounted)   │
         │  ~/.config/claude         │
         │  ~/.config/openai         │
         └───────────────────────────┘
```

### 2.2 Component Interaction Flow

1. **User creates workspace via Tauri UI**
2. **Tauri → Go API**: POST /api/workspaces (template, config)
3. **Go API → BuildKit**: Build image if custom template
4. **Go API → k3s**: Create Knative Service + PVC
5. **Go API → Traefik**: Configure IngressRoute
6. **Go API → DNS**: Update /etc/hosts
7. **Go API → Tauri**: Return workspace URL
8. **Tauri**: Open embedded WebView to workspace-{id}.local

---

### 2.3 Deployment Modes

#### 2.3.1 Local Mode (Default)

All components run on local machine:

```
┌─────────────────────┐
│   Desktop App       │  localhost
│   (Tauri)           │
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│   API Server        │  localhost:8090
│   (Go)              │
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│   k3s Cluster       │  local
│   - Workspaces      │
│   - BuildKit        │
│   - Registry        │
└─────────────────────┘
```

**Pros**:
- No cloud costs
- Fastest performance
- Complete offline capability
- Full data privacy

**Cons**:
- Limited by local resources
- No remote access
- Manual backup required

---

#### 2.3.2 Cloud Mode

Desktop app local, backend/workspaces in cloud:

```
┌─────────────────────┐
│   Desktop App       │  localhost
│   (Tauri)           │
└──────────┬──────────┘
           │ HTTPS (WireGuard tunnel)
           │
           ▼
┌────────────────────────────────┐
│   Cloud Provider               │
│   (AWS/GCP/DigitalOcean)       │
│                                │
│   ┌────────────────────────┐  │
│   │  API Server            │  │  public IP / domain
│   │  (Go + Load Balancer)  │  │
│   └──────────┬─────────────┘  │
│              │                 │
│   ┌──────────▼─────────────┐  │
│   │  k3s Cluster           │  │
│   │  - Workspaces          │  │
│   │  - BuildKit            │  │
│   │  - Registry            │  │
│   │  - cert-manager        │  │  (for TLS)
│   └────────────────────────┘  │
└────────────────────────────────┘
```

**Pros**:
- Access from anywhere
- Unlimited cloud resources
- Automatic backups
- Team collaboration ready

**Cons**:
- Monthly cloud costs (~$50-200/month)
- Requires internet connection
- Slightly higher latency

**Cloud Setup**:
1. User provides cloud credentials (AWS Access Key, GCP Service Account)
2. App provisions k3s cluster using Terraform/Pulumi
3. Installs Knative + Traefik + cert-manager
4. Configures WireGuard tunnel for secure access
5. Desktop app connects to `api.yourworkspace.cloud`

**Configuration**:
```yaml
# config/deployment.yaml
mode: cloud  # or "local"

cloud:
  provider: digitalocean  # aws, gcp, digitalocean
  region: nyc3
  cluster:
    nodes: 2
    node_type: s-4vcpu-8gb
  domain: workspace.yourdomain.com
  tunnel:
    type: wireguard
    port: 51820
```

---

#### 2.3.3 Hybrid Mode (Future)

Some workspaces local, some in cloud:

```
Desktop App → Local k3s (lightweight workspaces)
           └→ Cloud k3s (heavy workspaces, team sharing)
```

---

## 3. Technical Stack

### 3.1 Stack Decisions

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| **Desktop App** | Tauri + React + TypeScript | Cross-platform, native performance, smaller binary than Electron |
| **Backend API** | Go + Gin/Echo | Excellent k8s client library, single binary, fast, low resource usage |
| **Container Orchestrator** | k3s | Lightweight k8s (<512MB RAM), perfect for local, single-node setup |
| **Workload Management** | Knative Serving | Scale-to-zero for resource efficiency, easy domain routing |
| **Ingress Controller** | Traefik | Built into k3s, excellent local DNS support, simple config |
| **Container Builder** | BuildKit | Kubernetes-native, faster than Docker, better caching, no daemon |
| **Image Registry** | registry:2 | Lightweight, sufficient for local use, no external dependencies |
| **IDE** | code-server | Official VS Code in browser, full extension support, embeddable |
| **Storage** | LocalPath Provisioner | Included with k3s, dynamic PVC provisioning |

### 3.2 Language & Framework Versions

- **Go**: 1.21+
- **Node.js**: 20 LTS
- **Rust**: 1.70+ (for Tauri 2.0)
- **Tauri**: 2.x (latest stable)
- **React**: 19.x (latest stable)
- **Kubernetes**: 1.27+ (via k3s)
- **Knative Serving**: 1.11+

### 3.3 Key Dependencies

**Go Backend**:
```go
github.com/gin-gonic/gin                    // Web framework
k8s.io/client-go                            // Kubernetes client
knative.dev/serving/pkg/client              // Knative client
github.com/moby/buildkit/client             // BuildKit client
github.com/containerd/containerd            // Registry interaction
```

**Tauri Frontend**:
```json
{
  "react": "^18.2.0",
  "react-dom": "^18.2.0",
  "@tanstack/react-query": "^5.0.0",  // Server state
  "zustand": "^4.4.0",                 // Client state
  "tailwindcss": "^3.3.0",             // Styling
  "@monaco-editor/react": "^4.6.0"    // Dockerfile editor
}
```

---

## 4. Core Components

### 4.1 Desktop App

**Location**: `app/`

#### 4.1.1 Frontend Structure

**Organization**: Feature-based with separate `components/` and `styles/` subdirectories.

```
src/
├── App.tsx
├── components/
│   ├── shared/                        # Cross-feature shared components
│   │   ├── TitleBar.tsx
│   │   └── TitleBar.css
│   ├── setup/                         # Setup wizard feature
│   │   ├── components/
│   │   │   ├── AuthenticationSetup.tsx
│   │   │   ├── ConfigurationSetup.tsx
│   │   │   ├── InstallationInstructions.tsx
│   │   │   ├── KubernetesSetup.tsx
│   │   │   ├── ProgressSidebar.tsx
│   │   │   └── ReadySetup.tsx
│   │   └── styles/
│   │       ├── setup.css              # Shared feature styles
│   │       ├── AuthenticationSetup.css
│   │       ├── ConfigurationSetup.css
│   │       ├── InstallationInstructions.css
│   │       ├── ProgressSidebar.css
│   │       └── ReadySetup.css
│   ├── workspace/                     # Workspace management feature
│   │   ├── components/
│   │   │   ├── WorkspaceList.tsx    # Grid/list view of workspaces
│   │   │   ├── WorkspaceCard.tsx    # Individual workspace item
│   │   │   ├── WorkspaceCreate.tsx  # Creation wizard modal
│   │   │   ├── WorkspaceSettings.tsx # Per-workspace config
│   │   │   └── WorkspaceEmbed.tsx   # Embedded code-server iframe
│   │   └── styles/
│   │       ├── WorkspaceList.css
│   │       └── (other component styles)
│   ├── template/
│   │   ├── components/
│   │   │   ├── TemplateGallery.tsx  # Built-in + custom templates
│   │   │   ├── TemplateBuilder.tsx  # Dockerfile editor
│   │   │   ├── TemplateCard.tsx     # Template preview card
│   │   │   └── BuildLogs.tsx        # Real-time build output (SSE)
│   │   └── styles/
│   ├── cluster/
│   │   ├── components/
│   │   │   ├── ClusterStatus.tsx    # k3s health indicator
│   │   │   ├── ClusterSetup.tsx     # First-time installation wizard
│   │   │   └── ResourceMonitor.tsx  # CPU/Memory usage
│   │   └── styles/
│   ├── credentials/
│   │   ├── components/
│   │   │   ├── CredentialList.tsx   # List all stored credentials
│   │   │   ├── CredentialAdd.tsx    # Add credential modal
│   │   │   ├── AIAgentForm.tsx      # AI agent credential form
│   │   │   ├── GitConfigForm.tsx    # Git config form
│   │   │   ├── SSHKeyForm.tsx       # SSH key generation/import
│   │   │   └── CredentialCard.tsx   # Individual credential item
│   │   └── styles/
│   └── settings/
│       ├── components/
│       │   ├── CredentialSettings.tsx # Main credentials management page
│       │   ├── DNSSettings.tsx      # Domain configuration
│       │   └── GeneralSettings.tsx  # Resource defaults
│       └── styles/
├── hooks/
│   ├── useWorkspaces.ts               # React Query workspace CRUD
│   ├── useTemplates.ts                # Template management
│   ├── useCredentials.ts              # Credential CRUD operations
│   ├── useCluster.ts                  # k3s status polling
│   └── useBuildLogs.ts                # SSE connection
├── lib/
│   ├── api.ts                         # HTTP client (axios/fetch)
│   └── types.ts                       # TypeScript definitions
├── store/
│   └── appStore.ts                    # Zustand global state
└── styles/                            # Global design system
    ├── tokens.css                     # CSS variables
    ├── animations.css                 # Keyframe animations
    ├── base.css                       # Base styles & resets
    └── utilities.css                  # Reusable UI utilities
```

**Style Hierarchy**:
1. **Global** (`src/styles/`): Design tokens, animations, base resets, reusable utilities
2. **Feature-level** (`src/components/[feature]/styles/[feature].css`): Shared layout/containers within a feature
3. **Component-specific** (`src/components/[feature]/styles/[Component].css`): Styles unique to a single component

**Naming Conventions**:
- Directories: `lowercase` or `kebab-case`
- Component files: `PascalCase.tsx`
- Component-specific styles: `PascalCase.css`
- Feature-level styles: `kebab-case.css` (e.g., `setup.css`)

**Import Patterns**:
```typescript
// Component importing its own styles
import '../styles/AuthenticationSetup.css';

// Component importing feature-level shared styles
import '../styles/setup.css';
```

See `docs/adr/0003-frontend-organization.md` for the architectural decision record.

#### 4.1.2 Key Features

- **Multi-tab WebView**: Each workspace opens in a tab
- **Real-time Status**: WebSocket connection for workspace events
- **Drag-and-drop**: Import Dockerfiles to create templates
- **System Tray**: Quick access, minimizes to tray
- **Native Notifications**: Build complete, workspace ready, errors

#### 4.1.3 Design System & Theming

**Philosophy**: Nerdy but smooth - terminal-inspired with vibrant accents and modern geometric typography.

**Color Palette**:
```css
/* Pure Black Theme */
--bg-primary:     #000000;      /* Pure black background */
--bg-secondary:   #0a0a0a;      /* Slightly elevated panels */
--bg-tertiary:    #111111;      /* Hover states */
--bg-elevated:    #0f0f0f;      /* Cards, modals */

--text-primary:   #ffffff;      /* Main text (pure white) */
--text-secondary: #a0a0a0;      /* Muted text */
--text-tertiary:  #666666;      /* Disabled text */

--border:         #1a1a1a;      /* Borders, dividers */
--border-hover:   #2a2a2a;      /* Interactive borders */

/* Vibrant 4-Color Accent Palette */
--accent-primary:   #00ABAB;    /* Teal - Primary actions, links, completed states */
--accent-hover:     #00d4d4;    /* Teal hover */
--accent-secondary: #FF7D4B;    /* Orange - Recommended badges, warnings */
--accent-tertiary:  #F102F3;    /* Pink - Active states, buttons, focus */
--accent-yellow:    #F5F50A;    /* Yellow - Button hover, highlights */

/* System States */
--success:        #3fb950;      /* Running, success states */
--warning:        #d29922;      /* Warnings */
--error:          #f85149;      /* Errors, delete actions */

/* Gradients (for accents and visual interest) */
/* Primary gradient: linear-gradient(90deg, var(--accent-primary), var(--accent-tertiary)) */
/* Recommended gradient: linear-gradient(135deg, var(--accent-secondary), var(--accent-yellow)) */
```

**Typography**:
```css
/* Font Stack */
--font-mono:    'JetBrains Mono', 'Fira Code', 'Monaco', monospace;
--font-sans:    'Space Grotesk', 'DM Sans', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
--font-display: 'Space Grotesk', 'DM Sans', -apple-system, sans-serif;

/* Sizes */
--text-xs:     0.75rem;   /* 12px */
--text-sm:     0.875rem;  /* 14px */
--text-base:   1rem;      /* 16px */
--text-lg:     1.125rem;  /* 18px */
--text-xl:     1.25rem;   /* 20px */

/* Line Heights */
--leading-tight:  1.25;
--leading-normal: 1.5;
--leading-loose:  1.75;

/* Typography Notes */
/* Space Grotesk: Geometric sans-serif for UI and display text - unique, modern, not boring */
/* JetBrains Mono: Monospace for code blocks and technical content */
```

**Spacing** (Tailwind-inspired):
```css
--spacing-1:  0.25rem;   /* 4px */
--spacing-2:  0.5rem;    /* 8px */
--spacing-3:  0.75rem;   /* 12px */
--spacing-4:  1rem;      /* 16px */
--spacing-6:  1.5rem;    /* 24px */
--spacing-8:  2rem;      /* 32px */
--spacing-12: 3rem;      /* 48px */
```

**Border Radius**:
```css
--radius-sm:  0.25rem;   /* 4px - buttons, inputs */
--radius-md:  0.5rem;    /* 8px - cards */
--radius-lg:  0.75rem;   /* 12px - modals */
--radius-full: 9999px;   /* Circular */
```

**Component Patterns**:

*Workspace Card*:
```tsx
<Card>
  <Status color={running ? 'success' : 'muted'} />
  <Title>{workspace.name}</Title>
  <Metadata>
    <Icon>Template</Icon>
    <Icon>CPU</Icon>
    <Icon>Memory</Icon>
  </Metadata>
  <Actions>
    <Button variant="primary">Open</Button>
    <Button variant="ghost">Settings</Button>
  </Actions>
</Card>
```

*Button Variants*:
- `primary`: Accent color, high emphasis
- `secondary`: Secondary color, medium emphasis
- `ghost`: Transparent, low emphasis
- `danger`: Red, destructive actions

*Status Indicators*:
- 🟢 Running (green)
- ⚪ Stopped (gray)
- 🟡 Starting (yellow)
- 🔴 Failed (red)
- 🔵 Building (blue)

**Accessibility**:
- WCAG 2.1 AA compliant contrast ratios
- Focus indicators on all interactive elements
- Keyboard navigation support
- Screen reader labels

**Animation**:
```css
/* Fast, subtle animations */
--transition-fast:   150ms ease-in-out;
--transition-normal: 250ms ease-in-out;
--transition-slow:   350ms ease-in-out;
```

**Icons**: Use [Lucide](https://lucide.dev/) icon set (consistent, developer-friendly).

#### 4.1.4 Tauri Commands (Rust)

```rust
// src-tauri/src/main.rs
#[tauri::command]
async fn install_k3s() -> Result<String, String> {
    // Shell out to k3s installer
}

#[tauri::command]
async fn save_credential(cred_type: String, data: CredentialData) -> Result<String, String> {
    // Encrypt and save to app storage
    // Uses Tauri's built-in secure storage (OS keychain)
}

#[tauri::command]
async fn get_credentials() -> Result<Vec<CredentialSummary>, String> {
    // List stored credentials (without sensitive data)
}

#[tauri::command]
async fn delete_credential(id: String) -> Result<(), String> {
    // Remove from secure storage
}

#[tauri::command]
async fn update_hosts_file(entries: Vec<HostEntry>) -> Result<(), String> {
    // Requires sudo, prompt user
}

#[tauri::command]
async fn generate_ssh_key(key_type: String) -> Result<SshKeyPair, String> {
    // Generate new SSH key pair (ed25519 or rsa)
    // Store private key in secure storage
    // Return public key for display
}
```

---

### 4.2 API Server

**Location**: `api/`

#### 4.2.1 Package Structure

```
api/
├── cmd/
│   └── server/
│       └── main.go                    # Entry point
├── pkg/
│   ├── handler/
│   │   ├── workspace.go               # Workspace handlers
│   │   ├── template.go                # Template handlers
│   │   ├── credential.go              # Credential handlers
│   │   ├── cluster.go                 # Cluster handlers
│   │   └── middleware.go              # CORS, logging, auth
│   ├── k3s/
│   │   ├── detector.go                # k3s/kubectl detection
│   │   ├── client.go                  # Kubernetes client
│   │   └── health.go                  # Cluster health checks
│   ├── workspace/
│   │   ├── service.go                 # Business logic
│   │   ├── knative.go                 # Knative Service management
│   │   ├── storage.go                 # PVC management
│   │   └── lifecycle.go               # Start/stop/scale logic
│   ├── template/
│   │   ├── service.go                 # Template business logic
│   │   ├── builder.go                 # BuildKit integration
│   │   └── logs.go                    # Build log streaming (SSE)
│   ├── credential/
│   │   ├── service.go                 # Credential business logic
│   │   ├── secrets.go                 # Kubernetes Secret generation
│   │   └── types.go                   # Credential types
│   ├── registry/
│   │   ├── client.go                  # registry:2 API client
│   │   └── images.go                  # Image list/pull/push
│   ├── network/
│   │   ├── ingress.go                 # IngressRoute management
│   │   ├── dns.go                     # /etc/hosts manipulation
│   │   └── proxy.go                   # Port forwarding (if needed)
│   └── model/
│       ├── workspace.go               # Workspace model
│       ├── template.go                # Template model
│       ├── credential.go              # Credential model
│       └── config.go                  # Configuration
└── config/
    └── config.yaml                    # Default settings
```

#### 4.2.2 Configuration

```yaml
# config/config.yaml
server:
  port: 8090
  host: localhost

k3s:
  install_path: /usr/local/bin/k3s
  config_path: /etc/rancher/k3s
  kubeconfig_paths:
    - ~/.kube/config              # Rancher Desktop, k3d, kubectl default
    - /etc/rancher/k3s/k3s.yaml  # Native k3s default
    - ${KUBECONFIG}               # User-defined environment variable

registry:
  url: localhost:5000
  insecure: true

buildkit:
  address: tcp://buildkitd.default.svc.cluster.local:1234

workspace:
  namespace: workspace
  default_resources:
    cpu: "2"
    memory: "4Gi"
    storage: "10Gi"
  dns_domain: local

credential:
  storage_path: ~/.workspace/credential
  encryption: aes-256-gcm
```

---

### 4.3 Kubernetes Cluster Setup

#### 4.3.1 Installation Approach (MVP)

**Phase 1 (MVP)**: Detection + Guided Setup

The app **detects** existing Kubernetes installations and guides users to install if missing. This approach:
- ✅ Targets developer early adopters
- ✅ Supports multiple install methods (k3s, Rancher Desktop, k3d)
- ✅ Ships faster (no bundling complexity)
- ✅ More secure (no sudo from app)

**Supported Installations**:
1. **Rancher Desktop** (Recommended) - GUI-based k3s management
2. **Native k3s** (Advanced) - Command-line installation
3. **k3d** (Alternative) - k3s in Docker
4. **Existing clusters** - Any accessible Kubernetes cluster

**Detection Logic**:
```typescript
// app/src/hooks/useKubernetesStatus.ts
// TODO(#14): Implement this detection logic
async function detectKubernetes() {
  try {
    // 1. Check kubectl availability
    const kubectlAvailable = await invoke('check_kubectl');
    if (!kubectlAvailable) {
      return {
        available: false,
        error: 'kubectl not found in PATH',
        suggestedAction: 'install_kubernetes'
      };
    }

    // 2. Find kubeconfig (check multiple locations)
    const kubeconfigPath = await invoke('find_kubeconfig', {
      paths: [
        '~/.kube/config',              // Rancher Desktop, k3d
        '/etc/rancher/k3s/k3s.yaml',  // Native k3s
        process.env.KUBECONFIG         // User-defined
      ]
    });

    // 3. Check cluster connectivity
    const clusterHealthy = await invoke('check_cluster_health', { kubeconfigPath });
    if (!clusterHealthy) {
      return {
        available: false,
        error: 'Cluster unreachable or not running',
        suggestedAction: 'start_kubernetes'
      };
    }

    // 4. Detect installation type and version
    const installType = await invoke('detect_install_type'); // k3s, rancher, k3d, unknown
    const version = await invoke('get_cluster_version');

    return {
      available: true,
      type: installType,
      version: version,
      kubeconfigPath: kubeconfigPath
    };
  } catch (error) {
    return {
      available: false,
      error: error.message,
      suggestedAction: 'check_installation'
    };
  }
}
```

**Platform-Specific Installation Instructions**:

**macOS**:
```bash
# Option 1 (Recommended): Rancher Desktop
# Download from https://rancherdesktop.io/
# Enable Kubernetes in settings

# Option 2 (Advanced): Native k3s
brew install k3s
```

**Linux**:
```bash
# Option 1: Native k3s
curl -sfL https://get.k3s.io | sh -s - \
  --write-kubeconfig-mode 644 \
  --disable traefik

# Option 2: Rancher Desktop
# Download .deb/.rpm from https://rancherdesktop.io/
```

**Windows**:
```powershell
# Recommended: Rancher Desktop
# Download installer from https://rancherdesktop.io/

# Alternative: WSL2 + k3s
# Install WSL2, then run k3s inside Linux
```

**Future (Phase 3)**: Full bundling with VM/k3s binaries for zero-config installation. See GitHub Issue #15.

#### 4.3.2 Recommended k3s Configuration

When users install k3s manually, these are the recommended settings:

```bash
# Minimal k3s setup for Workspace
curl -sfL https://get.k3s.io | sh -s - \
  --write-kubeconfig-mode 644 \
  --disable traefik \
  --disable servicelb \
  --kube-apiserver-arg=feature-gates=ServerSideApply=true
```

**Why these flags**:
- `--write-kubeconfig-mode 644`: Readable kubeconfig (no sudo for kubectl)
- `--disable traefik`: We install our own ingress controller
- `--disable servicelb`: Use Knative networking instead
- `ServerSideApply=true`: Required for Knative

#### 4.3.3 Post-Installation Setup

Once Kubernetes is available (detected by the app), the following components are installed:

```bash
# Install Knative Serving
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.11.0/serving-crds.yaml
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.11.0/serving-core.yaml

# Install Traefik (Ingress)
kubectl apply -f k8s/traefik.yaml

# Install Local Registry
kubectl apply -f k8s/registry.yaml

# Install BuildKit (Image Building)
kubectl apply -f k8s/buildkit.yaml

# Wait for readiness
kubectl wait --for=condition=ready pod --all --all-namespaces --timeout=5m
```

**Note**: This setup is automated by the app once Kubernetes is detected.

#### 4.3.4 Manifests

**Registry** (`k8s/registry.yaml`):
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: registry-data
  namespace: default
spec:
  accessModes: [ReadWriteOnce]
  resources:
    requests:
      storage: 50Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: registry
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: registry
  template:
    metadata:
      labels:
        app: registry
    spec:
      containers:
      - name: registry
        image: registry:2
        ports:
        - containerPort: 5000
        volumeMounts:
        - name: data
          mountPath: /var/lib/registry
        env:
        - name: REGISTRY_STORAGE_DELETE_ENABLED
          value: "true"
      volumes:
      - name: data
        persistentVolumeClaim:
          claimName: registry-data
---
apiVersion: v1
kind: Service
metadata:
  name: registry
  namespace: default
spec:
  type: NodePort
  ports:
  - port: 5000
    targetPort: 5000
    nodePort: 30500
  selector:
    app: registry
```

**BuildKit** (`k8s/buildkit.yaml`):
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: buildkitd
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: buildkitd
  template:
    metadata:
      labels:
        app: buildkitd
    spec:
      containers:
      - name: buildkitd
        image: moby/buildkit:v0.12.0
        args:
        - --addr
        - unix:///run/buildkit/buildkitd.sock
        - --addr
        - tcp://0.0.0.0:1234
        securityContext:
          privileged: true
        volumeMounts:
        - name: buildkit-socket
          mountPath: /run/buildkit
        ports:
        - containerPort: 1234
      volumes:
      - name: buildkit-socket
        emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: buildkitd
  namespace: default
spec:
  ports:
  - port: 1234
    targetPort: 1234
  selector:
    app: buildkitd
```

**Traefik** (`k8s/traefik.yaml`):
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: traefik
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: traefik
  namespace: traefik
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: traefik
rules:
- apiGroups: [""]
  resources: ["services", "endpoints", "secrets"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["extensions", "networking.k8s.io"]
  resources: ["ingresses", "ingressclasses"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["traefik.containo.us"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: traefik
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: traefik
subjects:
- kind: ServiceAccount
  name: traefik
  namespace: traefik
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: traefik
  namespace: traefik
spec:
  replicas: 1
  selector:
    matchLabels:
      app: traefik
  template:
    metadata:
      labels:
        app: traefik
    spec:
      serviceAccountName: traefik
      containers:
      - name: traefik
        image: traefik:v2.10
        args:
        - --api.insecure=true
        - --providers.kubernetesingress
        - --providers.kubernetescrd
        - --entrypoints.web.address=:80
        - --entrypoints.websecure.address=:443
        ports:
        - name: web
          containerPort: 80
        - name: websecure
          containerPort: 443
        - name: admin
          containerPort: 8080
---
apiVersion: v1
kind: Service
metadata:
  name: traefik
  namespace: traefik
spec:
  type: NodePort
  ports:
  - name: web
    port: 80
    targetPort: 80
    nodePort: 30080
  - name: websecure
    port: 443
    targetPort: 443
    nodePort: 30443
  - name: admin
    port: 8080
    targetPort: 8080
    nodePort: 30808
  selector:
    app: traefik
```

---

## 5. Workspace Specifications

### 5.1 Workspace Resource

```yaml
# Example: Knative Service for a Next.js workspace
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: workspace-abc123
  namespace: workspace
  labels:
    workspace.dev/template: nextjs
    workspace.dev/owner: user
    workspace.dev/persistent: "true"
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/minScale: "1"  # or "0" for scale-to-zero
        autoscaling.knative.dev/maxScale: "1"
        autoscaling.knative.dev/target: "1"
    spec:
      containerConcurrency: 1
      timeoutSeconds: 300
      containers:
      - name: workspace
        image: localhost:5000/workspace-nextjs:latest
        ports:
        - containerPort: 8080
          name: http1
        env:
        - name: WORKSPACE_ID
          value: "abc123"
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: workspace-abc123-secrets
              key: claude-api-key
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: workspace-abc123-secrets
              key: openai-api-key
        - name: GIT_AUTHOR_NAME
          valueFrom:
            secretKeyRef:
              name: workspace-abc123-secrets
              key: git-author-name
        - name: GIT_AUTHOR_EMAIL
          valueFrom:
            secretKeyRef:
              name: workspace-abc123-secrets
              key: git-author-email
        - name: GIT_COMMITTER_NAME
          valueFrom:
            secretKeyRef:
              name: workspace-abc123-secrets
              key: git-author-name
        - name: GIT_COMMITTER_EMAIL
          valueFrom:
            secretKeyRef:
              name: workspace-abc123-secrets
              key: git-author-email
        resources:
          requests:
            cpu: "1"
            memory: "2Gi"
          limits:
            cpu: "2"
            memory: "4Gi"
        volumeMounts:
        # Workspace data (persistent)
        - name: workspace-data
          mountPath: /workspace
        # SSH keys from app-managed secrets
        - name: ssh-keys
          mountPath: /home/coder/.ssh
          readOnly: true
      volumes:
      - name: workspace-data
        persistentVolumeClaim:
          claimName: workspace-abc123-pvc
      - name: ssh-keys
        secret:
          secretName: workspace-abc123-ssh
          defaultMode: 0600
          optional: true  # SSH keys are optional
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: workspace-abc123-pvc
  namespace: workspace
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: local-path
---
apiVersion: v1
kind: Secret
metadata:
  name: workspace-abc123-secrets
  namespace: workspace
type: Opaque
stringData:
  claude-api-key: "sk-ant-..."
  openai-api-key: "sk-..."
  git-author-name: "John Doe"
  git-author-email: "john@example.com"
---
apiVersion: v1
kind: Secret
metadata:
  name: workspace-abc123-ssh
  namespace: workspace
type: kubernetes.io/ssh-auth
stringData:
  id_ed25519: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
  id_ed25519.pub: |
    ssh-ed25519 AAAAC3... john@example.com
```

### 5.2 Ingress Configuration

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: workspace-abc123
  namespace: workspace
spec:
  entryPoints:
  - web
  routes:
  - match: Host(`workspace-abc123.local`)
    kind: Rule
    services:
    - name: workspace-abc123
      port: 80
      kind: Service
```

### 5.3 Workspace Lifecycle

```
State Machine:

   [Creating] → [Starting] → [Running] → [Stopping] → [Stopped]
       ↓            ↓           ↓           ↓
   [Failed]    [Failed]    [Scaling]   [Deleting] → [Deleted]
                               ↓
                           [Running]

Transitions:
- Creating: Building image (if custom), creating k8s resources
- Starting: Knative scaling up from 0 → 1
- Running: Pod ready, code-server accessible
- Stopping: Knative scaling 1 → 0 (on-demand mode)
- Stopped: No pods running, PVC retained
- Scaling: Horizontal scaling (future feature)
- Deleting: Removing all resources including PVC
- Failed: Error state, requires manual intervention
```

---

## 6. API Documentation

### 6.1 API Endpoints

**Base URL**: `http://localhost:8090/api/v1`

#### 6.1.1 Workspaces

```
POST   /workspaces
GET    /workspaces
GET    /workspaces/:id
PUT    /workspaces/:id
DELETE /workspaces/:id
POST   /workspaces/:id/start
POST   /workspaces/:id/stop
GET    /workspaces/:id/logs
GET    /workspaces/:id/status
```

#### 6.1.2 Templates

```
GET    /templates
GET    /templates/:id
POST   /templates
PUT    /templates/:id
DELETE /templates/:id
POST   /templates/:id/build
GET    /templates/:id/build/logs    # SSE endpoint
POST   /templates/import             # From Dockerfile
```

#### 6.1.3 Cluster

```
GET    /cluster/status
POST   /cluster/install
POST   /cluster/uninstall
GET    /cluster/resources
```

#### 6.1.4 Credentials

```
GET    /credentials              # List all stored credentials (summary)
GET    /credentials/:id          # Get credential details (no secrets)
POST   /credentials              # Add new credential
PUT    /credentials/:id          # Update credential
DELETE /credentials/:id          # Delete credential
POST   /credentials/ssh/generate # Generate SSH key pair
```

#### 6.1.5 Registry

```
GET    /registry/images
DELETE /registry/images/:name/:tag
```

### 6.2 Request/Response Schemas

#### Create Workspace

```http
POST /api/v1/workspaces
Content-Type: application/json

{
  "name": "my-nextjs-project",
  "template": "nextjs",
  "persistent": true,
  "resources": {
    "cpu": "2",
    "memory": "4Gi",
    "storage": "20Gi"
  },
  "agents": {
    "claude": {
      "enabled": true,
      "model": "claude-3-5-sonnet-20241022"
    },
    "openai": {
      "enabled": false
    }
  },
  "networking": {
    "exposePort": 3000,
    "allowInterWorkspace": false
  }
}
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{
  "id": "abc123",
  "name": "my-nextjs-project",
  "template": "nextjs",
  "status": "creating",
  "urls": {
    "codeServer": "http://workspace-abc123.local",
    "app": "http://workspace-abc123-3000.local"
  },
  "createdAt": "2025-10-07T10:30:00Z",
  "resources": {
    "cpu": "2",
    "memory": "4Gi",
    "storage": "20Gi"
  }
}
```

#### List Workspaces

```http
GET /api/v1/workspaces
```

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "workspaces": [
    {
      "id": "abc123",
      "name": "my-nextjs-project",
      "template": "nextjs",
      "status": "running",
      "urls": {
        "codeServer": "http://workspace-abc123.local",
        "app": "http://workspace-abc123-3000.local"
      },
      "persistent": true,
      "createdAt": "2025-10-07T10:30:00Z",
      "lastAccessedAt": "2025-10-07T15:45:00Z"
    },
    {
      "id": "def456",
      "name": "data-analysis",
      "template": "jupyter",
      "status": "stopped",
      "urls": {
        "jupyter": "http://workspace-def456.local"
      },
      "persistent": false,
      "createdAt": "2025-10-06T09:00:00Z",
      "lastAccessedAt": "2025-10-06T18:30:00Z"
    }
  ],
  "total": 2
}
```

#### Build Custom Template

```http
POST /api/v1/templates
Content-Type: application/json

{
  "name": "my-django-template",
  "baseImage": "localhost:5000/workspace-base:latest",
  "dockerfile": "FROM localhost:5000/workspace-base:latest\n\nRUN apt-get update && apt-get install -y python3.11 python3-pip\n\nRUN pip3 install django djangorestframework\n\nUSER coder\nWORKDIR /workspace\n\nCMD [\"code-server\", \"--bind-addr\", \"0.0.0.0:8080\", \"--auth\", \"none\"]",
  "metadata": {
    "description": "Django REST Framework environment",
    "icon": "🐍",
    "category": "backend"
  }
}
```

```http
HTTP/1.1 202 Accepted
Content-Type: application/json

{
  "id": "template-xyz789",
  "name": "my-django-template",
  "status": "building",
  "buildLogsUrl": "/api/v1/templates/template-xyz789/build/logs"
}
```

#### Stream Build Logs (SSE)

```http
GET /api/v1/templates/template-xyz789/build/logs
Accept: text/event-stream
```

```
data: {"step": 1, "message": "FROM localhost:5000/workspace-base:latest", "timestamp": "2025-10-07T10:35:01Z"}

data: {"step": 2, "message": "RUN apt-get update && apt-get install -y python3.11 python3-pip", "timestamp": "2025-10-07T10:35:05Z"}

data: {"step": 2, "message": "Reading package lists...", "timestamp": "2025-10-07T10:35:06Z"}

data: {"step": 3, "message": "RUN pip3 install django djangorestframework", "timestamp": "2025-10-07T10:35:45Z"}

data: {"step": 4, "message": "Build complete", "timestamp": "2025-10-07T10:36:20Z", "status": "success"}
```

#### Cluster Status

```http
GET /api/v1/cluster/status
```

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "installed": true,
  "version": "v1.27.3+k3s1",
  "status": "healthy",
  "nodes": [
    {
      "name": "local",
      "ready": true,
      "cpu": {
        "used": "2.5",
        "total": "8"
      },
      "memory": {
        "used": "6Gi",
        "total": "16Gi"
      }
    }
  ],
  "components": {
    "knative": {
      "installed": true,
      "version": "v1.11.0",
      "healthy": true
    },
    "traefik": {
      "installed": true,
      "version": "v2.10",
      "healthy": true
    },
    "buildkit": {
      "installed": true,
      "healthy": true
    },
    "registry": {
      "installed": true,
      "healthy": true,
      "url": "localhost:5000"
    }
  }
}
```

#### Add Credential

```http
POST /api/v1/credentials
Content-Type: application/json

{
  "type": "ai_agent",
  "name": "My Claude API",
  "provider": "anthropic",
  "data": {
    "apiKey": "sk-ant-api03-...",
    "defaultModel": "claude-3-5-sonnet-20241022"
  }
}
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{
  "id": "cred-abc123",
  "type": "ai_agent",
  "name": "My Claude API",
  "provider": "anthropic",
  "createdAt": "2025-10-07T10:30:00Z"
}
```

#### List Credentials

```http
GET /api/v1/credentials
```

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "credentials": [
    {
      "id": "cred-abc123",
      "type": "ai_agent",
      "name": "My Claude API",
      "provider": "anthropic",
      "createdAt": "2025-10-07T10:30:00Z"
    },
    {
      "id": "cred-def456",
      "type": "git_config",
      "name": "Git Config",
      "data": {
        "name": "John Doe",
        "email": "john@example.com"
      },
      "createdAt": "2025-10-07T09:15:00Z"
    },
    {
      "id": "cred-ghi789",
      "type": "ssh_key",
      "name": "GitHub SSH Key",
      "data": {
        "publicKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...",
        "keyType": "ed25519"
      },
      "createdAt": "2025-10-07T09:20:00Z"
    }
  ]
}
```

#### Generate SSH Key

```http
POST /api/v1/credentials/ssh/generate
Content-Type: application/json

{
  "name": "GitHub SSH Key",
  "keyType": "ed25519",
  "comment": "john@workspace"
}
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{
  "id": "cred-ssh123",
  "name": "GitHub SSH Key",
  "publicKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJqfH... john@workspace",
  "keyType": "ed25519",
  "createdAt": "2025-10-07T10:35:00Z"
}
```

### 6.3 Error Responses

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "Invalid request",
  "details": "Template 'unknown' does not exist",
  "code": "TEMPLATE_NOT_FOUND"
}
```

```http
HTTP/1.1 409 Conflict
Content-Type: application/json

{
  "error": "Resource conflict",
  "details": "Workspace with name 'my-project' already exists",
  "code": "WORKSPACE_EXISTS"
}
```

```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{
  "error": "Cluster unavailable",
  "details": "k3s is not running. Please start the cluster.",
  "code": "CLUSTER_DOWN"
}
```

---

## 7. Security & Credentials

### 7.1 Credential Management

#### 7.1.1 In-App Credential Management

Users authenticate directly within the app. Credentials are stored securely and injected into workspaces as needed.

**Supported credential types**:
```
- AI Agents (Claude, OpenAI, etc.)
- Git (username, email, SSH keys)
- Docker registries
- Cloud providers (AWS, GCP, Azure)
- Package managers (npm, PyPI)
```

**Storage**:
- **Location**: App data directory (`~/.workspace/credentials/`)
- **Encryption**: AES-256 at rest, keys managed by OS keychain (Tauri secure storage)
- **Isolation**: Per-workspace credential assignment (opt-in)

#### 7.1.2 Credential Injection Strategy

Credentials are injected into workspaces via **Kubernetes Secrets**, not host volume mounts:

```yaml
env:
- name: ANTHROPIC_API_KEY
  valueFrom:
    secretKeyRef:
      name: workspace-abc123-secrets
      key: claude-api-key
- name: OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: workspace-abc123-secrets
      key: openai-api-key
- name: GIT_AUTHOR_NAME
  value: "John Doe"
- name: GIT_AUTHOR_EMAIL
  value: "john@example.com"
```

**For SSH keys** (generated or imported):
```yaml
volumeMounts:
- name: ssh-keys
  mountPath: /home/coder/.ssh
  readOnly: true
volumes:
- name: ssh-keys
  secret:
    secretName: workspace-abc123-ssh
    defaultMode: 0600
```

**Security considerations**:
- No direct host filesystem access
- Credentials stored encrypted in app storage
- Kubernetes Secrets created per-workspace
- Automatic cleanup on workspace deletion
- User controls which workspaces get which credentials

#### 7.1.3 AI Agent Authentication

**Flow**:
1. User opens Settings → Credentials in app
2. Clicks "Add Credential" → "AI Agent"
3. Selects provider (Claude, OpenAI, etc.)
4. Enters API key OR initiates OAuth flow
5. Credential saved to encrypted app storage
6. When creating workspace, user selects which agents to enable
7. Backend creates Kubernetes Secret with selected credentials
8. Environment variables available in workspace

**Example UI Flow**:
```
Settings → Credentials
┌─────────────────────────────────────┐
│ [+ Add Credential]                  │
├─────────────────────────────────────┤
│ 🤖 Claude API                       │
│    Model: claude-3-5-sonnet         │
│    [Edit] [Delete]                  │
├─────────────────────────────────────┤
│ 🔑 GitHub SSH Key                   │
│    Key: ssh-ed25519 AA...           │
│    [Edit] [Delete]                  │
├─────────────────────────────────────┤
│ 🐙 Git Config                       │
│    Name: John Doe                   │
│    Email: john@example.com          │
│    [Edit] [Delete]                  │
└─────────────────────────────────────┘
```

### 7.2 Network Security

#### 7.2.1 Workspace Isolation

By default, workspaces are **network-isolated**:

```yaml
# NetworkPolicy: Deny all inter-workspace traffic by default
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: workspace-isolation
  namespace: workspace
spec:
  podSelector: {}
  policyTypes:
  - Ingress
  - Egress
  egress:
  # Allow DNS
  - to:
    - namespaceSelector:
        matchLabels:
          name: kube-system
    ports:
    - protocol: UDP
      port: 53
  # Allow internet (but not other workspaces)
  - to:
    - namespaceSelector:
        matchExpressions:
        - key: name
          operator: NotIn
          values:
          - workspaces
```

#### 7.2.2 Inter-Workspace Communication (Opt-in)

When enabled via workspace settings:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: workspace-abc123-allow-def456
  namespace: workspace
spec:
  podSelector:
    matchLabels:
      workspace.dev/id: abc123
  ingress:
  - from:
    - podSelector:
        matchLabels:
          workspace.dev/id: def456
```

Access via: `workspace-def456.workspaces.svc.cluster.local`

---

## 8. Networking

### 8.1 Local DNS

#### 8.1.1 Strategy

**Primary**: `/etc/hosts` manipulation (requires sudo once)

```bash
# Automatically added by backend
127.0.0.1  workspace-abc123.local
127.0.0.1  workspace-abc123-3000.local
127.0.0.1  workspace-def456.local
```

**Alternative**: dnsmasq (for wildcard support)

```bash
# /etc/dnsmasq.conf
address=/workspace.local/127.0.0.1
```

#### 8.1.2 Domain Structure

- **Code Server**: `workspace-{id}.local` → port 8080
- **App Port 1**: `workspace-{id}-3000.local` → port 3000
- **App Port 2**: `workspace-{id}-8000.local` → port 8000
- **Custom**: `workspace-{id}-{port}.local` → port {port}

#### 8.1.3 Traefik Routing

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: workspace-abc123-codeserver
spec:
  entryPoints: [web]
  routes:
  - match: Host(`workspace-abc123.local`)
    kind: Rule
    services:
    - name: workspace-abc123
      port: 80
---
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: workspace-abc123-app-3000
spec:
  entryPoints: [web]
  routes:
  - match: Host(`workspace-abc123-3000.local`)
    kind: Rule
    services:
    - name: workspace-abc123
      port: 80
    middlewares:
    - name: port-forward-3000
---
apiVersion: traefik.containo.us/v1alpha1
kind: Middleware
metadata:
  name: port-forward-3000
spec:
  headers:
    customRequestHeaders:
      X-Forwarded-Port: "3000"
```

### 8.2 Port Forwarding

#### 8.2.1 Workspace → Host

Automatic via Traefik IngressRoutes (see above).

#### 8.2.2 Dynamic Port Exposure

User can expose additional ports at runtime:

```http
POST /api/v1/workspaces/abc123/expose
{
  "port": 5432,
  "protocol": "tcp"
}
```

Backend creates:
1. New IngressRoute for `workspace-abc123-5432.local`
2. Updates `/etc/hosts`
3. Returns URL

---

### 8.3 Certificate Management (Cloud Mode)

#### 8.3.1 cert-manager Integration

**Installation**:
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

**ClusterIssuer** (Let's Encrypt):
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: user@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: traefik
    # DNS-01 challenge for wildcard certs
    - dns01:
        cloudflare:
          email: user@example.com
          apiTokenSecretRef:
            name: cloudflare-api-token
            key: api-token
      selector:
        dnsZones:
        - "yourdomain.com"
```

#### 8.3.2 Automatic Certificate Provisioning

When workspace is created in cloud mode:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: workspace-abc123-cert
  namespace: workspace
spec:
  secretName: workspace-abc123-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
  - workspace-abc123.yourdomain.com
  - myproject.example.com  # custom domain
```

**IngressRoute with TLS**:
```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: workspace-abc123
  namespace: workspace
spec:
  entryPoints:
  - websecure
  routes:
  - match: Host(`workspace-abc123.yourdomain.com`)
    kind: Rule
    services:
    - name: workspace-abc123
      port: 80
  tls:
    secretName: workspace-abc123-tls
```

**Auto-renewal**: cert-manager handles renewal 30 days before expiry.

---

### 8.4 Custom Domains

#### 8.4.1 Domain Management

Users can assign custom domains to workspaces:

**UI Flow**:
```
Workspace Settings → Domains
├─ Default: workspace-abc123.yourdomain.com
└─ Custom Domains:
   ├─ [+ Add Domain]
   │  ├─ Domain: [myproject.example.com]
   │  ├─ Auto-configure DNS: [✓]  (requires provider credentials)
   │  └─ [Add]
   │
   └─ myproject.example.com ✓ Active
      [Remove] [Verify DNS]
```

**API Endpoint**:
```http
POST /api/v1/workspaces/abc123/domains
{
  "domain": "myproject.example.com",
  "autoConfigureDNS": true,
  "provider": "cloudflare"
}
```

#### 8.4.2 DNS Provider Integration

**Supported Providers**:
- Cloudflare (via API token)
- Route53 (via AWS credentials)
- DigitalOcean DNS (via API token)
- Google Cloud DNS (via service account)
- Manual (user adds DNS records themselves)

**Automatic DNS Configuration**:

When `autoConfigureDNS: true`:
1. Backend validates domain ownership (TXT record challenge)
2. Creates DNS records via provider API:
   ```
   A record: myproject.example.com → <cluster-public-ip>
   or
   CNAME: myproject.example.com → workspace.yourdomain.com
   ```
3. Updates IngressRoute with new domain
4. cert-manager automatically provisions certificate
5. Workspace accessible via custom domain over HTTPS

**Go Implementation**:
```go
// api/pkg/network/dns.go
package network

import (
    "github.com/cloudflare/cloudflare-go"
    "github.com/aws/aws-sdk-go/service/route53"
)

type DNSProvider interface {
    CreateRecord(domain, target string) error
    DeleteRecord(domain string) error
    VerifyOwnership(domain string) (bool, error)
}

type CloudflareProvider struct {
    api *cloudflare.API
}

func (p *CloudflareProvider) CreateRecord(domain, target string) error {
    zoneID, _ := p.api.ZoneIDByName(extractZone(domain))
    record := cloudflare.DNSRecord{
        Type:    "CNAME",
        Name:    domain,
        Content: target,
        TTL:     300,
    }
    _, err := p.api.CreateDNSRecord(zoneID, record)
    return err
}
```

#### 8.4.3 Multi-Domain Support

One workspace can have multiple domains:

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: workspace-abc123
spec:
  entryPoints: [websecure]
  routes:
  - match: Host(`workspace-abc123.yourdomain.com`) || Host(`myproject.example.com`) || Host(`app.custom.dev`)
    kind: Rule
    services:
    - name: workspace-abc123
      port: 80
  tls:
    secretName: workspace-abc123-tls
    domains:
    - main: workspace-abc123.yourdomain.com
    - main: myproject.example.com
    - main: app.custom.dev
```

**Certificate for Multiple Domains**:
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: workspace-abc123-cert
spec:
  secretName: workspace-abc123-tls
  dnsNames:
  - workspace-abc123.yourdomain.com
  - myproject.example.com
  - app.custom.dev
```

#### 8.4.4 Wildcard Domains (Advanced)

For SaaS-style deployments:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: wildcard-cert
spec:
  secretName: wildcard-tls
  dnsNames:
  - "*.workspace.yourdomain.com"
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
```

All workspaces can use:
- `workspace-abc123.workspace.yourdomain.com`
- `workspace-def456.workspace.yourdomain.com`
- etc.

---

## 9. Template System

### 9.1 Built-in Templates

#### 9.1.1 Base Image

**Location**: `images/base/Dockerfile`

```dockerfile
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
RUN curl -fsSL https://code-server.dev/install.sh | sh -s -- --version=4.18.0

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
```

#### 9.1.2 Next.js Template

**Location**: `images/templates/nextjs/Dockerfile`

```dockerfile
FROM localhost:5000/workspace-base:latest

USER root

# Install Node.js 20 LTS
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*

# Install pnpm
RUN npm install -g pnpm

USER coder

# Pre-install commonly used VS Code extensions
RUN code-server --install-extension dbaeumer.vscode-eslint && \
    code-server --install-extension esbenp.prettier-vscode && \
    code-server --install-extension bradlc.vscode-tailwindcss && \
    code-server --install-extension formulahendry.auto-rename-tag && \
    code-server --install-extension csstools.postcss

# Pre-create a Next.js app template (optional)
RUN cd /tmp && \
    pnpm create next-app@latest next-template \
      --typescript \
      --tailwind \
      --app \
      --no-src-dir \
      --import-alias "@/*" && \
    tar czf /home/coder/next-template.tar.gz -C /tmp next-template && \
    rm -rf /tmp/next-template

# Create init script to scaffold on first run
RUN echo '#!/bin/bash\n\
if [ ! -f /workspace/.initialized ]; then\n\
  echo "Initializing Next.js workspace..."\n\
  tar xzf ~/next-template.tar.gz -C /workspace --strip-components=1\n\
  cd /workspace && pnpm install\n\
  touch /workspace/.initialized\n\
fi' > /home/coder/init.sh && chmod +x /home/coder/init.sh

CMD ["/bin/bash", "-c", "~/init.sh && code-server /workspace"]
```

#### 9.1.3 Vue Template

**Location**: `images/templates/vue/Dockerfile`

```dockerfile
FROM localhost:5000/workspace-base:latest

USER root
RUN curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && \
    apt-get install -y nodejs && \
    rm -rf /var/lib/apt/lists/*
RUN npm install -g pnpm

USER coder

# Vue-specific extensions
RUN code-server --install-extension Vue.volar && \
    code-server --install-extension Vue.vscode-typescript-vue-plugin && \
    code-server --install-extension dbaeumer.vscode-eslint && \
    code-server --install-extension esbenp.prettier-vscode

# Pre-create Vue 3 + Vite template
RUN cd /tmp && \
    pnpm create vite@latest vue-template --template vue-ts && \
    tar czf /home/coder/vue-template.tar.gz -C /tmp vue-template && \
    rm -rf /tmp/vue-template

RUN echo '#!/bin/bash\n\
if [ ! -f /workspace/.initialized ]; then\n\
  echo "Initializing Vue workspace..."\n\
  tar xzf ~/vue-template.tar.gz -C /workspace --strip-components=1\n\
  cd /workspace && pnpm install\n\
  touch /workspace/.initialized\n\
fi' > /home/coder/init.sh && chmod +x /home/coder/init.sh

CMD ["/bin/bash", "-c", "~/init.sh && code-server /workspace"]
```

#### 9.1.4 Jupyter Template

**Location**: `images/templates/jupyter/Dockerfile`

```dockerfile
FROM localhost:5000/workspace-base:latest

USER root

# Install Python 3.11
RUN apt-get update && apt-get install -y \
    python3.11 \
    python3.11-dev \
    python3-pip \
    && rm -rf /var/lib/apt/lists/*

USER coder

# Install Jupyter and data science libraries
RUN pip3 install --user \
    jupyterlab \
    numpy \
    pandas \
    matplotlib \
    seaborn \
    scikit-learn \
    scipy \
    plotly

# Jupyter VS Code extension
RUN code-server --install-extension ms-toolsai.jupyter

# Configure Jupyter to run on port 8888
RUN mkdir -p ~/.jupyter && \
    echo "c.ServerApp.ip = '0.0.0.0'" > ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.port = 8888" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.token = ''" >> ~/.jupyter/jupyter_lab_config.py && \
    echo "c.ServerApp.password = ''" >> ~/.jupyter/jupyter_lab_config.py

EXPOSE 8080 8888

# Start both code-server and Jupyter
CMD ["/bin/bash", "-c", "jupyter lab --no-browser &>/dev/null & code-server /workspace"]
```

### 9.2 Custom Templates

#### 9.2.1 Template Metadata

```json
{
  "id": "template-xyz789",
  "name": "my-django-template",
  "description": "Django REST Framework with PostgreSQL",
  "author": "username",
  "version": "1.0.0",
  "icon": "🐍",
  "category": "backend",
  "tags": ["python", "django", "api"],
  "baseImage": "localhost:5000/workspace-base:latest",
  "dockerfile": "...",
  "defaultPorts": [8000, 5432],
  "defaultResources": {
    "cpu": "2",
    "memory": "4Gi",
    "storage": "10Gi"
  },
  "extensions": [
    "ms-python.python",
    "ms-python.vscode-pylance"
  ],
  "createdAt": "2025-10-07T10:30:00Z"
}
```

#### 9.2.2 Template Builder UI Flow

```
1. User clicks "Create Custom Template"
   ├─> Modal opens with form
   │   ├─ Name (required)
   │   ├─ Description
   │   ├─ Icon picker
   │   ├─ Base image dropdown (base, nextjs, vue, jupyter)
   │   └─ Dockerfile editor (Monaco)
   │
2. User writes/pastes Dockerfile
   ├─> Syntax highlighting
   ├─> Real-time validation
   └─> Auto-complete for common commands
   │
3. User clicks "Build"
   ├─> POST /api/templates
   ├─> Build starts in BuildKit
   ├─> Modal switches to "Build Logs" view
   │   └─> SSE connection shows real-time output
   │
4. Build completes
   ├─> Success: Template saved, image pushed to registry
   │   └─> Appears in template gallery
   └─> Failure: Show errors, allow retry
```

### 9.3 Template Sharing (Future)

#### 9.3.1 Export

```bash
# CLI command (future)
$ workspace template export my-django-template

# Creates:
my-django-template.tar.gz
├── Dockerfile
├── template.json  (metadata)
└── files/         (optional: init scripts, config files)
```

#### 9.3.2 Import

```bash
# CLI command
$ workspace template import my-django-template.tar.gz

# Or via UI: drag-and-drop .tar.gz file
```

#### 9.3.3 Marketplace (Future)

- GitHub repo: `workspace/templates`
- Each template is a directory with Dockerfile + metadata
- UI fetches index.json, shows gallery
- One-click install builds from GitHub

---

## 10. Implementation Phases

### Phase 1: Foundation (Weeks 1-2)

**Goal**: Basic infrastructure working

#### Tasks:
- [x] Project scaffolding
  - Tauri app skeleton
  - Go backend skeleton
  - Basic React UI with routing
- [x] k3s automation
  - Installation script
  - Health check API
  - Uninstall script
- [x] Local registry deployment
  - registry:2 manifest
  - NodePort service on 30500
  - /etc/rancher/k3s/registries.yaml config
- [x] BuildKit deployment
  - Deployment manifest
  - Service on port 1234
  - Go client integration
- [x] Base image build
  - Dockerfile with code-server
  - Push to local registry
- [x] Basic workspace CRUD (without Knative)
  - Simple Pod + Service
  - PVC creation
  - API endpoints

**Deliverable**: Can create a workspace, access code-server via port-forward

---

### Phase 2: Workspaces (Weeks 2-3)

**Goal**: Full workspace lifecycle with templates

#### Tasks:
- [x] Knative Serving integration
  - Install Knative manifests
  - Convert Pods to Knative Services
  - Scale-to-zero configuration
- [x] Template system
  - Build 3 base templates (Next.js, Vue, Jupyter)
  - Template metadata storage
  - Template selection in create flow
- [x] PVC management
  - Dynamic provisioning
  - Lifecycle tied to workspace
  - Optional deletion on workspace removal
- [x] Workspace status monitoring
  - Poll Knative Service status
  - WebSocket updates to frontend
  - Start/stop operations

**Deliverable**: Can create workspaces from templates, they scale to zero

---

### Phase 3: Networking (Weeks 3-4)

**Goal**: Local DNS and multi-port access

#### Tasks:
- [x] Traefik deployment
  - Install manifest
  - NodePort services
  - Basic IngressRoute
- [x] Local DNS setup
  - /etc/hosts manipulation (requires sudo prompt)
  - Auto-add entries on workspace create
  - Auto-remove on workspace delete
- [x] Dynamic IngressRoute creation
  - Code-server route (port 8080)
  - App port routes (e.g., 3000, 8000)
  - Go API to create/delete IngressRoutes
- [x] Port exposure API
  - POST /workspaces/:id/expose
  - Create new IngressRoute + DNS entry
  - Update workspace URLs

**Deliverable**: Access workspaces via `workspace-{id}.local`

---

### Phase 4: AI Integration (Week 4)

**Goal**: Seamless AI agent authentication via in-app credential management

#### Tasks:
- [x] Credential management UI
  - Settings panel for adding/editing credentials
  - Support for AI agents (Claude, OpenAI)
  - Git config and SSH key management
  - Secure storage using Tauri secure storage (OS keychain)
- [x] Credential encryption
  - AES-256 encryption at rest
  - Secure retrieval for workspace creation
- [x] SSH key generation
  - In-app SSH key pair generation (ED25519, RSA)
  - Public key display and copy
  - Private key encrypted storage
- [x] Kubernetes Secret generation
  - Create secrets per-workspace from app credentials
  - Environment variable injection
  - SSH key volume mounts
- [x] Extension pre-installation
  - Claude Code extension in base image
  - Auto-configure on first launch
  - GitHub Copilot (if user has subscription)

**Deliverable**: Login once in app, credentials available in all workspaces

---

### Phase 5: Custom Templates (Week 4-5)

**Goal**: Users can build custom templates

#### Tasks:
- [x] Template builder UI
  - Monaco editor for Dockerfile
  - Base image selector
  - Metadata form (name, icon, description)
- [x] BuildKit integration
  - Build API endpoint
  - SSE log streaming
  - Progress indicators
- [x] Template management
  - List custom templates
  - Edit existing templates
  - Delete templates (+ registry cleanup)
- [x] Error handling
  - Parse BuildKit errors
  - Display in UI with line numbers
  - Retry mechanism

**Deliverable**: Create custom templates via UI, use in workspaces

---

### Phase 6: Polish & Testing (Week 5)

**Goal**: Production-ready MVP

#### Tasks:
- [x] Embedded WebView
  - Multi-tab support in Tauri
  - Persistent tab state
  - Context menu (Open in Browser, Reload, etc.)
- [x] Resource monitoring
  - Real-time CPU/Memory usage per workspace
  - Cluster-wide resource dashboard
  - Alerts for high usage
- [x] Error handling
  - User-friendly error messages
  - Automatic retries for transient errors
  - "Recover" actions for failed workspaces
- [x] Documentation
  - README with quick start
  - Architecture diagram
  - API documentation
  - Troubleshooting guide
- [x] Testing
  - Unit tests (Go backend)
  - Integration tests (API + k3s)
  - E2E tests (Tauri app)
  - Manual QA checklist

**Deliverable**: Stable MVP ready for beta users

---

## 11. User Flows

### 11.1 First-Time Setup

#### MVP Flow (Phase 1)

```
1. User downloads and launches Workspace

2. Kubernetes Detection
   ├─> App checks for kubectl/k3s
   │
   ├─ If Found (Kubernetes Available):
   │   ├─> ✅ Kubernetes Ready (Rancher Desktop v1.x.x)
   │   │   Running k3s v1.28.x
   │   │
   │   └─> [Continue to Setup]
   │
   └─ If Not Found:
       ┌─────────────────────────────────────────────┐
       │ Kubernetes Required                         │
       │                                             │
       │ Workspace needs Kubernetes to run.          │
       │                                             │
       │ Recommended:                                │
       │ [Download Rancher Desktop]                  │
       │                                             │
       │ Or install manually:                        │
       │ macOS:   brew install k3s                   │
       │ Linux:   curl -sfL https://get.k3s.io | sh -│
       │ Windows: Use Rancher Desktop or WSL2        │
       │                                             │
       │ [Copy Command] [Show Instructions]          │
       │                                             │
       │ After installation:                         │
       │ [Verify Installation]                       │
       └─────────────────────────────────────────────┘

3. Post-Kubernetes Setup (Automated)
   ├─> [▓▓▓▓▓░░░░░] Deploying Knative...
   ├─> [▓▓▓▓▓▓▓░░░] Setting up Traefik...
   ├─> [▓▓▓▓▓▓▓▓░░] Installing local registry...
   └─> [▓▓▓▓▓▓▓▓▓▓] Building base images... (3/3)
   │
4. Setup complete!
   ├─> "Your local workspace cluster is ready!"
   │
   └─> [Create your first workspace]
       [Set up credentials (optional)]
```

#### Future Flow (Phase 3 - Full Bundling)

```
1. User downloads and launches Workspace

2. One-Click Setup
   ├─> "Set up Kubernetes cluster" button
   │   └─> Automatic installation (no sudo prompt from app)
   │
3. Installation progress
   ├─> [▓▓▓▓▓░░░░░] Setting up VM...
   ├─> [▓▓▓▓▓▓▓░░░] Installing k3s...
   ├─> [▓▓▓▓▓▓▓▓░░] Deploying Knative...
   └─> [▓▓▓▓▓▓▓▓▓▓] Building base images...
   │
4. Setup complete! (Zero configuration required)
```

### 11.1.1 Setting Up Credentials (Optional)

```
1. Click "Settings" → "Credentials" (or from welcome screen)

2. Credential management screen
   ┌─────────────────────────────────────┐
   │ Credentials                         │
   │ [+ Add Credential ▼]                │
   │   ├─ AI Agent                       │
   │   ├─ Git Config                     │
   │   ├─ SSH Key                        │
   │   ├─ Docker Registry                │
   │   └─ Cloud Provider                 │
   └─────────────────────────────────────┘

3. Example: Add Claude API
   ├─> Click "Add Credential" → "AI Agent"
   │
   └─> Modal: "Add AI Agent"
       Provider: [Anthropic Claude ▼]
       Name: [My Claude API]
       API Key: [sk-ant-api03-...        ]
       Default Model: [Claude 3.5 Sonnet ▼]

       [Test Connection] [Cancel] [Save]

4. Example: Generate SSH Key
   ├─> Click "Add Credential" → "SSH Key"
   │
   └─> Modal: "Add SSH Key"
       ○ Generate new key
       ● Import existing key

       Key Type: [ED25519 ▼]
       Comment: [john@workspace]

       [Generate] [Cancel]

5. Key generated
   ├─> Shows public key for copying
   │   ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...
   │   [Copy] [Add to GitHub] [Add to GitLab]
   │
   └─> Private key stored encrypted in app

6. Credentials ready
   └─> Now available in workspace creation flow
```

### 11.2 Create Workspace (Built-in Template)

```
1. Click "New Workspace"

2. Modal opens: "Create Workspace"
   ├─ Step 1: Choose Template
   │  ┌─────────┐  ┌─────────┐  ┌─────────┐
   │  │ Next.js │  │   Vue   │  │ Jupyter │  [+Custom]
   │  │  ⚛️     │  │   🟢    │  │   📊    │
   │  └─────────┘  └─────────┘  └─────────┘
   │
   ├─ Step 2: Configure
   │  Name: [my-nextjs-project        ]
   │  Persistent: [x] Keep running
   │  Resources:
   │    CPU:     [2 cores ▼]
   │    Memory:  [4 GB    ▼]
   │    Storage: [10 GB   ▼]
   │
   ├─ Step 3: AI Agents (optional)
   │  ☑ Claude Code
   │    Credential: [My Claude API ▼] or [+ Add new]
   │    Model: [Claude 3.5 Sonnet ▼]
   │  ☐ OpenAI Codex
   │    Credential: [Not configured - Add in Settings]
   │
   └─ Step 4: Advanced (optional)
      ☐ Allow inter-workspace networking
      Expose ports: [3000, 3001        ]
      Environment variables:
        NODE_ENV=development

3. Click "Create"
   ├─> Progress indicator
   │   "Creating workspace..."
   │   "Pulling image..."         ✓
   │   "Creating volumes..."      ✓
   │   "Starting container..."    ...
   │
4. Workspace ready!
   ├─> Card appears in workspace list
   │   Status: 🟢 Running
   │   URLs:
   │     Code: workspace-abc123.local
   │     App:  workspace-abc123-3000.local
   │
   └─> [Open in App] [Open in Browser] [Settings]
```

### 11.3 Create Workspace (Custom Template)

```
1. Click "New Workspace" → "Custom Template"

2. Template Builder
   ├─ Name: [my-django-template]
   ├─ Icon: [🐍 ▼]
   ├─ Base: [workspace-base ▼]
   │
   └─ Dockerfile:
      ┌─────────────────────────────────────┐
      │ FROM localhost:5000/workspace-base  │
      │                                     │
      │ USER root                           │
      │ RUN apt-get update && \             │
      │     apt-get install -y python3.11   │
      │                                     │
      │ USER coder                          │
      │ RUN pip3 install django             │
      └─────────────────────────────────────┘

3. Click "Build"
   ├─> Build logs appear (real-time SSE)
   │   [Step 1/4] FROM localhost:5000/workspace-base
   │   [Step 2/4] RUN apt-get update
   │     Reading package lists...
   │   [Step 3/4] RUN pip3 install django
   │     Collecting django...
   │   [Step 4/4] Build complete ✓
   │
4. Template saved
   └─> Now appears in template gallery

5. Create workspace from it (same flow as 11.2)
```

### 11.4 Daily Usage

```
1. User opens Workspace app

2. Dashboard shows existing workspaces
   ┌─────────────────────────────────────┐
   │ my-nextjs-project         🟢 Running │
   │ Last accessed: 2 hours ago          │
   │ [Open] [Stop] [Settings]            │
   ├─────────────────────────────────────┤
   │ data-analysis             ⚪ Stopped │
   │ Last accessed: yesterday            │
   │ [Start] [Delete]                    │
   └─────────────────────────────────────┘

3. Click "Open" on running workspace
   ├─> New tab opens in app with embedded code-server
   │   Multiple tabs can be open simultaneously
   │
   └─> User codes with Claude Code assistant
       (No auth required, uses host credentials)

4. Click "Start" on stopped workspace
   ├─> Knative scales pod from 0 → 1
   │   Progress: "Starting workspace... (30s)"
   │
   └─> Once ready, [Open] button appears

5. Workspace auto-stops after inactivity (if not persistent)
   └─> Knative scales 1 → 0 (saves resources)
```

### 11.5 Expose New Port

```
1. User is working in workspace, starts dev server on port 8000

2. In Workspace app:
   ├─> Click workspace → "Settings" → "Ports" tab
   │
   └─> Click "Expose Port"
       Port: [8000]
       Protocol: [HTTP ▼]

3. Click "Add"
   ├─> Backend creates IngressRoute
   ├─> Updates /etc/hosts
   └─> Returns URL

4. New URL appears in workspace card
   App: workspace-abc123-8000.local
   └─> Click to open in browser
```

---

## 12. Future Enhancements

### 12.1 Post-MVP Features

#### 12.1.1 GitOps with ArgoCD

**When**: After MVP, when multi-user/team scenarios emerge

**What**:
- Workspace definitions as Git repos
- Automatic sync on Git push
- Rollback to previous workspace states
- Audit trail of all changes

**Why**:
- Teams can share workspace configs
- Version-controlled infrastructure
- Better collaboration

#### 12.1.2 Image Registry Upgrade (Harbor)

**When**: Security scanning and RBAC become requirements

**What**:
- Replace registry:2 with Harbor
- Vulnerability scanning for all images
- User/team-based access control
- Webhook integrations (Slack, etc.)
- Image signing and verification

**Why**:
- Enterprise security requirements
- Compliance (scan for CVEs)
- Better multi-user support

#### 12.1.3 Multi-User Support

**When**: Teams want to run on a shared machine/server

**What**:
- User authentication (OAuth, LDAP)
- Per-user workspaces and quotas
- Shared templates marketplace
- Team workspaces (multiple users, same workspace)

**Why**:
- Shared infrastructure cost savings
- Collaboration on same environment

#### 12.1.4 Cloud Deployment

**When**: Users want remote access or more resources

**What**:
- Deploy to remote k3s/k8s cluster
- Cloud provider integrations (AWS, GCP, Azure)
- SSH tunneling for secure access
- Workspace migration (local ↔ cloud)

**Why**:
- Access from anywhere
- More powerful machines
- CI/CD integration

#### 12.1.5 Template Marketplace

**When**: Community grows, users want to share

**What**:
- Public template registry
- Browse/search/install templates
- Ratings and reviews
- One-click install from GitHub

**Why**:
- Ecosystem growth
- Reduce duplication
- Discover best practices

#### 12.1.6 Advanced Networking

**When**: Complex multi-service architectures needed

**What**:
- Service mesh (Istio/Linkerd)
- Workspace-to-workspace mTLS
- Traffic splitting (A/B testing)
- Observability (Jaeger, Grafana)

**Why**:
- Microservices development
- Better debugging
- Production-like environments

#### 12.1.7 Workspace Snapshots

**When**: Users need backup/restore capability

**What**:
- One-click snapshot (PVC + metadata)
- Restore to point in time
- Share snapshots with team
- Scheduled automatic snapshots

**Why**:
- Experiment without fear
- Recover from mistakes
- Reproduce bugs

#### 12.1.8 IDE Alternatives

**When**: Users want choice beyond VS Code

**What**:
- JetBrains Gateway integration
- Vim/Neovim in terminal
- Emacs with LSP
- Eclipse Che

**Why**:
- Personal preference
- Language-specific IDEs (PyCharm, GoLand)

### 12.2 Roadmap

```
MVP (v0.1.0) - Week 5
├─ Local k3s cluster
├─ 3 built-in templates
├─ Custom template builder
├─ Host credential mounting
└─ Embedded VS Code

v0.2.0 - Week 8
├─ Template marketplace (GitHub)
├─ Workspace snapshots
├─ Resource usage dashboard
└─ CLI tool

v0.3.0 - Week 12
├─ Multi-user support
├─ Team workspaces
├─ Harbor integration
└─ Advanced networking

v1.0.0 - Month 6
├─ Cloud deployment
├─ ArgoCD integration
├─ Production-ready
└─ Full documentation
```

---

## 13. Non-Functional Requirements

### 13.1 Performance

- **Workspace start time**: <30s (from stopped to accessible)
- **Scale-to-zero**: <3s (from idle to stopped)
- **Build time**: <5min for typical template
- **API response**: <200ms (p95)
- **UI rendering**: <100ms (p95)

### 13.2 Resource Usage

- **k3s overhead**: <512MB RAM idle
- **Workspace idle**: <200MB RAM (scaled to zero)
- **Workspace active**: ~2GB RAM (code-server + app)
- **Disk**: ~10GB per workspace (adjustable)
- **Total**: Minimum 8GB RAM, 50GB disk for 5 workspaces

### 13.3 Reliability

- **Workspace uptime**: 99% (excluding user-initiated stops)
- **Data durability**: 99.9% (PVC on local disk)
- **Cluster recovery**: Automatic restart after crash
- **Backup**: User-initiated snapshots

### 13.4 Security

- **Credential isolation**: Read-only host mounts
- **Network isolation**: Default deny inter-workspace
- **Privilege escalation**: Containers run as non-root (UID 1000)
- **Secret management**: Kubernetes Secrets (base64)
- **API authentication**: (Future) JWT tokens

### 13.5 Usability

- **First-time setup**: <5 minutes
- **Create workspace**: <3 clicks, <60s
- **Learning curve**: No Kubernetes knowledge required
- **Documentation**: Inline help, tooltips, video tutorials

---

## 14. Open Questions

1. **Windows support**: WSL2 for k3s, or native Windows containers?
2. **macOS M1/M2**: ARM-based templates, or cross-compile x86?
3. **Backup strategy**: User-initiated only, or automated?
4. **Telemetry**: Anonymous usage stats for product improvements?
5. **Pricing**: Forever free and open-source, or hosted version?
6. **License**: MIT, Apache 2.0, or custom?

---

## 15. References

- [k3s Documentation](https://docs.k3s.io/)
- [Knative Serving](https://knative.dev/docs/serving/)
- [Traefik](https://doc.traefik.io/traefik/)
- [BuildKit](https://github.com/moby/buildkit)
- [code-server](https://github.com/coder/code-server)
- [Tauri](https://tauri.app/)
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code)

---

**End of Specification**

*Version 1.0.0 - 2025-10-07*