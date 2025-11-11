# vibespace - Technical Specification

**Version:** 1.0.0
**Date:** 2025-10-07
**Status:** MVP Specification

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture](#architecture)
3. [Technical Stack](#technical-stack)
4. [Core Components](#core-components)
5. [Vibespace Specifications](#vibespace-specifications)
6. [API Documentation](#api-documentation)
7. [Security & Credentials](#security--credentials)
8. [Networking](#networking)
9. [Template System](#template-system)
10. [Implementation Phases](#implementation-phases)
11. [User Flows](#user-flows)
12. [Future Enhancements](#future-enhancements)

---

## 1. Project Overview

**Project Name**: `vibespace`

### 1.1 Naming Conventions

**Project Structure**:
```
vibespace/                  # Root (singular)
├── app/                   # Desktop application
├── api/                   # Backend API server
├── images/                # Container images
├── k8s/                   # Kubernetes manifests
└── docs/                  # Documentation
```

**Kubernetes Resources**:
- Namespace: `vibespace` (singular)
- Labels: `vibespace.dev/*`
- Resources: `vibespace-{id}`, `vibespace-{id}-pvc`, `vibespace-{id}-secrets`

**Domains**:
- Pattern: `vibespace-{id}.local`
- App ports: `vibespace-{id}-3000.local`

**Go Packages**:
- Singular names: `vibespace/`, `template/`, `credential/`
- Standard layout: `cmd/`, `pkg/`, `config/`

**API Paths**:
- Collections: `/api/v1/vibespaces`
- Single resource: `/api/v1/vibespaces/{id}`

### 1.2 Vision

An open-source Tauri desktop app for managing isolated dev environments running in local k3s. Each vibespace is a containerized environment with code-server (VS Code in browser), supports AI coding agents like Claude Code and OpenAI Codex.

### 1.3 Goals

- **Isolated Environments**: Spin up project-specific vibespaces with custom configurations
- **AI-Ready**: Pre-configured for coding agents with seamless authentication
- **Local-First**: All vibespaces run on local k3s cluster, no cloud dependency
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
✅ On-demand and persistent vibespaces
✅ Scale-to-zero with Knative
✅ Local DNS (*.local)
✅ Host credential mounting (SSH, Git, AI agents)
✅ Custom template builder with BuildKit
✅ 3 built-in templates (Next.js, Vue, Jupyter)
✅ AI agent configuration (Claude Code, OpenAI Codex)
✅ Inter-vibespace networking (configurable)
✅ Port forwarding to host

### 1.6 Key Features (Extended)

🎯 **Cloud Deployment Mode**
- Desktop app runs locally
- Backend API + vibespaces run in cloud (AWS, GCP, DigitalOcean, etc.)
- Connect to remote vibespaces via embedded code-server
- Managed k3s cluster in cloud
- Secure tunnel for vibespace access

🎯 **Certificate Management**
- Automatic TLS certificate provisioning (Let's Encrypt via cert-manager)
- Per-vibespace HTTPS endpoints
- Custom domain support (`myproject.example.com`)
- Wildcard certificates for subdomains
- Certificate auto-renewal

🎯 **Custom Domains**
- Map vibespaces to custom domains
- DNS provider integration (Cloudflare, Route53, etc.)
- Automatic DNS record creation
- Support for multiple domains per vibespace
- CNAME and A record management

---

## 2. Architecture

### 2.1 System Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     Desktop App (Tauri)                     │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────┐ │
│  │  Vibespace List │  │  Template       │  │  Settings   │ │
│  │  + Control      │  │  Builder        │  │  Panel      │ │
│  └─────────────────┘  └─────────────────┘  └─────────────┘ │
│  ┌───────────────────────────────────────────────────────┐  │
│  │         Embedded VS Code (WebView)                    │  │
│  │         Multiple Tabs for Vibespaces                  │  │
│  └───────────────────────────────────────────────────────┘  │
└──────────────────────┬──────────────────────────────────────┘
                       │ HTTP (localhost:8090)
                       ▼
┌─────────────────────────────────────────────────────────────┐
│                    API Server (Go)                          │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │  Vibespace   │  │  Template    │  │  k3s Manager    │   │
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
│  │  Knative Serving (Vibespaces)                         │ │
│  │  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐  │ │
│  │  │ vibespace-1 │  │ vibespace-2 │  │ vibespace-3  │  │ │
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

1. **User creates vibespace via Tauri UI**
2. **Tauri → Go API**: POST /api/vibespaces (template, config)
3. **Go API → BuildKit**: Build image if custom template
4. **Go API → k3s**: Create Knative Service + PVC
5. **Go API → Traefik**: Configure IngressRoute
6. **Go API → DNS**: Update /etc/hosts
7. **Go API → Tauri**: Return vibespace URL
8. **Tauri**: Open embedded WebView to vibespace-{id}.local

---

### 2.3 Deployment Modes

**vibespace supports two deployment architectures**. See [ADR 0006](adr/0006-bundled-kubernetes-runtime.md) for architectural details.

#### 2.3.1 Local Mode (Current Implementation - MVP)

All components run on local machine with **bundled Kubernetes runtime**:

```
┌─────────────────────┐
│   Desktop App       │  localhost
│   (Tauri)           │
└──────────┬──────────┘
           │ HTTP
┌──────────▼──────────┐
│   API Server        │  localhost:8090
│   (Go)              │
└──────────┬──────────┘
           │ Kubernetes API
┌──────────▼──────────┐
│  Bundled k3s        │  local (Colima/k3s)
│   - Vibespaces      │
│   - BuildKit        │
│   - Registry        │
└─────────────────────┘

All on same machine
```

**Implementation** (ADR 0006):
- macOS: Colima (lightweight VM) + k3s
- Linux: Native k3s binary
- Windows: Not supported (use WSL2 + Linux version)

**Pros**:
- Zero-configuration setup (~3-5 minutes)
- No external dependencies
- No cloud costs
- Fastest performance
- Complete offline capability
- Full data privacy
- Consistent experience across users

**Cons**:
- Limited by local resources
- No remote access without tunneling
- Manual backup required
- Larger app download (~150MB vs ~20MB)

**Status**: Fully implemented in MVP Phase 1

---

#### 2.3.2 Remote Mode (Planned for Post-MVP)

Desktop app local, backend/vibespaces in cloud:

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
│   │  - Vibespaces          │  │
│   │  - BuildKit            │  │
│   │  - Registry            │  │
│   │  - cert-manager        │  │  (for TLS)
│   └────────────────────────┘  │
└────────────────────────────────┘
```

**Implementation** (planned Post-MVP):
- Tauri app does NOT bundle Kubernetes
- User manually provisions VPS with k8s (or uses managed k8s)
- App configured with remote API endpoint (HTTPS)
- `RemoteK8sProvider` trait handles connection, auth, tunneling

**Pros**:
- Access from anywhere
- Unlimited cloud resources
- Automatic backups (if using managed k8s)
- Team collaboration ready

**Cons**:
- Monthly cloud costs (~$50-200/month for VPS)
- Requires internet connection
- Slightly higher latency
- Manual VPS setup required (initially)

**Remote Setup** (initial implementation):
1. User provisions VPS (DigitalOcean, AWS, GCP, etc.)
2. User installs k3s on VPS manually
3. User configures vibespace API server on VPS
4. Desktop app: User enters remote API endpoint URL
5. App saves connection config (endpoint, auth token)
6. Optional: WireGuard tunnel for secure vibespace access
7. Desktop app connects to remote API via HTTPS

**Future Enhancement** (Auto-Provisioning):
1. User provides cloud credentials (AWS Access Key, GCP Service Account)
2. App provisions k3s cluster using Terraform/Pulumi
3. Installs Knative + Traefik + cert-manager automatically
4. Configures WireGuard tunnel for secure access
5. Desktop app connects to `api.yourvibespace.cloud`

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
  domain: vibespace.yourdomain.com
  tunnel:
    type: wireguard
    port: 51820
```

---

#### 2.3.3 Hybrid Mode (Future)

Some vibespaces local, some in cloud:

```
Desktop App → Local k3s (lightweight vibespaces)
           └→ Cloud k3s (heavy vibespaces, team sharing)
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

**Component Versions (MVP - Phase 1)**:
- **Knative Serving**: v1.15.2 (stable, avoids v1.19 OTel transition bugs)
- **Traefik**: v3.5.3 (latest stable ingress controller)
- **Registry**: 2.8.3 (proven for local use)
- **BuildKit**: v0.17.3 (stable, avoids v0.24+ CPU issues)

*Version selection rationale documented in [ADR 0004](../docs/adr/0004-component-version-selection.md). Upgrades to newer versions will be evaluated in Phase 2 after MVP validation.*

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
│   ├── vibespace/                     # Vibespace management feature
│   │   ├── components/
│   │   │   ├── VibespaceList.tsx    # Grid/list view of vibespaces
│   │   │   ├── VibespaceCard.tsx    # Individual vibespace item
│   │   │   ├── VibespaceCreate.tsx  # Creation wizard modal
│   │   │   ├── VibespaceSettings.tsx # Per-vibespace config
│   │   │   └── VibespaceEmbed.tsx   # Embedded code-server iframe
│   │   └── styles/
│   │       ├── VibespaceList.css
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
│   ├── useVibespaces.ts               # React Query vibespace CRUD
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

- **Multi-tab WebView**: Each vibespace opens in a tab
- **Real-time Status**: WebSocket connection for vibespace events
- **Drag-and-drop**: Import Dockerfiles to create templates
- **System Tray**: Quick access, minimizes to tray
- **Native Notifications**: Build complete, vibespace ready, errors

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

*Vibespace Card*:
```tsx
<Card>
  <Status color={running ? 'success' : 'muted'} />
  <Title>{vibespace.name}</Title>
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
│   │   ├── vibespace.go               # Vibespace handlers
│   │   ├── template.go                # Template handlers
│   │   ├── credential.go              # Credential handlers
│   │   ├── cluster.go                 # Cluster handlers
│   │   └── middleware.go              # CORS, logging, auth
│   ├── k8s/
│   │   ├── client.go                  # Kubernetes client (bundled k8s)
│   │   └── context.go                 # Kubeconfig management
│   ├── vibespace/
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
│       ├── vibespace.go               # Vibespace model
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

vibespace:
  namespace: vibespace
  default_resources:
    cpu: "2"
    memory: "4Gi"
    storage: "10Gi"
  dns_domain: local

credential:
  storage_path: ~/.vibespace/credential
  encryption: aes-256-gcm
```

---

### 4.3 Kubernetes Cluster Setup (Local Mode)

#### 4.3.1 Installation Approach (Bundled Kubernetes)

**Implementation** (ADR 0006): **Bundle Kubernetes runtime** with vibespace application

The app **bundles** Kubernetes binaries and provides one-click installation. This approach:
- ✅ Zero-configuration onboarding (~3-5 minutes)
- ✅ Consistent experience across all users
- ✅ No external dependencies or installation guides
- ✅ Matches competitor simplicity (Vercel, Replit, GitHub Codespaces)
- ✅ Predictable behavior (same k8s version for everyone)

**Bundled Components**:
- **macOS**: Colima (~20MB) + Lima (~30MB) + k3s
- **Linux**: k3s binary (~50MB)
- **kubectl**: Kubernetes CLI (~50MB, shared across platforms)
- **Total app size**: ~150MB (vs ~20MB with detection approach)

**Platforms**:
- ✅ **macOS** (Intel + ARM): Colima + Lima VM + k3s
- ✅ **Linux**: Native k3s binary
- ❌ **Windows**: Not supported in Local Mode (use WSL2 + Linux version)

**Installation Flow**:
```typescript
// app/src/hooks/useKubernetesStatus.ts
// Implemented with bundled k8s approach

// 1. Check if bundled k8s is installed
const { status } = useKubernetesStatus();
//   status.installed: bool
//   status.running: bool
//   status.version: string
//   status.is_external: bool (for backward compatibility)

// 2. If not installed, user clicks "Install Kubernetes"
const { install, isInstalling, progress } = useKubernetesInstall();
await install(); // One-click installation

// 3. Progress tracking via events
listen('install-progress', (event) => {
  // event.stage: 'extracting' | 'installing' | 'starting_vm' | 'verifying'
  // event.progress: 0-100
  // event.message: "Starting Colima VM..." etc.
});

// 4. Kubernetes starts automatically after installation
// 5. Components installed (Knative, Traefik, etc.)
// 6. Ready to create vibespaces
```

**Binary Bundling**:

Binaries downloaded during build (not committed to git):

```bash
# macOS
app/src-tauri/binaries/macos/colima         # ~20MB
app/src-tauri/binaries/macos/limactl        # ~30MB
app/src-tauri/binaries/kubectl-darwin-amd64 # ~50MB

# Linux
app/src-tauri/binaries/linux/k3s            # ~50MB
app/src-tauri/binaries/kubectl-linux-amd64  # ~50MB
```

Configured in `tauri.conf.json`:
```json
{
  "bundle": {
    "resources": {
      "binaries/macos/*": "binaries/macos/",
      "binaries/linux/*": "binaries/linux/"
    },
    "externalBin": [
      "binaries/macos/colima",
      "binaries/macos/limactl",
      "binaries/linux/k3s",
      "binaries/kubectl-darwin-amd64",
      "binaries/kubectl-linux-amd64"
    ]
  }
}
```

**Platform-Specific Installation**:

**macOS** (Colima + k3s):
```bash
# Automated by app - user clicks "Install Kubernetes"
# 1. Extract colima, limactl, kubectl binaries to ~/.vibespace/bin/
# 2. Start Colima with k3s:
colima start --kubernetes --cpu 2 --memory 4 --disk 10
# 3. Kubeconfig: ~/.colima/default/kubeconfig.yaml
# 4. Verify cluster ready
# 5. Install components (Knative, Traefik, etc.)
```

**Linux** (native k3s):
```bash
# Automated by app - user clicks "Install Kubernetes"
# 1. Extract k3s, kubectl binaries to ~/.vibespace/bin/
# 2. Start k3s server:
k3s server --data-dir ~/.vibespace/k3s-data \
  --write-kubeconfig ~/.kube/config \
  --write-kubeconfig-mode 644 \
  --disable traefik
# 3. Verify cluster ready
# 4. Install components (Knative, Traefik, etc.)
```

**Windows**:
Not supported in Local Mode. Use WSL2 + Linux version for bundled k8s experience.

# Alternative: WSL2 + k3s
# Install WSL2, then run k3s inside Linux
```

**Future (Phase 3)**: Full bundling with VM/k3s binaries for zero-config installation. See GitHub Issue #15.

#### 4.3.2 Recommended k3s Configuration

When users install k3s manually, these are the recommended settings:

```bash
# Minimal k3s setup for Vibespace
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

Once Kubernetes is installed (via one-click bundled installation), the following components are installed:

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

**Note**: This setup is automated by the app once Kubernetes is installed.

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

## 5. Vibespace Specifications

### 5.1 Vibespace Resource

```yaml
# Example: Knative Service for a Next.js vibespace
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: vibespace-abc123
  namespace: vibespace
  labels:
    vibespace.dev/template: nextjs
    vibespace.dev/owner: user
    vibespace.dev/persistent: "true"
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
      - name: vibespace
        image: localhost:5000/vibespace-nextjs:latest
        ports:
        - containerPort: 8080
          name: http1
        env:
        - name: VIBESPACE_ID
          value: "abc123"
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: vibespace-abc123-secrets
              key: claude-api-key
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: vibespace-abc123-secrets
              key: openai-api-key
        - name: GIT_AUTHOR_NAME
          valueFrom:
            secretKeyRef:
              name: vibespace-abc123-secrets
              key: git-author-name
        - name: GIT_AUTHOR_EMAIL
          valueFrom:
            secretKeyRef:
              name: vibespace-abc123-secrets
              key: git-author-email
        - name: GIT_COMMITTER_NAME
          valueFrom:
            secretKeyRef:
              name: vibespace-abc123-secrets
              key: git-author-name
        - name: GIT_COMMITTER_EMAIL
          valueFrom:
            secretKeyRef:
              name: vibespace-abc123-secrets
              key: git-author-email
        resources:
          requests:
            cpu: "1"
            memory: "2Gi"
          limits:
            cpu: "2"
            memory: "4Gi"
        volumeMounts:
        # Vibespace data (persistent)
        - name: vibespace-data
          mountPath: /vibespace
        # SSH keys from app-managed secrets
        - name: ssh-keys
          mountPath: /home/coder/.ssh
          readOnly: true
      volumes:
      - name: vibespace-data
        persistentVolumeClaim:
          claimName: vibespace-abc123-pvc
      - name: ssh-keys
        secret:
          secretName: vibespace-abc123-ssh
          defaultMode: 0600
          optional: true  # SSH keys are optional
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: vibespace-abc123-pvc
  namespace: vibespace
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
  name: vibespace-abc123-secrets
  namespace: vibespace
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
  name: vibespace-abc123-ssh
  namespace: vibespace
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
  name: vibespace-abc123
  namespace: vibespace
spec:
  entryPoints:
  - web
  routes:
  - match: Host(`vibespace-abc123.local`)
    kind: Rule
    services:
    - name: vibespace-abc123
      port: 80
      kind: Service
```

### 5.3 Vibespace Lifecycle

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

#### 6.1.1 Vibespaces

```
POST   /vibespaces
GET    /vibespaces
GET    /vibespaces/:id
PUT    /vibespaces/:id
DELETE /vibespaces/:id
POST   /vibespaces/:id/start
POST   /vibespaces/:id/stop
GET    /vibespaces/:id/logs
GET    /vibespaces/:id/status
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

#### Create Vibespace

```http
POST /api/v1/vibespaces
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
    "allowInterVibespace": false
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
    "codeServer": "http://vibespace-abc123.local",
    "app": "http://vibespace-abc123-3000.local"
  },
  "createdAt": "2025-10-07T10:30:00Z",
  "resources": {
    "cpu": "2",
    "memory": "4Gi",
    "storage": "20Gi"
  }
}
```

#### List Vibespaces

```http
GET /api/v1/vibespaces
```

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "vibespaces": [
    {
      "id": "abc123",
      "name": "my-nextjs-project",
      "template": "nextjs",
      "status": "running",
      "urls": {
        "codeServer": "http://vibespace-abc123.local",
        "app": "http://vibespace-abc123-3000.local"
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
        "jupyter": "http://vibespace-def456.local"
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
  "baseImage": "localhost:5000/vibespace-base:latest",
  "dockerfile": "FROM localhost:5000/vibespace-base:latest\n\nRUN apt-get update && apt-get install -y python3.11 python3-pip\n\nRUN pip3 install django djangorestframework\n\nUSER coder\nWORKDIR /vibespace\n\nCMD [\"code-server\", \"--bind-addr\", \"0.0.0.0:8080\", \"--auth\", \"none\"]",
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
data: {"step": 1, "message": "FROM localhost:5000/vibespace-base:latest", "timestamp": "2025-10-07T10:35:01Z"}

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
  "comment": "john@vibespace"
}
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{
  "id": "cred-ssh123",
  "name": "GitHub SSH Key",
  "publicKey": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJqfH... john@vibespace",
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
  "details": "Vibespace with name 'my-project' already exists",
  "code": "VIBESPACE_EXISTS"
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

Users authenticate directly within the app. Credentials are stored securely and injected into vibespaces as needed.

**Supported credential types**:
```
- AI Agents (Claude, OpenAI, etc.)
- Git (username, email, SSH keys)
- Docker registries
- Cloud providers (AWS, GCP, Azure)
- Package managers (npm, PyPI)
```

**Storage**:
- **Location**: App data directory (`~/.vibespace/credentials/`)
- **Encryption**: AES-256 at rest, keys managed by OS keychain (Tauri secure storage)
- **Isolation**: Per-vibespace credential assignment (opt-in)

#### 7.1.2 Credential Injection Strategy

Credentials are injected into vibespaces via **Kubernetes Secrets**, not host volume mounts:

```yaml
env:
- name: ANTHROPIC_API_KEY
  valueFrom:
    secretKeyRef:
      name: vibespace-abc123-secrets
      key: claude-api-key
- name: OPENAI_API_KEY
  valueFrom:
    secretKeyRef:
      name: vibespace-abc123-secrets
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
    secretName: vibespace-abc123-ssh
    defaultMode: 0600
```

**Security considerations**:
- No direct host filesystem access
- Credentials stored encrypted in app storage
- Kubernetes Secrets created per-vibespace
- Automatic cleanup on vibespace deletion
- User controls which vibespaces get which credentials

#### 7.1.3 AI Agent Authentication

**Flow**:
1. User opens Settings → Credentials in app
2. Clicks "Add Credential" → "AI Agent"
3. Selects provider (Claude, OpenAI, etc.)
4. Enters API key OR initiates OAuth flow
5. Credential saved to encrypted app storage
6. When creating vibespace, user selects which agents to enable
7. Backend creates Kubernetes Secret with selected credentials
8. Environment variables available in vibespace

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

#### 7.2.1 Vibespace Isolation

By default, vibespaces are **network-isolated**:

```yaml
# NetworkPolicy: Deny all inter-vibespace traffic by default
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vibespace-isolation
  namespace: vibespace
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
  # Allow internet (but not other vibespaces)
  - to:
    - namespaceSelector:
        matchExpressions:
        - key: name
          operator: NotIn
          values:
          - vibespaces
```

#### 7.2.2 Inter-Vibespace Communication (Opt-in)

When enabled via vibespace settings:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vibespace-abc123-allow-def456
  namespace: vibespace
spec:
  podSelector:
    matchLabels:
      vibespace.dev/id: abc123
  ingress:
  - from:
    - podSelector:
        matchLabels:
          vibespace.dev/id: def456
```

Access via: `vibespace-def456.vibespaces.svc.cluster.local`

---

## 8. Networking

### 8.1 Local DNS

> **Note**: Local DNS configuration (*.local domains) is implemented in **MVP Phase 2** alongside Traefik IngressRoute integration. MVP Phase 1 uses `kubectl port-forward` with `127.0.0.1:PORT` URLs. See Section 8.2.1 for vibespace access strategy.

#### 8.1.1 Strategy (MVP Phase 2)

**Primary**: `/etc/hosts` manipulation (requires sudo once)

```bash
# Automatically added by backend (Phase 2)
127.0.0.1  vibespace-abc123.local
127.0.0.1  vibespace-abc123-3000.local
127.0.0.1  vibespace-def456.local
```

**Alternative**: dnsmasq (for wildcard support)

```bash
# /etc/dnsmasq.conf
address=/vibespace.local/127.0.0.1
```

#### 8.1.2 Domain Structure

- **Code Server**: `vibespace-{id}.local` → port 8080
- **App Port 1**: `vibespace-{id}-3000.local` → port 3000
- **App Port 2**: `vibespace-{id}-8000.local` → port 8000
- **Custom**: `vibespace-{id}-{port}.local` → port {port}

#### 8.1.3 Traefik Routing (MVP Phase 2)

When Knative Services are introduced in Phase 2, vibespace will be accessed via Traefik IngressRoutes with custom *.local domains:

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-abc123-codeserver
spec:
  entryPoints: [web]
  routes:
  - match: Host(`vibespace-abc123.local`)
    kind: Rule
    services:
    - name: vibespace-abc123
      port: 80
---
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-abc123-app-3000
spec:
  entryPoints: [web]
  routes:
  - match: Host(`vibespace-abc123-3000.local`)
    kind: Rule
    services:
    - name: vibespace-abc123
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

### 8.2 Vibespace Access Strategy

#### 8.2.1 Overview: Staged Migration Approach

Vibespace access has **completed migration** to production-ready architecture:

| Phase | Vibespace Type | Access Method | URLs | Status |
|-------|---------------|---------------|------|--------|
| **MVP Phase 1 (Initial)** | Kubernetes Pods | `kubectl port-forward` | `http://127.0.0.1:8080+N` | ⚠️ **Legacy** |
| **MVP Phase 1 (Current)** | Knative Services | Traefik IngressRoutes + DNS | `http://{subdomain}.{project}.vibe.space` | ✅ **Complete** |

**Migration Completed** (Issue #52):
- Vibespaces now run as **Knative Services** with scale-to-zero capability
- **3 subdomains per vibespace**: `code.{project}.vibe.space`, `preview.{project}.vibe.space`, `prod.{project}.vibe.space`
- **Custom DNS resolution** via bundled dnsd server (miekg/dns on port 5353)
- **Dual-mode operation**: Knative mode (default), Pod mode (legacy fallback via `ENABLE_KNATIVE_ROUTING=false`)
- **Project names**: DNS-friendly identifiers (e.g., `brave-eagle-7421`) auto-generated with uniqueness check

**Benefits of Migration**:
- ✅ Scale-to-zero when idle (minScale=0)
- ✅ Multi-port routing (code/preview/prod on single vibespace)
- ✅ Human-readable URLs instead of localhost ports
- ✅ Seamless integration with Knative lifecycle
- ✅ Backward compatible via feature flags

---

#### 8.2.2 MVP Phase 1: Port-Forward to Pods

**Current Implementation** (as of MVP Phase 1):

Vibespaces run as plain Kubernetes Pods. The API server starts a `kubectl port-forward` when a user clicks "Open Vibespace":

```go
// API: GET /api/v1/vibespaces/:id/access
// Returns: { "url": "http://127.0.0.1:8081" }

// Backend implementation (api/pkg/vibespace/service.go)
func (s *Service) Access(ctx context.Context, id string) (string, error) {
    podName := fmt.Sprintf("vibespace-%s", id)

    // Assign consistent local port: 8080 + hash(id) mod 1000
    localPort := 8080 + hashStringToPort(id)

    // Start port-forward to pod's code-server (port 8080)
    err := s.k8sClient.StartPortForwardToPod(ctx, "vibespace", podName, localPort, 8080)
    if err != nil {
        return "", fmt.Errorf("failed to start port-forward: %w", err)
    }

    return fmt.Sprintf("http://127.0.0.1:%d", localPort), nil
}
```

**How It Works**:
1. User clicks "Open" in UI
2. Frontend calls `GET /api/v1/vibespaces/:id/access`
3. Backend starts `kubectl port-forward pod/vibespace-{id} {localPort}:8080`
4. Returns `http://127.0.0.1:{localPort}`
5. User opens URL in browser → code-server loads

**Port Assignment**:
- Each vibespace gets a consistent local port: `8080 + hash(vibespaceID) % 1000`
- Range: `8080-9079` (1000 ports)
- Same vibespace always gets same port (idempotent)

**Limitations**:
- ⚠️ URLs not human-friendly (`http://127.0.0.1:8142` instead of `http://my-project.local`)
- ⚠️ Port-forward process runs for lifetime of API server
- ⚠️ Multiple vibespaces = multiple port-forward processes
- ⚠️ No automatic cleanup if API server crashes (manual `kubectl delete pod`)

**Accepted for Phase 1** because:
- ✅ Simple implementation (no DNS, no IngressRoutes)
- ✅ Works immediately (no sudo for /etc/hosts)
- ✅ Fast MVP delivery
- ✅ Easy to debug (`lsof -i :8080` to find port-forwards)

---

#### 8.2.3 MVP Phase 2: Traefik IngressRoutes + Knative

**Future Implementation** (planned for Phase 2 with Knative Services):

When vibespaces migrate to Knative Services, access will use Traefik IngressRoutes with custom *.local domains:

```yaml
# Created automatically when vibespace is created
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-abc123
  namespace: vibespace
spec:
  entryPoints: [web]
  routes:
  - match: Host(`vibespace-abc123.local`)
    kind: Rule
    services:
    - name: vibespace-abc123  # Knative Service name
      port: 80
```

```go
// Phase 2 API: GET /api/v1/vibespaces/:id/access
// Returns: { "url": "http://vibespace-abc123.local" }

func (s *Service) Access(ctx context.Context, id string) (string, error) {
    // Phase 2: Create IngressRoute + /etc/hosts entry
    vibespaceDomain := fmt.Sprintf("vibespace-%s.local", id)

    // 1. Create IngressRoute (if not exists)
    if err := s.createIngressRoute(ctx, id, vibespaceDomain); err != nil {
        return "", err
    }

    // 2. Add /etc/hosts entry: 127.0.0.1 vibespace-abc123.local
    if err := s.updateHostsFile(vibespaceDomain); err != nil {
        return "", err
    }

    // 3. Return clean URL
    return fmt.Sprintf("http://%s", vibespaceDomain), nil
}
```

**How It Works** (Phase 2):
1. User clicks "Open" in UI
2. Backend creates IngressRoute (if needed)
3. Backend adds `/etc/hosts` entry (requires sudo once)
4. Returns `http://vibespace-abc123.local`
5. User opens URL → DNS resolves to 127.0.0.1 → Traefik routes to Knative Service → code-server loads

**Benefits** (Phase 2):
- ✅ Clean URLs (`http://my-project.local`)
- ✅ Works with Knative scale-from-zero (Traefik waits for pod to start)
- ✅ Multiple ports supported (`vibespace-abc123-3000.local` for app ports)
- ✅ Production-ready architecture
- ✅ No long-running port-forward processes

**Migration Path**:
```
Phase 1 (Current)                Phase 2 (Knative)
-----------------                -----------------
Pod: vibespace-{id}       →      Knative Service: vibespace-{id}
kubectl port-forward      →      Traefik IngressRoute
http://127.0.0.1:8080+N  →      http://vibespace-{id}.local
```

When Phase 2 migration happens:
1. Update `api/pkg/vibespace/service.go::Access()` to create IngressRoutes
2. Add `/etc/hosts` management (with sudo prompt)
3. Update frontend to handle `.local` URLs
4. Remove port-forward cleanup logic
5. Update tests

---

#### 8.2.4 Dynamic Port Exposure (MVP Phase 2)

In Phase 2, users can expose additional vibespace ports at runtime:

```http
POST /api/v1/vibespaces/abc123/expose
{
  "port": 5432,
  "protocol": "tcp"
}
```

Backend creates:
1. New IngressRoute for `vibespace-abc123-5432.local`
2. Updates `/etc/hosts`
3. Returns URL: `http://vibespace-abc123-5432.local`

**Use Cases**:
- Expose dev server (`:3000` for Next.js, `:8000` for Python)
- Expose database (`:5432` for PostgreSQL)
- Expose API server (`:8080` for custom backend)

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

When vibespace is created in cloud mode:

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: vibespace-abc123-cert
  namespace: vibespace
spec:
  secretName: vibespace-abc123-tls
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
  dnsNames:
  - vibespace-abc123.yourdomain.com
  - myproject.example.com  # custom domain
```

**IngressRoute with TLS**:
```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-abc123
  namespace: vibespace
spec:
  entryPoints:
  - websecure
  routes:
  - match: Host(`vibespace-abc123.yourdomain.com`)
    kind: Rule
    services:
    - name: vibespace-abc123
      port: 80
  tls:
    secretName: vibespace-abc123-tls
```

**Auto-renewal**: cert-manager handles renewal 30 days before expiry.

---

### 8.4 Custom Domains

#### 8.4.1 Domain Management

Users can assign custom domains to vibespaces:

**UI Flow**:
```
Vibespace Settings → Domains
├─ Default: vibespace-abc123.yourdomain.com
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
POST /api/v1/vibespaces/abc123/domains
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
   CNAME: myproject.example.com → vibespace.yourdomain.com
   ```
3. Updates IngressRoute with new domain
4. cert-manager automatically provisions certificate
5. Vibespace accessible via custom domain over HTTPS

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

One vibespace can have multiple domains:

```yaml
apiVersion: traefik.containo.us/v1alpha1
kind: IngressRoute
metadata:
  name: vibespace-abc123
spec:
  entryPoints: [websecure]
  routes:
  - match: Host(`vibespace-abc123.yourdomain.com`) || Host(`myproject.example.com`) || Host(`app.custom.dev`)
    kind: Rule
    services:
    - name: vibespace-abc123
      port: 80
  tls:
    secretName: vibespace-abc123-tls
    domains:
    - main: vibespace-abc123.yourdomain.com
    - main: myproject.example.com
    - main: app.custom.dev
```

**Certificate for Multiple Domains**:
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: vibespace-abc123-cert
spec:
  secretName: vibespace-abc123-tls
  dnsNames:
  - vibespace-abc123.yourdomain.com
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
  - "*.vibespace.yourdomain.com"
  issuerRef:
    name: letsencrypt-prod
    kind: ClusterIssuer
```

All vibespaces can use:
- `vibespace-abc123.vibespace.yourdomain.com`
- `vibespace-def456.vibespace.yourdomain.com`
- etc.

---

## 9. Template System

### 9.0 Template Definition

**What is a Template?**

A template is a **complete vibespace configuration**, not just a development stack. When a user creates a vibespace from a template, they're selecting:

1. **Base Development Stack**: Next.js, Python, Vue, Jupyter, etc.
2. **AI Coding Agent**: Claude Code, OpenAI Codex, Cursor, or custom agent
3. **Agent Instructions**: CLAUDE.md or agent.md file with project-specific context
4. **Git Repository** (optional): Clone existing repo or start fresh
5. **Resource Limits**: Default CPU/memory allocation

**Phase 1 (MVP)**: Single agent baked into vibespace container
**Phase 2**: Multiple agents as sidecars (see Section 9.3)

**Example**: A "Next.js + Claude Code" template includes:
- Next.js 14 + TypeScript + Tailwind (stack)
- Claude Code CLI pre-installed (agent)
- CLAUDE.md with Next.js best practices (instructions)
- Option to clone user's GitHub repo (repository)

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
WORKDIR /vibespace

# Pre-configure code-server
RUN mkdir -p ~/.config/code-server && \
    echo "bind-addr: 0.0.0.0:8080" > ~/.config/code-server/config.yaml && \
    echo "auth: none" >> ~/.config/code-server/config.yaml && \
    echo "cert: false" >> ~/.config/code-server/config.yaml

EXPOSE 8080

CMD ["code-server", "/vibespace"]
```

#### 9.1.2 Next.js Template

**Location**: `images/templates/nextjs/Dockerfile`

```dockerfile
FROM localhost:5000/vibespace-base:latest

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
if [ ! -f /vibespace/.initialized ]; then\n\
  echo "Initializing Next.js vibespace..."\n\
  tar xzf ~/next-template.tar.gz -C /vibespace --strip-components=1\n\
  cd /vibespace && pnpm install\n\
  touch /vibespace/.initialized\n\
fi' > /home/coder/init.sh && chmod +x /home/coder/init.sh

CMD ["/bin/bash", "-c", "~/init.sh && code-server /vibespace"]
```

#### 9.1.3 Vue Template

**Location**: `images/templates/vue/Dockerfile`

```dockerfile
FROM localhost:5000/vibespace-base:latest

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
if [ ! -f /vibespace/.initialized ]; then\n\
  echo "Initializing Vue vibespace..."\n\
  tar xzf ~/vue-template.tar.gz -C /vibespace --strip-components=1\n\
  cd /vibespace && pnpm install\n\
  touch /vibespace/.initialized\n\
fi' > /home/coder/init.sh && chmod +x /home/coder/init.sh

CMD ["/bin/bash", "-c", "~/init.sh && code-server /vibespace"]
```

#### 9.1.4 Jupyter Template

**Location**: `images/templates/jupyter/Dockerfile`

```dockerfile
FROM localhost:5000/vibespace-base:latest

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
CMD ["/bin/bash", "-c", "jupyter lab --no-browser &>/dev/null & code-server /vibespace"]
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
  "baseImage": "localhost:5000/vibespace-base:latest",
  "dockerfile": "...",
  "defaultPorts": [8000, 5432],
  "agent": {
    "type": "claude-code",
    "version": "latest",
    "instructionsFile": "CLAUDE.md",
    "preInstalled": true
  },
  "repository": {
    "type": "git",
    "url": "",
    "branch": "main",
    "optional": true
  },
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

### 9.3 Multi-Agent Architecture (Phase 2)

#### 9.3.1 Overview

**Phase 1**: Single agent baked into vibespace container
**Phase 2**: Multiple agents running as Kubernetes sidecars

**Why Sidecars?**
- **Isolation**: Each agent has dedicated resources (CPU, memory)
- **Parallelism**: Multiple agents work simultaneously on different tasks
- **Independence**: Agent crashes don't affect vibespace or other agents
- **Flexibility**: Add/remove agents without rebuilding vibespace image

#### 9.3.2 Pod Architecture

```yaml
Pod: vibespace-abc123
├── vibespace (main container)
│   ├── code-server (VS Code in browser)
│   ├── /vibespace (shared volume)
│   └── agent CLI (for switching between agents)
│
├── frontend-agent (sidecar)
│   ├── Claude Code / OpenAI Codex
│   ├── Shell server (for terminal access)
│   └── /vibespace (shared volume, read-write)
│
├── backend-agent (sidecar)
│   ├── Claude Code / OpenAI Codex
│   ├── Shell server (for terminal access)
│   └── /vibespace (shared volume, read-write)
│
└── test-agent (sidecar)
    ├── Claude Code / OpenAI Codex
    ├── Shell server (for terminal access)
    └── /vibespace (shared volume, read-write)
```

**All containers share**: `/vibespace` volume (PVC)

#### 9.3.3 Terminal-Based Interaction

Users interact with agents via terminal commands in code-server:

```bash
# List available agents
$ agent list
Available agents:
  ● frontend-agent (ready)
  ● backend-agent (ready)
  ○ test-agent (not running)

# Switch to agent's shell
$ agent use frontend-agent
Connecting to frontend-agent...

# Now in frontend-agent sidecar shell
frontend-agent@sidecar:~$ pwd
/vibespace

frontend-agent@sidecar:~$ ls
components/  pages/  package.json

frontend-agent@sidecar:~$ # Work with the agent
frontend-agent@sidecar:~$ exit
Disconnected from frontend-agent

# Back in main vibespace shell
$
```

**Key Features**:
- Direct shell access to agent sidecars
- No custom UI needed - terminal-first approach
- Agents have full vibespace filesystem access
- Simple switching: `agent use <name>`
- Each agent has its own environment and tools

#### 9.3.4 Use Cases

**Parallel Development**:
- Frontend agent: Building UI components
- Backend agent: Creating API endpoints
- Test agent: Writing tests
- All working simultaneously on shared codebase

**Specialized Roles**:
- Code review agent: Analyzing PRs
- Documentation agent: Writing docs
- Refactoring agent: Improving code quality
- Security agent: Finding vulnerabilities

### 9.4 Template Sharing (Future)

#### 9.4.1 Export

```bash
# CLI command (future)
$ vibespace template export my-django-template

# Creates:
my-django-template.tar.gz
├── Dockerfile
├── template.json  (metadata)
└── files/         (optional: init scripts, config files)
```

#### 9.4.2 Import

```bash
# CLI command
$ vibespace template import my-django-template.tar.gz

# Or via UI: drag-and-drop .tar.gz file
```

#### 9.3.3 Marketplace (Future)

- GitHub repo: `vibespace/templates`
- Each template is a directory with Dockerfile + metadata
- UI fetches index.json, shows gallery
- One-click install builds from GitHub

---

## 10. Implementation Milestones

**Note:** These are internal development milestones for **MVP Phase 1** (see ROADMAP.md for full product phases). MVP Phase 1 corresponds to ROADMAP "Foundation" - proving core vibespace management works locally before adding production features.

**Implementation Order** (SPEC.md Section 10) tracks **how** we build, while **Product Phases** (ROADMAP.md) track **what** we release to users.

---

### Milestone 1: Infrastructure ✅ COMPLETE (Week 1)

**Goal**: Core platform infrastructure operational

#### Completed:
- [x] **Project scaffolding**
  - Tauri 2.x desktop app with React 19 + TypeScript UI
  - Go 1.21+ API server with Gin framework
  - Rust backend for Tauri native functionality
  - Frontend component organization (ADR 0003)
- [x] **Kubernetes manifests** (api/pkg/k8s/manifests/:2556-2755)
  - Knative Serving v1.15.2 (CRDs: 6,536 lines, Core: 9,310 lines)
  - Traefik v3.5.3 ingress controller (94 lines)
  - Registry 2.8.3 local image storage (55 lines)
  - BuildKit v0.17.3 container builder (45 lines)
  - All manifests embedded via `go:embed`
  - Component versions (ADR 0004)
- [x] **Cluster installation & setup** (app/src/components/setup/components/KubernetesSetup.tsx:1-628)
  - One-click bundled Kubernetes installation (Colima on macOS, k3s on Linux)
  - Real-time installation progress with SSE streaming
  - Auto-installs cluster components: Knative v1.15.2, Traefik v3.5.3, Registry 2.8.3, BuildKit v0.17.3
  - Trait-based K8sProvider architecture for Local/Remote modes (ADR 0006)
  - Full frontend → backend integration (API_ENDPOINTS.clusterStatus, clusterSetup)
- [x] **API server** (api/cmd/server/main.go:1-97)
  - Vibespace CRUD endpoints (GET/POST/DELETE /api/v1/vibespaces)
  - Cluster management endpoints (GET/POST /api/v1/cluster/status, /setup)
  - SSE streaming for installation and setup progress
- [x] **Frontend UI**
  - Setup wizard flow (AuthenticationSetup → KubernetesSetup → ConfigurationSetup)
  - Vibespace list with status polling (app/src/hooks/useVibespaces.ts:1-238)
  - Full integration with backend APIs (app/src/lib/api-config.ts:1-54)

**Status**: ~95% of MVP Phase 1 complete. Infrastructure, UI foundation, and Knative migration complete. Only credential management backend remaining.

---

### Milestone 2: Core Functionality ⏳ IN PROGRESS (Week 2-3)

**Goal**: Complete vibespace creation with AI agents and real images

#### Completed:
- [x] **Docker images with AI agents** (api/pkg/template/images/):
  - [x] **Base images** (base-claude, base-codex, base-gemini)
    - ✅ code-server 4.104.3+ installed
    - ✅ Claude Code CLI / OpenAI Codex / Gemini CLI installed
    - ✅ CLAUDE.md / AGENT.md instruction files
    - ✅ Agent auto-start configured via init-agents.sh
    - ✅ Custom VS Code theme matching design system
  - [x] **Next.js template** (templates/nextjs/Dockerfile)
    - ✅ Next.js 15.5.5 + TypeScript + Tailwind + pnpm 10.18.3
    - ✅ VS Code extensions (ESLint, Prettier, Tailwind CSS)
    - ✅ AI agent integration (multi-agent support via ARG)
    - ✅ Next.js-specific CLAUDE.md instructions
  - [x] **Vue template** (templates/vue/Dockerfile)
    - ✅ Vue 3 + Vite + TypeScript
    - ✅ VS Code Vue extensions
    - ✅ AI agent integration
  - [x] **Jupyter template** (templates/jupyter/Dockerfile)
    - ✅ Python 3.11 + Jupyter Lab
    - ✅ Data science libraries (numpy, pandas, matplotlib, scikit-learn, seaborn)
    - ✅ AI agent integration
- [x] **Vibespace implementation with real images** (api/pkg/vibespace/service.go:220)
  - ✅ Uses real vibespace images: `localhost:30500/vibespace-{template}-{agent}:latest`
  - ✅ Dynamic container ports based on template (code-server:8080, preview ports per template)
  - ✅ PVC mounting at /vibespace with proper permissions
  - ✅ Init containers for git clone and permission fixes
  - ✅ Security context with non-root user (UID 1001)
- [x] **Knative + Traefik + DNS integration** (Issue #52, PR #53):
  - ✅ Knative Services replace Pods for scale-to-zero capability
  - ✅ Traefik IngressRoutes for multi-port routing (code/preview/prod subdomains)
  - ✅ Custom DNS resolution (miekg/dns server on port 5353, *.vibe.space wildcard)
  - ✅ Project name generation with uniqueness check (e.g., `brave-eagle-7421`)
  - ✅ All CRUD operations refactored (Create/List/Get/Delete/Start/Stop use Knative)
  - ✅ Dual-mode operation via feature flags (Knative default, Pod fallback)
  - ✅ PatchService for scaling operations (minScale 0↔1)
  - ✅ Helper functions for Knative→Vibespace conversion
  - ✅ URL structure: `{code,preview,prod}.{project}.vibe.space`

#### In Progress:
- [ ] **Credential management backend**:
  - [ ] API handler (api/pkg/handler/credential.go) - CREATE
  - [ ] Service logic (api/pkg/credential/service.go) - CREATE
  - [ ] Tauri secure storage integration
  - [ ] API endpoints: GET/POST/PUT/DELETE /api/v1/credentials
- [ ] **Kubernetes Secret generation**:
  - [ ] Create secrets from app credentials
  - [ ] Environment variable injection (ANTHROPIC_API_KEY, OPENAI_API_KEY)
  - [ ] Git config injection (.gitconfig)
  - [ ] SSH key volume mounts (read-only)

#### Blocked By:
- Credential backend needed before secrets can be injected

**Target**: End of Week 3 - vibespaces launch with AI agents, persistent storage, and credentials

**Note**: Docker image building is fully automated via BuildAllTemplates during cluster setup (api/pkg/k8s/setup.go:462)

---

### Milestone 3: Integration & Testing 🔮 PLANNED (Week 3)

**Goal**: Production-ready MVP for beta release

#### Planned:
- [ ] **Testing**:
  - [ ] Backend unit tests (Go packages)
  - [ ] Frontend component tests (Vitest + React Testing Library)
  - [ ] Integration tests (API + k3s interaction)
  - [ ] E2E tests (full vibespace lifecycle: create → open → delete)
  - [ ] Manual QA checklist
- [ ] **Error handling**:
  - [ ] User-friendly error messages
  - [ ] Automatic retries for transient failures
  - [ ] "Recover" actions for failed vibespaces
  - [ ] Graceful degradation when cluster unavailable
- [ ] **Documentation finalization**:
  - [ ] Update README with accurate feature status
  - [ ] API documentation (endpoints, request/response formats)
  - [ ] Troubleshooting guide (common issues, kubectl commands)
  - [ ] Architecture diagrams
- [ ] **Build & packaging**:
  - [ ] Tauri app builds (.dmg for macOS, .exe for Windows, .deb for Linux)
  - [ ] Docker images pushed to local registry
  - [ ] Release artifacts uploaded to GitHub Releases
- [ ] **Beta validation**:
  - [ ] 10 beta users can create and use vibespaces
  - [ ] AI agents work successfully (Claude Code, OpenAI Codex)
  - [ ] < 5 minutes from download to first vibespace
  - [ ] Positive feedback on core vibespace management

**Target**: End of Week 3 - Alpha release to 10 beta testers

**Success Criteria (MVP Phase 1)**:
- ✅ Kubernetes cluster installed via one-click bundled setup
- ✅ All infrastructure components installed (Knative, Traefik, Registry, BuildKit)
- ✅ Users can create vibespaces from templates (Next.js, Vue, Jupyter)
- ✅ AI coding agents pre-configured and working (Claude Code, OpenAI Codex)
- ✅ Vibespaces have persistent storage (PVCs)
- ✅ Code accessible via port-forward to localhost:8080
- ✅ < 5 minutes from app launch to first vibespace open

---

**What Comes After MVP Phase 1?**

See ROADMAP.md for post-MVP features:
- **MVP Phase 2** (Weeks 4-8): Knative scale-to-zero, custom template builder, multi-agent sidecars, cloud deployment (AWS/GCP/DigitalOcean), TLS certificates, custom domains
- **Post-MVP** (Month 3+): Bundled Kubernetes (zero-config), template marketplace, CI/CD integration, monitoring, teams, enterprise features (SSO, audit logs, GitOps)

---

## 11. User Flows

### 11.1 First-Time Setup

#### MVP Flow (Phase 1)

```
1. User downloads and launches Vibespace

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
       │ Vibespace needs Kubernetes to run.          │
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
   ├─> "Your local vibespace cluster is ready!"
   │
   └─> [Create your first vibespace]
       [Set up credentials (optional)]
```

#### Future Flow (Phase 3 - Full Bundling)

```
1. User downloads and launches Vibespace

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
       Comment: [john@vibespace]

       [Generate] [Cancel]

5. Key generated
   ├─> Shows public key for copying
   │   ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA...
   │   [Copy] [Add to GitHub] [Add to GitLab]
   │
   └─> Private key stored encrypted in app

6. Credentials ready
   └─> Now available in vibespace creation flow
```

### 11.2 Create Vibespace (Built-in Template)

```
1. Click "New Vibespace"

2. Modal opens: "Create Vibespace"
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
      ☐ Allow inter-vibespace networking
      Expose ports: [3000, 3001        ]
      Environment variables:
        NODE_ENV=development

3. Click "Create"
   ├─> Progress indicator
   │   "Creating vibespace..."
   │   "Pulling image..."         ✓
   │   "Creating volumes..."      ✓
   │   "Starting container..."    ...
   │
4. Vibespace ready!
   ├─> Card appears in vibespace list
   │   Status: 🟢 Running
   │   URLs:
   │     Code: vibespace-abc123.local
   │     App:  vibespace-abc123-3000.local
   │
   └─> [Open in App] [Open in Browser] [Settings]
```

### 11.3 Create Vibespace (Custom Template)

```
1. Click "New Vibespace" → "Custom Template"

2. Template Builder
   ├─ Name: [my-django-template]
   ├─ Icon: [🐍 ▼]
   ├─ Base: [vibespace-base ▼]
   │
   └─ Dockerfile:
      ┌─────────────────────────────────────┐
      │ FROM localhost:5000/vibespace-base  │
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
   │   [Step 1/4] FROM localhost:5000/vibespace-base
   │   [Step 2/4] RUN apt-get update
   │     Reading package lists...
   │   [Step 3/4] RUN pip3 install django
   │     Collecting django...
   │   [Step 4/4] Build complete ✓
   │
4. Template saved
   └─> Now appears in template gallery

5. Create vibespace from it (same flow as 11.2)
```

### 11.4 Daily Usage

```
1. User opens Vibespace app

2. Dashboard shows existing vibespaces
   ┌─────────────────────────────────────┐
   │ my-nextjs-project         🟢 Running │
   │ Last accessed: 2 hours ago          │
   │ [Open] [Stop] [Settings]            │
   ├─────────────────────────────────────┤
   │ data-analysis             ⚪ Stopped │
   │ Last accessed: yesterday            │
   │ [Start] [Delete]                    │
   └─────────────────────────────────────┘

3. Click "Open" on running vibespace
   ├─> New tab opens in app with embedded code-server
   │   Multiple tabs can be open simultaneously
   │
   └─> User codes with Claude Code assistant
       (No auth required, uses host credentials)

4. Click "Start" on stopped vibespace
   ├─> Knative scales pod from 0 → 1
   │   Progress: "Starting vibespace... (30s)"
   │
   └─> Once ready, [Open] button appears

5. Vibespace auto-stops after inactivity (if not persistent)
   └─> Knative scales 1 → 0 (saves resources)
```

### 11.5 Expose New Port

```
1. User is working in vibespace, starts dev server on port 8000

2. In Vibespace app:
   ├─> Click vibespace → "Settings" → "Ports" tab
   │
   └─> Click "Expose Port"
       Port: [8000]
       Protocol: [HTTP ▼]

3. Click "Add"
   ├─> Backend creates IngressRoute
   ├─> Updates /etc/hosts
   └─> Returns URL

4. New URL appears in vibespace card
   App: vibespace-abc123-8000.local
   └─> Click to open in browser
```

---

## 12. Future Enhancements

### 12.1 Post-MVP Features

#### 12.1.1 GitOps with ArgoCD

**When**: After MVP, when multi-user/team scenarios emerge

**What**:
- Vibespace definitions as Git repos
- Automatic sync on Git push
- Rollback to previous vibespace states
- Audit trail of all changes

**Why**:
- Teams can share vibespace configs
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
- Per-user vibespaces and quotas
- Shared templates marketplace
- Team vibespaces (multiple users, same vibespace)

**Why**:
- Shared infrastructure cost savings
- Collaboration on same environment

#### 12.1.4 Cloud Deployment

**When**: Users want remote access or more resources

**What**:
- Deploy to remote k3s/k8s cluster
- Cloud provider integrations (AWS, GCP, Azure)
- SSH tunneling for secure access
- Vibespace migration (local ↔ cloud)

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
- Vibespace-to-vibespace mTLS
- Traffic splitting (A/B testing)
- Observability (Jaeger, Grafana)

**Why**:
- Microservices development
- Better debugging
- Production-like environments

#### 12.1.7 Vibespace Snapshots

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
├─ Vibespace snapshots
├─ Resource usage dashboard
└─ CLI tool

v0.3.0 - Week 12
├─ Multi-user support
├─ Team vibespaces
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

- **Vibespace start time**: <30s (from stopped to accessible)
- **Scale-to-zero**: <3s (from idle to stopped)
- **Build time**: <5min for typical template
- **API response**: <200ms (p95)
- **UI rendering**: <100ms (p95)

### 13.2 Resource Usage

- **k3s overhead**: <512MB RAM idle
- **Vibespace idle**: <200MB RAM (scaled to zero)
- **Vibespace active**: ~2GB RAM (code-server + app)
- **Disk**: ~10GB per vibespace (adjustable)
- **Total**: Minimum 8GB RAM, 50GB disk for 5 vibespaces

### 13.3 Reliability

- **Vibespace uptime**: 99% (excluding user-initiated stops)
- **Data durability**: 99.9% (PVC on local disk)
- **Cluster recovery**: Automatic restart after crash
- **Backup**: User-initiated snapshots

### 13.4 Security

- **Credential isolation**: Read-only host mounts
- **Network isolation**: Default deny inter-vibespace
- **Privilege escalation**: Containers run as non-root (UID 1000)
- **Secret management**: Kubernetes Secrets (base64)
- **API authentication**: (Future) JWT tokens

### 13.5 Usability

- **First-time setup**: <5 minutes
- **Create vibespace**: <3 clicks, <60s
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