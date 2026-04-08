# Troubleshooting

Common issues and how to resolve them.

## First launch

### VM setup is slow

The first launch downloads and configures a Lima VM with containerd. This can take 2-5 minutes depending on your internet connection. Subsequent launches are fast because the VM is reused.

### VM fails to start

If the VM fails to initialize:

1. Check that you have enough disk space (~2GB for the VM and container images)
2. Try restarting the app
3. Check `~/.vibespace/daemon.log` for error details
4. As a last resort, delete `~/.lima/vibespace/` and relaunch — the VM will be recreated

## Authentication

### Claude Code authentication fails

Make sure you have an active Anthropic Max subscription. The authentication flow opens a browser window — check that it completed successfully. If it gets stuck, try signing out and back in from the provider selection screen.

### Codex device code expired

The OpenAI device code flow has a short expiration window. If the code expires before you enter it, the app will generate a new one. Make sure to enter the code promptly at the URL shown.

## Agents

### Agent stuck in "starting" state

This usually means the SSH connection failed to establish. Check:

1. The VM is running (the app shows "running" status in the sidebar)
2. The container actually started — check `~/.vibespace/daemon.log`
3. Try stopping and restarting the workspace

### Agent not responding

If an agent appears idle but doesn't respond to messages:

1. Check the agent's status in the live feed
2. Try sending a message directly from the chat tab
3. The agent may have hit a token limit — check the token tracker

### Container keeps restarting

The daemon has a reconciler that watches container events. If a container crashes, it tries to restart it. Check `~/.vibespace/daemon.log` for crash reasons — common causes include SSH key issues or missing credentials.

## Communication

### Messages not appearing

Agent-to-agent messages flow through the comms server. If messages aren't showing up:

1. Check that the comms server is running (it starts automatically with the daemon)
2. The SSH reverse tunnel may have dropped — restarting the workspace usually fixes this
3. Check `~/.vibespace/daemon.log` for comms server errors

### Channel not visible

Channels only appear in the sidebar after an agent creates them. If you expect a channel but don't see it, the lead agent may not have created it yet — check the live feed for activity.

## Vault

### Secret not being injected

Secrets are injected when an agent starts. If you add a secret after an agent is already running, the agent needs to reload secrets via `vs-comms secret load`. For new agents, secrets are available immediately.

## Performance

### High memory usage

Each agent runs in its own container inside the VM. Memory is shared across all containers. If you're running many agents, consider stopping workspaces you're not actively using.

### Slow file operations

Bind mounts use virtiofs for file sharing between your Mac and the VM. Performance is generally good but can slow down with very large directories. The shared directory is `~/.vibespace/data/<id>/` — only files here are shared with containers.

## Reset

### Clean slate

If something is deeply broken, you can do a full reset:

1. Quit vibespace
2. Delete `~/.vibespace/` (removes all state, secrets, databases)
3. Delete `~/.lima/vibespace/` (removes the VM)
4. Relaunch vibespace — everything will be recreated from scratch

**Warning:** This deletes all your workspaces, secrets, and agent data.
