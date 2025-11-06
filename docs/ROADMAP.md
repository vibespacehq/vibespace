# vibespace - Product Roadmap

**Vision**: The easiest way to create, manage, and scale development vibespaces with AI coding agents.

---

## MVP Phase 1: Foundation ⏳ IN PROGRESS

**Timeline**: Weeks 1-3 (Week 2 of 3)
**Goal**: Prove the core value proposition - vibespace management with AI agents works locally.
**Target Users**: Developer early adopters comfortable with command-line tools.

### Core Features
- [x] **Tauri desktop app** - React 19 + TypeScript UI
- [x] **Kubernetes detection** - Detects k3s, Rancher Desktop, k3d
- [x] **Cluster setup wizard** - Guided installation with component status
- [x] **Component installation** - Knative v1.15.2, Traefik v3.5.3, Registry 2.8.3, BuildKit v0.17.3
- [x] **Go API server** - Full REST API with Kubernetes client-go integration
- [x] **Vibespace CRUD backend** - Create, list, get, delete (currently uses placeholder image)
- [ ] **Docker images with AI agents**:
  - [ ] Base image: code-server + Claude Code CLI
  - [ ] Next.js template with AI agent + CLAUDE.md
  - [ ] Vue template with AI agent + CLAUDE.md
  - [ ] Python/Jupyter template with AI agent + CLAUDE.md
- [ ] **Single AI agent per vibespace** - Baked into container (Claude Code, OpenAI Codex)
- [ ] **Credential management**:
  - [ ] AI agent API keys (Claude, OpenAI)
  - [ ] Git config (name, email)
  - [ ] SSH keys (generate or import)
  - [ ] Kubernetes Secret generation
- [ ] **Local vibespace access** - Via NodePort or `kubectl port-forward`
- [ ] **Template selection UI** - Choose template and configure agent during creation

**Note**: A "template" includes the complete vibespace configuration: development stack (Next.js, Python, etc.), AI coding agent selection, agent instruction files (CLAUDE.md), and optionally a connected Git repository.

### Infrastructure
- **Kubernetes**: Detection-based (users install k3s or Rancher Desktop manually)
- **Deployment**: Simple Kubernetes Pods (no Knative yet - easier to implement and debug)
- **Networking**: NodePort or kubectl port-forward (no Ingress yet)
- **Storage**: Local PersistentVolumes via k3s local-path provisioner
- **Access**: `localhost:8080` via port-forward

### Success Metrics
- [ ] 10 beta users can create and use vibespaces
- [ ] AI agents work successfully (Claude Code, OpenAI Codex)
- [ ] Positive feedback on core vibespace management
- [ ] < 5 minutes from download to first vibespace

**Status**: ~70% complete
- ✅ Infrastructure, API server, frontend UI, cluster setup
- ❌ AI agents, templates, credential backend, real images

---

## MVP Phase 2: Scale & Cloud 🔮 PLANNED

**Timeline**: Weeks 4-8
**Goal**: Production-grade features with local scale-to-zero and cloud deployment support.
**Target Users**: Developers who need production-like environments and cloud resources.

### Core Features

#### Knative Scale-to-Zero
- [ ] Migrate from simple Pods to **Knative Services**
- [ ] Auto-stop vibespaces when idle (scale to zero)
- [ ] Auto-start vibespaces on access (scale from zero)
- [ ] Saves local machine resources
- [ ] Vibespace start/stop lifecycle UI

#### Custom Template Builder
- [ ] **Visual Dockerfile editor** with syntax highlighting (Monaco)
- [ ] **BuildKit integration** for building custom images
- [ ] **Real-time build logs** via Server-Sent Events
- [ ] Save and manage custom templates
- [ ] Use custom templates in vibespace creation
- [ ] Template metadata (name, description, icon)

