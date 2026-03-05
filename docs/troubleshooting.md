# Troubleshooting

## Common problems

### `vibespace init` is slow

First init downloads binaries and boots a VM. This can take several minutes depending on your internet connection and machine.

On Linux VPS without KVM support, Lima/QEMU is especially slow. Use `--bare-metal` instead:

```bash
vibespace init --bare-metal
```

### Cluster not running

```bash
vibespace status
```

If the cluster shows as stopped, the VM may have been shut down. Run `vibespace init` again to restart it (existing data is preserved).

### Daemon not running

The daemon starts automatically with `vibespace init` and restarts on demand when you run commands that need it. If it's not responding:

```bash
# Check status
vibespace status

# Re-initialize (restarts the daemon)
vibespace init
```

### Agent container stuck in Pending

The cluster doesn't have enough resources to schedule the pod. Either:

- Reduce resource requests: `--cpu 100m --memory 256Mi`
- Increase cluster size: `vibespace init --cpu 8 --memory 16`
- Delete unused vibespaces to free resources

### SSH connection refused

The agent container may still be starting up. Wait a few seconds and retry. If it persists, check that the agent is running:

```bash
vibespace agent list --vibespace my-project
```

### Port forward not working

```bash
vibespace forward list --vibespace my-project
```

Check the status column. If it shows `error`, the tunnel may have dropped. Remove and re-add the forward.

### DNS names not resolving

On macOS, DNS for `.vibespace.internal` works through `/etc/resolver/`. This is respected by Safari, curl, and most CLI tools, but **not** by Chromium-based browsers (Chrome, Edge, Brave). Use Safari or curl instead.

### Remote mode: connection times out

1. Check that ports 51820/UDP and 7781/TCP are open on the server firewall
2. Check that the token hasn't expired (default TTL is 30 minutes)
3. Run `vibespace remote status` for diagnostics

### Remote mode: token rejected

Tokens are single-use and expire. Generate a new one on the server:

```bash
vibespace serve --generate-token
```

### WireGuard not found (Linux)

Install WireGuard tools:

```bash
sudo apt-get install -y wireguard-tools
```

On macOS, WireGuard is bundled automatically.

## Debug logging

Enable debug output for any command:

```bash
VIBESPACE_DEBUG=1 vibespace <command>
```

Logs are written to `~/.vibespace/debug.log`. Set the log level for more or less detail:

```bash
VIBESPACE_LOG_LEVEL=debug vibespace <command>
```

## Exit codes

Non-zero exit codes indicate specific failure categories:

| Code | Meaning |
|---|---|
| 1 | Internal error |
| 2 | Invalid usage (bad flags or arguments) |
| 10 | Resource not found |
| 11 | Already exists (conflict) |
| 12 | Permission denied |
| 13 | Timeout |
| 14 | Cancelled |
| 15 | Service unavailable (cluster or daemon down) |

In JSON mode (`--json`), errors include a machine-readable code and a hint:

```json
{
  "success": false,
  "error": {
    "message": "vibespace not found: my-project",
    "code": "NOT_FOUND",
    "exit_code": 10,
    "hint": "Check available vibespaces with 'vibespace list'"
  }
}
```

## Getting help

- [GitHub Issues](https://github.com/vibespacehq/vibespace/issues) — bug reports and feature requests
