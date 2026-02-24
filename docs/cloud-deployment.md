# Cloud Deployment

Run vibespace on a VPS for more resources, persistent uptime, and remote access.

## Quick manual setup

Any Linux VPS with root access works. Tested on Hetzner, DigitalOcean, and similar providers.

**Recommended specs:**
- 4+ vCPUs (8 recommended)
- 8GB+ RAM (16 recommended)
- 40GB+ disk
- Ubuntu 22.04 or Debian 12
- Public IPv4 address

### Steps

1. SSH into your VPS
2. Install vibespace (build from source or copy the binary)
3. Initialize with bare metal mode:

```bash
vibespace init --bare-metal
```

4. Start serving:

```bash
vibespace serve
```

5. Open firewall ports:

```bash
# WireGuard tunnel
sudo ufw allow 51820/udp

# Registration API
sudo ufw allow 7781/tcp
```

6. Generate a token and connect from your local machine:

```bash
# On the VPS
vibespace serve --generate-token

# On your machine
sudo vibespace remote connect <token>
```

See [Remote Mode](remote-mode.md) for detailed connection instructions.

## Ansible playbooks

Coming soon. Ansible playbooks for automated provisioning on Hetzner, DigitalOcean, and other providers.
