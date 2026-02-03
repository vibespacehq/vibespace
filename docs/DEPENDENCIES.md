# vibespace CLI Dependencies

This document tracks all external dependencies required by the vibespace CLI, their installation methods, and bundling status across platforms.

## Dependency Matrix

| Dependency | macOS | Linux | Notes |
|------------|-------|-------|-------|
| **Colima** | ✅ Bundled | N/A | macOS only, manages VM + Docker |
| **Lima (limactl)** | ✅ Bundled | ✅ Bundled | VM orchestration |
| **QEMU** | N/A | ✅ Bundled | Linux VM emulation |
| **Docker CLI** | ✅ Bundled | N/A | macOS only (Colima provides daemon) |
| **kubectl** | ✅ Bundled | ✅ Bundled | Kubernetes management |
| **wg** | ✅ Bundled | ❌ System (apt) | WireGuard CLI tool |
| **wg-quick** | ✅ Bundled | ❌ System (apt) | WireGuard helper script |
| **wireguard-go** | ✅ Bundled | N/A | Userspace WireGuard (macOS only) |
| **wireguard kernel module** | N/A | ❌ System (modprobe) | Linux kernel module |
| **ssh** | ❌ System | ❌ System | Pre-installed on both |
| **bash** | ❌ System | ❌ System | Pre-installed on both |
| **tar** | ❌ System | ❌ System | Pre-installed on both |
| **pgrep** | ❌ System | ❌ System | Pre-installed on both |
| **sudo** | ❌ System | ❌ System | Required for WireGuard ops |

**Legend:**
- ✅ Bundled = Downloaded to `~/.vibespace/` during setup
- ❌ System = Uses system-installed binary
- N/A = Not applicable for this platform

---

## macOS Dependencies

### Bundled (`~/.vibespace/`)

| Binary | Location | Source | Version |
|--------|----------|--------|---------|
| colima | `~/.vibespace/bin/colima` | [GitHub releases](https://github.com/abiosoft/colima) | Latest |
| limactl | `~/.vibespace/lima/bin/limactl` | [GitHub releases](https://github.com/lima-vm/lima) | Latest |
| docker | `~/.vibespace/bin/docker` | download.docker.com | v27.5.1 |
| kubectl | `~/.vibespace/bin/kubectl` | dl.k8s.io | Latest stable (fallback: v1.29.0) |
| wg | `~/.vibespace/bin/wg` | Homebrew bottles | Latest |
| wireguard-go | `~/.vibespace/bin/wireguard-go` | Homebrew bottles | Latest |

### System (pre-installed)

| Binary | Location | Used For |
|--------|----------|----------|
| ssh | `/usr/bin/ssh` | Agent connections |
| bash | `/bin/bash` | Command execution |
| tar | `/usr/bin/tar` | Archive extraction |
| open | `/usr/bin/open` | Browser launching |
| sw_vers | `/usr/bin/sw_vers` | macOS version detection |
| ifconfig | `/sbin/ifconfig` | WireGuard IP assignment |
| route | `/sbin/route` | WireGuard routing |
| sudo | `/usr/bin/sudo` | Privileged operations |

---

## Linux Dependencies

### Bundled (`~/.vibespace/`)

| Binary | Location | Source | Version |
|--------|----------|--------|---------|
| limactl | `~/.vibespace/lima/bin/limactl` | [GitHub releases](https://github.com/lima-vm/lima) | Latest |
| qemu-* | `~/.vibespace/qemu/bin/` | vibespace-binaries GitHub | v10.2.0 |
| kubectl | `~/.vibespace/bin/kubectl` | dl.k8s.io | Latest stable (fallback: v1.29.0) |

### System (apt-get install)

| Package | Binaries | Installation Command |
|---------|----------|---------------------|
| wireguard-tools | `wg`, `wg-quick` | `sudo apt-get install -y wireguard-tools` |

**Note:** WireGuard on Linux also requires `sudo modprobe wireguard` to load the kernel module.

### System (pre-installed)

| Binary | Location | Used For |
|--------|----------|----------|
| ssh | `/usr/bin/ssh` | Agent connections |
| bash | `/bin/bash` | Command execution |
| tar | `/usr/bin/tar` | Archive extraction |
| xdg-open | `/usr/bin/xdg-open` | Browser launching |
| pgrep | `/usr/bin/pgrep` | Process discovery |
| sudo | `/usr/bin/sudo` | Privileged operations |

---

## Installation Methods

### Direct Binary Download
- **colima**: GitHub release asset (darwin-amd64/darwin-arm64)
- **kubectl**: dl.k8s.io/release/{version}/bin/{os}/{arch}/kubectl
- **docker**: download.docker.com/mac/static/stable/{arch}/docker-{version}.tgz

### GitHub Release tar.gz Extraction
- **lima**: lima-vm/lima releases → extract limactl
- **qemu**: vibespace-binaries releases → extract qemu binaries

### Homebrew Bottles (macOS only)
- **wireguard-tools**: formulae.brew.sh API → bottle tar.gz → extract `wg`
- **wireguard-go**: formulae.brew.sh API → bottle tar.gz → extract `wireguard-go`

Supported macOS versions for Homebrew bottles:
- Tahoe (26), Sequoia (15), Sonoma (14), Ventura (13), Monterey (12), Big Sur (11)

### System Package Manager (Linux only)
- **wireguard-tools**: `sudo apt-get install -y wireguard-tools`

---

## Sudo Requirements

Operations requiring elevated privileges:

| Operation | Platform | Command |
|-----------|----------|---------|
| WireGuard config install | Linux only | `sudo cp <config> /etc/wireguard/` |
| WireGuard interface up | Linux | `sudo wg-quick up <interface>` |
| WireGuard interface up | macOS | `sudo wireguard-go utun` |
| WireGuard IP assignment | macOS | `sudo ifconfig <tun> inet <ip>` |
| WireGuard routing | macOS | `sudo route -n add -net 10.100.0.0/24` |
| Kernel module load | Linux | `sudo modprobe wireguard` |
| apt-get install | Linux | `sudo apt-get install -y wireguard-tools` |

**Note:** On macOS, WireGuard configs stay in `~/.vibespace/` (user-writable), so no sudo is needed for config file operations. On Linux, configs go to `/etc/wireguard/` per wg-quick convention.

---

## Known Issues

### Linux WireGuard Not Bundled

Unlike macOS where WireGuard tools are downloaded as standalone binaries from Homebrew bottles, Linux uses `apt-get install` which:

1. **Requires Debian/Ubuntu**: Won't work on other distros (Fedora, Arch, etc.)
2. **Modifies system packages**: Not isolated to `~/.vibespace/`
3. **Potential conflicts**: May conflict with existing WireGuard installations
4. **Requires sudo**: System-wide package installation

**Potential fix**: Download static WireGuard binaries for Linux similar to macOS approach.

---

## Container Dependencies (Out of Scope)

Dependencies inside Kubernetes pods (agent containers) are installed via apt-get in Dockerfiles. These are sandboxed and do not affect the host system:

- openssh-server, supervisor, ttyd
- Development tools (git, vim, curl, etc.)
- Language runtimes (python3, nodejs)
- Claude Code CLI

See `build/base/Dockerfile` and `build/agents/*/Dockerfile` for details.
