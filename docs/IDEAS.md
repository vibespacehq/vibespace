# Ideas

Possible enhancements and features for future consideration.

---

## Agent Capabilities

- `<vs> logs <agent>` - Stream agent container logs
- `<vs> exec <agent> <cmd>` - Run arbitrary command in agent container
- `<vs> ssh <agent>` - Direct SSH without daemon (one-off connection)
- `<vs> snapshot <agent>` - Save agent state to tarball
- `<vs> restore <agent> <snapshot>` - Restore agent from snapshot
- `<vs> clone <agent> <new-name>` - Clone an agent with its state
- `<vs> pause/resume <agent>` - Pause agent without stopping pod
- Agent resource limits (`--cpu`, `--memory` flags on create)
- Agent environment variables (`--env KEY=VALUE`)
- Agent labels and annotations for organization
- Agent health checks with custom probes

## Multi-Agent Collaboration

- `@team <message>` - Broadcast to predefined agent groups
- Agent roles (leader/follower) for coordinated tasks
- Shared scratchpad between agents in same vibespace
- Agent-to-agent direct messaging without user relay
- Task queue that agents can claim work from
- Consensus mode - agents vote on decisions
- Code review workflow - one agent writes, another reviews

## Session Management

- `session export <name>` - Export session transcript to markdown
- `session replay <name>` - Replay session messages (demo mode)
- `session merge <a> <b>` - Combine two sessions
- `session fork <name>` - Create branch from session point
- Session templates for common workflows
- Auto-save session on crash recovery
- Session sharing via URL (with remote mode)

## TUI Enhancements

- Split pane view (multiple agents visible)
- Agent status bar (tokens used, response time)
- Message search/filter in history
- Bookmarks for important messages
- Syntax highlighting in code blocks
- Image/screenshot display (sixel/kitty)
- Mouse support for scrolling/selection
- Customizable keybindings
- Themes (dark/light/custom)

## Developer Experience

- `vibespace dev` - Auto-create vibespace from current git repo
- Git integration (auto-mount repo, track branch)
- `.vibespace.yaml` in repo root for project defaults
- Pre/post hooks for agent start/stop
- Integration with VS Code / JetBrains
- Browser extension for web-based access
- Slack/Discord bot integration
- GitHub Actions integration

## Performance & Scaling

- Agent pooling (pre-warmed agents ready to go)
- Lazy agent startup (start on first message)
- Agent hibernation after idle timeout
- Multi-node cluster support
- GPU passthrough for ML workloads
- Shared model cache across agents
- Response caching for repeated queries

## Security & Access

- API key rotation commands
- Per-agent API key isolation
- Audit log of all agent actions
- Role-based access control (RBAC)
- Session encryption at rest
- Signed container images
- Network policies between agents

## Observability

- `vibespace metrics` - Prometheus metrics endpoint
- `vibespace dashboard` - Web UI for monitoring
- Token usage tracking per agent/session
- Cost estimation and budgets
- Alert on high token usage
- Grafana dashboard template

## AI Features

- Agent memory/context persistence across sessions
- RAG integration (agent can search local docs)
- Tool/MCP server management per agent
- Custom system prompts per vibespace
- Model switching mid-session
- A/B testing different models
- Prompt templates library
- Agent personality presets

## Networking

- Custom domain per vibespace (with remote mode)
- mTLS between agents
- Service mesh integration (Istio/Linkerd)
- Ingress controller for web access
- DNS-based agent discovery

## Data Management

- `<vs> backup` - Backup vibespace data
- `<vs> restore` - Restore from backup
- S3/GCS sync for persistent volumes
- Data retention policies
- GDPR-compliant data deletion

## Misc

- `vibespace upgrade` - Self-update CLI
- `vibespace news` - Show changelog/announcements
- `vibespace feedback` - Submit feedback from CLI
- `vibespace benchmark` - Test agent response times
- Plugin system for custom commands
- Shell completions with descriptions
- Man pages generation
- Offline mode (queue messages for later)
