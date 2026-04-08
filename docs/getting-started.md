# Getting Started

Welcome to vibespace — AI agent teams for your Mac.

## Download

Grab the latest `.dmg` from [GitHub Releases](https://github.com/vibespacehq/vibespace/releases/latest). Open it and drag vibespace to your Applications folder.

## First launch

When you open vibespace for the first time, it sets up a lightweight Linux VM and pulls the agent container images. This takes a couple of minutes — you'll see a progress indicator. After that, workspaces launch in seconds.

## Sign in to a provider

Before creating a workspace, you'll need to authenticate with at least one AI provider:

- **Claude Code** — requires an Anthropic Max subscription. Vibespace opens a browser-based authentication flow.
- **Codex** — requires an OpenAI Plus subscription. Vibespace uses a device code flow — you'll get a code to enter on OpenAI's website.

You can sign in to both providers and mix them in the same workspace.

## Create your first workspace

1. Click the **+** button or choose "New Vibespace" from the menu
2. Pick a blueprint:
   - **Code Studio** — best for software projects. Give it a repo and a description.
   - **Startup Machine** — for business ideas. Describe your idea and let an AI CEO build a team.
   - **Think Tank** — for research tasks. Pose a question and get multi-angle analysis.
3. Fill in the form fields (project name, description, tech stack, etc.)
4. Click **Launch**

A lead agent spins up in its own container, reads your prompt, and starts working. Depending on the blueprint, it may recruit additional agents and create channels for team coordination.

## What happens next

Once your workspace is running:

- **Live feed** shows all agent activity in real time
- **Channels** appear in the sidebar as agents create them (e.g., `#dev-team`)
- **Direct messages** let you chat with individual agents
- **Approval ticker** notifies you when agents need permission for something
- **Vault** prompts appear when agents request API keys or secrets

You can switch between the live feed (overview of all agents) and individual agent views (chat, files, port forwards) using the sidebar.

## Next steps

- [Blueprints](blueprints.md) — learn about built-in blueprints and how to create your own
- [Agents](agents.md) — understand agent types, communication, and lifecycle
- [Vault](vault.md) — manage secrets and API keys
- [Concepts](concepts.md) — dive into the architecture