#### Multi-Agent Sidecars
- [ ] **Run multiple AI agents per vibespace** (e.g., frontend + backend + testing agents)
- [ ] Agents as **Kubernetes sidecar containers** (resource isolation)
- [ ] **Terminal-based agent switching**: `agent use <name>`
- [ ] Direct shell access to each agent's environment
- [ ] Agent coordination for complex tasks
- [ ] Phase 1 vibespaces (single baked-in agent) remain supported

#### Cloud Service Provider Integration
- [ ] **AWS EKS** - Deploy vibespaces to Amazon Kubernetes
- [ ] **GCP GKE** - Deploy vibespaces to Google Kubernetes
- [ ] **DigitalOcean Kubernetes** - Deploy to DOKS
- [ ] **Azure AKS** - Deploy to Azure Kubernetes
- [ ] **Deployment mode toggle** - Switch between Local ↔ Cloud
- [ ] **Remote vibespace access** - Secure tunnels for cloud vibespaces
- [ ] **Vibespace migration** - Move vibespace from local to cloud (and back)
- [ ] Cloud credential management (AWS, GCP, Azure, DO API keys)

#### TLS Certificates
- [ ] **cert-manager deployment** - Automated certificate management
- [ ] **Let's Encrypt integration** - Free TLS certificates
- [ ] Automatic certificate renewal
- [ ] HTTPS for all vibespaces (local and cloud)

#### Custom Domains & DNS
- [ ] **Custom domain support** - `myproject.example.com` instead of `vibespace-abc123.local`
- [ ] **Automated DNS management**:
  - [ ] Cloudflare API integration
  - [ ] AWS Route53 integration
  - [ ] Azure DNS integration
- [ ] Domain validation via TXT records
- [ ] Traefik IngressRoutes with custom domains
- [ ] Local DNS setup (`/etc/hosts` manipulation for `.local` domains)

#### Rancher Desktop Integration
- [ ] Auto-detect Rancher Desktop installation
- [ ] "Open in Rancher Desktop" button
- [ ] Status synchronization with Rancher Desktop UI
- [ ] Recommend Rancher Desktop prominently in setup wizard

### Infrastructure
- **Kubernetes**: Local (k3s, Rancher Desktop) OR Cloud (EKS, GKE, DOKS, AKS)
- **Deployment**: Knative Services with scale-to-zero
- **Networking**: Traefik Ingress + TLS certificates
- **DNS**: Automated via cloud provider APIs (Cloudflare, Route53, Azure)
- **Storage**: Local PVCs OR cloud-native volumes (EBS, Persistent Disk, etc.)
- **Access**: `vibespace-{id}.local` OR custom domain with HTTPS

### Success Metrics
- [ ] 100+ active users
- [ ] Vibespaces successfully scale to zero locally
- [ ] Users deploy to at least 2 cloud providers
- [ ] Custom domains work with automatic TLS
- [ ] < 2 minutes to create new vibespace
- [ ] Users report production-grade stability

**ETA**: End of Month 2

---

## Post-MVP: Enterprise & Ecosystem 🔮 FUTURE

**Timeline**: Month 3+
**Goal**: Enterprise-ready platform with zero-config installation, community ecosystem, and advanced features.
**Target Users**: Teams, enterprises, general developers.

### Core Features

#### Bundled Kubernetes (Zero-Config Installation)
- [ ] **macOS**: Embedded Lima VM + k3s (no manual installation)
- [ ] **Windows**: WSL2 integration + k3s
- [ ] **Linux**: Native k3s with auto-setup
- [ ] Signed installers (`.dmg`, `.msi`, `.deb`, `.rpm`)
- [ ] Automatic component updates
- [ ] System tray integration (start/stop cluster)
- [ ] Resource limits UI (CPU/RAM allocation)
- [ ] Works offline after initial download
- [ ] Installer size: ~150-200MB

