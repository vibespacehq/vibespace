# Live Feed

The live feed is the main dashboard view — a real-time stream of everything happening in your workspace.

## Layouts

Switch between layouts using the icons in the top-right corner:

- **Grid** — agent cards in a responsive grid (default)
- **Columns** — side-by-side columns for each agent
- **Split** — two-pane view for focused comparison
- **Stacked** — full-width cards stacked vertically

## Agent cards

Each agent gets a card showing:
- Agent name, icon, and status (busy/idle)
- Current output — text being generated, tools being used
- Recent files accessed (read/write/edit operations)
- Timestamp of last activity

Cards update in real time as agents work. Text output is debounced (merged every 250ms) to prevent UI thrashing.

## Event types

The live feed shows several types of events:

| Event | Description |
|-------|-------------|
| Text output | Agent writing text, reasoning, or explaining |
| Tool use | File reads, writes, edits with file paths shown |
| Messages | Agent-to-agent or agent-to-user communications |
| State changes | Agent going busy or idle |
| Approvals | Requests for user permission |

## Approval ticker

When an agent needs permission to do something (e.g., run a destructive command), an approval request appears in the notification ticker at the top of the live feed. You can:

- **Approve** — let the agent proceed
- **Deny** — block the action
- **Dismiss** — ignore the notification

Other notifications include authentication errors, secret requests (agent asking for an API key), and integration requests (agent requesting a package or service).

## Agent detail view

Click an agent in the sidebar to switch from the live feed to a detail view with three tabs:

### Chat
Direct conversation with the agent. See message history and send new messages.

### Files
Browse the agent's `/vibespace` directory. See file sizes, modification times, and which files were recently accessed via Read/Write/Edit tools.

### Previews
When an agent spins up a dev server, port forwards are listed here. You can preview web servers directly in the app with responsive viewport switching:
- Desktop (full width)
- Tablet (768px)
- Mobile (375px)
