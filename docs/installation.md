# Installation

## System Requirements

| | macOS | Linux |
|---|---|---|
| **Architecture** | Apple Silicon (arm64) or Intel (amd64) | x86_64 |
| **OS version** | macOS 12+ (Monterey) | Ubuntu 20.04+, Debian 11+ |
| **CPU** | 4+ cores recommended | 4+ cores recommended |
| **RAM** | 8GB+ recommended | 8GB+ recommended |
| **Disk** | ~5GB for cluster + storage | ~5GB for cluster + storage |

Linux support is tested on Debian-based distros. Other distributions may work but aren't officially supported yet.

## Install

### Homebrew (macOS / Linux)

```bash
brew install vibespacehq/tap/vibespace
```

### APT (Debian / Ubuntu)

```bash
curl -fsSL https://vibespacehq.github.io/apt/setup.sh | sudo bash
sudo apt install vibespace
```

### Shell script (any platform)

```bash
curl -fsSL https://raw.githubusercontent.com/vibespacehq/vibespace/main/install.sh | bash
```

Detects your OS and architecture, downloads the binary, verifies the checksum, and installs to `/usr/local/bin`.

### Go

Requires Go 1.25 or later.

```bash
go install github.com/vibespacehq/vibespace/cmd/vibespace@latest
```

### From source

Requires Go 1.25 or later.

```bash
git clone https://github.com/vibespacehq/vibespace.git
cd vibespace
./scripts/build.sh
sudo ./scripts/install.sh
```

This builds the binary with version info injected and copies it to `/usr/local/bin/vibespace`.

## Cluster initialization

After installing the binary, initialize the cluster:

```bash
vibespace init
```

This automatically downloads and sets up:

| Component | macOS | Linux |
|---|---|---|
| **k3s** | Inside VM | Inside VM or bare metal |
| **kubectl** | `~/.vibespace/bin/` | `~/.vibespace/bin/` |
| **Colima** | `~/.vibespace/bin/` | N/A |
| **Lima** | `~/.vibespace/lima/` | `~/.vibespace/lima/` |
| **QEMU** | N/A | `~/.vibespace/qemu/` |
| **Docker CLI** | `~/.vibespace/bin/` | N/A |
| **WireGuard tools** | `~/.vibespace/bin/` | System (`apt-get`) |

Everything except WireGuard on Linux is self-contained in `~/.vibespace/` — no system packages are modified.

### Cluster sizing

Default VM allocation is 4 CPU, 8GB RAM, 60GB disk. Override with:

```bash
vibespace init --cpu 8 --memory 16 --disk 100
```

Or set defaults via environment variables:

```bash
export VIBESPACE_CLUSTER_CPU=8
export VIBESPACE_CLUSTER_MEMORY=16
export VIBESPACE_CLUSTER_DISK=100
```

### Bare metal mode (Linux only)

Skip the VM and install k3s directly on the host:

```bash
vibespace init --bare-metal
```

This is faster and uses fewer resources, but k3s runs directly on your machine rather than inside an isolated VM. Useful for VPS deployments.

### External cluster

Use an existing Kubernetes cluster instead of creating one:

```bash
vibespace init --external --kubeconfig /path/to/kubeconfig
```

## Upgrade

```bash
vibespace upgrade
```

Downloads the latest release from GitHub, verifies the checksum, and replaces the current binary. Use `vibespace upgrade --check` to see if an update is available without installing it.

If the binary is in a system directory (e.g. `/usr/local/bin`), you may need `sudo vibespace upgrade`.

For Homebrew installations, use `brew upgrade vibespace` instead.

## Uninstall

```bash
vibespace uninstall
```

This removes the cluster, VMs, downloaded binaries, and all state in `~/.vibespace/`. Persistent data in vibespaces is deleted. The vibespace binary itself stays in `/usr/local/bin/` — remove it manually if you want.