#### Template Marketplace
- [ ] **Community template registry** - Browse and discover templates
- [ ] Template ratings and reviews
- [ ] One-click template installation
- [ ] **GitHub integration** - Import templates from GitHub repos
- [ ] Template categories (web, data science, mobile, etc.)
- [ ] Template versioning
- [ ] Template submission and approval workflow
- [ ] Search and filtering

#### Vibespace Snapshots & Backups
- [ ] **One-click snapshot** - Capture PVC + metadata
- [ ] **Point-in-time restore** - Restore vibespace to previous state
- [ ] Share snapshots with team members
- [ ] Scheduled automatic snapshots
- [ ] Snapshot storage management
- [ ] Cross-cloud snapshot migration

#### CI/CD Integration
- [ ] **GitHub Actions integration** - Use vibespaces as CI runners
- [ ] **GitLab CI integration**
- [ ] Vibespace-as-CI-runner architecture
- [ ] Automated testing in vibespaces
- [ ] Build artifact management

#### Monitoring & Observability
- [ ] **Prometheus metrics** - Real-time resource monitoring
- [ ] **Grafana dashboards** - Pre-built vibespace dashboards
- [ ] **Cost tracking** - Track cloud spending per vibespace
- [ ] **Usage analytics** - Vibespace usage patterns
- [ ] Alerts for resource limits
- [ ] Logs aggregation and search

#### Multi-User & Teams
- [ ] **User authentication** - OAuth, LDAP support
- [ ] **Team vibespaces** - Multiple users, same vibespace
- [ ] Per-user resource quotas
- [ ] Shared template library
- [ ] Role-based access control (RBAC)
- [ ] Team billing and usage tracking

#### Enterprise Features
- [ ] **SSO/SAML** - Enterprise single sign-on
- [ ] **Audit logs** - Complete activity tracking
- [ ] **Compliance** - SOC2, GDPR, HIPAA support
- [ ] **GitOps with ArgoCD** - Vibespace definitions as Git repos
- [ ] **Harbor registry** - Replace registry:2 with Harbor
  - Vulnerability scanning for all images
  - Image signing and verification
  - User/team-based access control
- [ ] On-premise deployment options
- [ ] Priority support tiers

### Infrastructure
- **Installation**: One-click with bundled VM + k3s (zero manual setup)
- **Multi-tenancy**: User isolation and per-user vibespaces
- **Registry**: Harbor with CVE scanning and RBAC
- **Compliance**: Audit logs, encryption at rest, SOC2 certification
- **Monitoring**: Prometheus + Grafana + cost tracking

### Success Metrics
- [ ] 1,000+ active users
- [ ] 90% of users install without external dependencies
- [ ] 50+ community-contributed templates
- [ ] 5+ enterprise pilot customers
- [ ] Zero support tickets about "k3s not found"
- [ ] Works completely offline

**ETA**: Month 3+

---

## Design Decisions & Rationale

### Why Detection First, Bundling Later?

**MVP Phase 1 Decision**: Use detection + guided setup instead of bundling Kubernetes.

**Rationale**:
1. **Speed to market**: Ship MVP in 3 weeks, not 11 weeks
2. **Validate first**: Prove vibespace management is valuable before building complex installer
3. **Focus**: Core vibespace features > cluster installation complexity
4. **Flexibility**: Supports k3s, Rancher Desktop, k3d, existing clusters
5. **Security**: No sudo execution from app (user controls their system)

**Future**: Full bundling in Post-MVP after product-market fit validation.

### Why Simple Pods in Phase 1, Knative in Phase 2?

**Rationale**:
- Phase 1: Simple Pods are faster to implement, easier to debug, sufficient for MVP
- Phase 2: Knative adds complexity but huge value (scale-to-zero, auto-scaling)
- Users validate vibespaces work before we add sophisticated lifecycle management
- Knative requires understanding of serverless patterns - better with feedback first

### Why Cloud Mode in MVP Phase 2, Not Later?

