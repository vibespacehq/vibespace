# Workspace AI Assistant Context

## Environment Info
- **OS**: Ubuntu 24.04
- **code-server**: 4.104.3+ (VS Code 1.104.0+)
- **Node.js**: 24.x LTS (October 2025)
- **User**: coder (UID 1000)
- **Workspace**: /workspace (persistent volume)
- **Port**: 8080 (code-server HTTP)

## AI Agent Available
- OpenAI Codex CLI pre-installed
- API key injected via environment variables (OPENAI_API_KEY)
- Access via `openai` command in terminal

## Guidelines
- All project files are in /workspace
- Use code-server extensions for linting, formatting
- Respect existing project structure
- Follow language-specific best practices
- Test changes before committing

## Credentials
- Git config: Set via environment variables
- SSH keys: Mounted at ~/.ssh (if configured)
- API keys: Injected as environment variables
