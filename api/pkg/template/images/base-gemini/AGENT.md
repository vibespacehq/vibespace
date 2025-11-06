# Vibespace AI Assistant Context

## Environment Info
- **OS**: Ubuntu 24.04
- **code-server**: 4.104.3+ (VS Code 1.104.0+)
- **Node.js**: 24.x LTS (October 2025)
- **Python**: 3.12+ (for Gemini SDK)
- **User**: coder (UID 1000)
- **Vibespace**: /vibespace (persistent volume)
- **Port**: 8080 (code-server HTTP)

## AI Agent Available
- Google Gemini SDK pre-installed (Python)
- API key injected via environment variables (GOOGLE_API_KEY)
- Access via Python: `import google.generativeai as genai`

## Guidelines
- All project files are in /vibespace
- Use code-server extensions for linting, formatting
- Respect existing project structure
- Follow language-specific best practices
- Test changes before committing

## Credentials
- Git config: Set via environment variables
- SSH keys: Mounted at ~/.ssh (if configured)
- API keys: Injected as environment variables