**Rationale**:
- Cloud deployment is valuable early - users want powerful cloud resources
- TLS and custom domains are table stakes for production use
- Multi-agent sidecars benefit from cloud resources (more CPU/RAM)
- Combined with scale-to-zero, makes Phase 2 a compelling "production-ready" release
- Local-first Phase 1 still validates core concept quickly

### Why Template Marketplace is Post-MVP?

**Rationale**:
- Custom template builder (Phase 2) lets users create templates first
- Community needs time to build templates worth sharing
- Marketplace requires moderation, search, ratings - significant overhead
- Focus Phase 2 on production features (cloud, scale-to-zero, multi-agent)
- Marketplace is ecosystem play - comes after product is mature

---

## Release Strategy

### Alpha (MVP Phase 1)
- **Audience**: Internal team + 10 beta testers
- **Distribution**: GitHub Releases (manual download)
- **Feedback**: Direct communication, GitHub Issues
- **Timeline**: Week 3

### Beta (MVP Phase 2)
- **Audience**: 50-100 early adopters
- **Distribution**: GitHub Releases + homebrew cask (macOS)
- **Feedback**: GitHub Issues, Discord/Slack community
- **Timeline**: End of Month 2

### v1.0 (Post-MVP with Bundled Kubernetes)
- **Audience**: General public
- **Distribution**: Official website, package managers (brew, choco, apt, pacman)
- **Support**: Documentation, community forums, video tutorials
- **Timeline**: Month 3

### v2.0 (Post-MVP with Enterprise Features)
- **Audience**: Teams, enterprises
- **Distribution**: Cloud marketplaces (AWS, GCP, Azure)
- **Support**: Paid priority support tiers
- **Timeline**: Month 6+

---

## Open Questions & Future Exploration

### Platform Support
- **Windows**: Native support or WSL2-only?
- **Linux**: Which package formats priority? (.deb, .rpm, AppImage, snap, flatpak?)
- **Mobile**: iPad/Android support via web UI?

### Business Model
- Free for individual developers?
- Paid cloud hosting service?
- Enterprise licensing model?
- Template marketplace revenue share (similar to VS Code marketplace)?

### Integrations
- **IDEs**: VS Code extension, JetBrains Gateway plugin?
- **DevOps**: Terraform provider, Pulumi provider?
- **Monitoring**: Datadog, New Relic, Honeycomb integrations?
- **Communication**: Slack, Discord notifications?

---

## How to Contribute

We welcome contributions! Here's how to get involved:

1. **Try the MVP** (Phase 1): Install and give honest feedback
2. **Report Issues**: Use GitHub Issues for bugs and feature requests
3. **Vote on Features**: Comment or 👍 on issues you care about
4. **Submit PRs**: Follow CONTRIBUTING.md guidelines
5. **Build Templates**: Create and share your custom vibespace templates

**Current Priority**: We're focused on **MVP Phase 1 (Foundation)**. Features in Phase 2+ are planned but not immediate.

**Labels Guide**:
- `mvp-phase-1`: Critical for initial release
- `mvp-phase-2`: Important for production-grade release
- `post-mvp`: Future enhancements
- `good-first-issue`: Easy for newcomers

---

## Changelog

### 2025-01 (Current)
- ✅ MVP Phase 1 started (Week 2 of 3)
- ✅ Infrastructure complete (k8s manifests, API server, frontend)
- ✅ Cluster setup wizard with real-time progress
- ✅ Architecture decisions: Detection over bundling (ADR 0001), JSDoc for future exports (ADR 0002), Frontend organization (ADR 0003), Component versions (ADR 0004)
- ⏳ Working on: Docker images with AI agents, credential backend, templates

### Future Updates
- Release notes will be added as phases complete

---

**Last Updated**: 2025-10-16 (MVP Phase 1 - Week 2 of 3)

**Questions?** Open a [GitHub Discussion](https://github.com/yagizdagabak/vibespace/discussions)
