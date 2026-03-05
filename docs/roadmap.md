**SUBJECT TO CHANGE PRIOR TO GOING PUBLIC**

# Roadmap

## Short term

- `vibespace doctor` — diagnose setup issues
- `vibespace wait` — block until cluster/agents ready
- Vibespace forking
- Custom agent images config
- Pre-built binaries, Homebrew tap, curl install script
- Improve `--help` text across all commands
- Agent health checks

## Medium term

- Declarative config (`vibespace apply -f spec.yaml`, `diff`, `export`)
- `.vibespace.yaml` project defaults in repo root
- `vibespace dev` — auto-create vibespace from current repo
- File sync for remote mode (`--sync local:container`)
- Agent snapshots and restore
- Agent cloning
- Agent logs streaming
- Agent pause/resume without stopping the pod
- Agent environment variables (`--env KEY=VALUE`)
- Agent labels and annotations
- More agent types as they become available
- Session export to markdown
- Session forking
- Session templates for common workflows
- Code review workflow — one agent writes, another reviews
- `@team` broadcasts to agent groups
- Agent roles (leader/follower) for coordinated tasks
- Shared scratchpad between agents in same vibespace
- Simultaneous local + remote clusters
- Custom domain per vibespace in remote mode
- List filtering (`--status`), sorting, pagination
- `--timeout` and `--non-interactive` global flags
- `multi --stream --json` JSONL streaming output
- Update checker

## Long term

- Agent-to-agent direct messaging
- Task queues agents can claim work from
- Consensus mode — agents vote on decisions
- Token usage tracking and cost estimation
- Prometheus metrics and Grafana dashboard template
- Agent memory/context persistence across sessions
- RAG integration — agents can search local docs
- MCP server management per agent
- Model switching mid-session
- Split-pane TUI for watching multiple agents
- Message search and bookmarks in chat history
- Customizable keybindings and themes
- Image display in terminal (sixel/kitty)
- Tailscale as alternative to WireGuard
- Web/mobile companion app
- IDE integrations (VS Code, JetBrains)
- GitHub Actions integration
- Slack/Discord bot
- GPU passthrough and multi-node clusters
- Plugin system for custom commands
- Backup/restore with S3/GCS sync
- Self-update (`vibespace upgrade`)

See [GitHub Issues](https://github.com/vibespacehq/vibespace/issues) for the full backlog.
