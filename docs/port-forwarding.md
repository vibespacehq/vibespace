# Port Forwarding

Port forwarding exposes ports from agent containers on your local machine. If an agent is running a dev server on port 3000 inside its container, you can make it available at `localhost:<port>` on your host.

## Adding a forward

```bash
vibespace forward add 3000 --vibespace my-project
```

This allocates a local port automatically. To specify the local port:

```bash
vibespace forward add 3000 --vibespace my-project --local 3000
```

Target a specific agent (defaults to the primary):

```bash
vibespace forward add 8080 --vibespace my-project --agent codex-1
```

## Listing forwards

```bash
vibespace forward list --vibespace my-project
```

Shows the agent, local port, remote port, type, and status for each active forward.

## Removing a forward

```bash
vibespace forward remove 3000 --vibespace my-project
```

## DNS names

Instead of remembering port numbers, you can give forwards a DNS name under `.vibespace.internal`:

```bash
vibespace forward add 3000 --vibespace my-project --dns
```

This makes the forward accessible at `claude-1.vibespace.internal:3000` (the name is derived from the agent). Set a custom name with:

```bash
vibespace forward add 3000 --vibespace my-project --dns --dns-name myapp
```

DNS setup requires sudo (it writes to `/etc/resolver/` on macOS). The daemon runs an embedded DNS server on `127.0.0.1:5553`.

**Note:** Chromium-based browsers bypass the macOS resolver and won't resolve `.vibespace.internal` names. Use Safari, curl, or other tools that respect `/etc/resolver/`.

## How it works

The daemon manages all forwards. When you add a forward, the daemon opens an SSH tunnel to the agent container and binds it to a local port. If the tunnel drops, the daemon reconnects automatically.

Forwards survive agent restarts. The daemon watches for pod changes and re-establishes tunnels when pods come back up.
