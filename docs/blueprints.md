# Blueprints

Blueprints are templates that define how a workspace launches. They configure the lead agent's role, the form you fill out, and the system prompt that drives the work.

## Built-in blueprints

### Code Studio

Spin up a lead developer with a team that builds your software project.

**Form fields:**
- Project name
- What are you building?
- Workspace mode (worktree or shared filesystem)
- Tech stack (optional)
- Repository URL (optional)

The lead agent reads your project description, analyzes the repo (if provided), and starts building. It can recruit additional agents for parallel work and create channels for coordination.

**Worktree mode** gives each agent its own git branch, so they can work in parallel without merge conflicts. **Shared filesystem** mode has all agents working on the same files.

### Startup Machine

Launch an AI CEO that analyzes your idea, builds a team, and runs your business.

**Form fields:**
- Business name
- Business type (new idea or existing business)
- Description
- Website URL (optional)

The lead agent takes on a CEO role — analyzing the market, defining strategy, recruiting specialist agents (researcher, developer, marketer), and coordinating execution.

### Think Tank

Deploy a research lead that coordinates deep analysis on any topic.

**Form fields:**
- Research topic
- Depth and focus areas

The lead agent breaks the topic into angles, assigns research tasks to specialist agents, and synthesizes findings into structured output.

## Custom blueprints

Click **Create custom** in the blueprint picker to design your own. You can configure:

- **Name and description** — what the blueprint is for
- **Icon and color** — visual identity in the picker
- **Form fields** — text inputs, text areas, and select dropdowns with custom labels and placeholders
- **First agent** — name, display name, icon, color, and provider type

Custom blueprints are saved locally and appear alongside built-in ones in the blueprint picker. You can delete custom blueprints from the picker view.

## How blueprints work

When you launch a workspace from a blueprint:

1. Your form values are rendered into the blueprint's **prompt template** — this becomes the lead agent's system prompt
2. The lead agent's container starts with the rendered prompt
3. A **kickoff message** (also rendered with your values) is sent to the agent
4. The agent starts working based on the prompt and kickoff message
5. If the blueprint or prompt instructs it, the agent recruits additional agents via the comms server

The system prompt also includes environment context — your platform, available package managers, and project paths — so agents can write platform-appropriate commands.
