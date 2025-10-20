package template

import (
	_ "embed"
	"fmt"
)

// Embedded Dockerfiles for base images (per agent)
//
//go:embed images/base-claude/Dockerfile
var BaseClaudeDockerfile []byte

//go:embed images/base-claude/CLAUDE.md
var BaseClaudeMD []byte

//go:embed images/base-codex/Dockerfile
var BaseCodexDockerfile []byte

//go:embed images/base-codex/AGENT.md
var BaseCodexMD []byte

//go:embed images/base-gemini/Dockerfile
var BaseGeminiDockerfile []byte

//go:embed images/base-gemini/AGENT.md
var BaseGeminiMD []byte

//go:embed images/templates/nextjs/Dockerfile
var NextjsDockerfile []byte

//go:embed images/templates/nextjs/CLAUDE.md
var NextjsCLAUDEMD []byte

//go:embed images/templates/vue/Dockerfile
var VueDockerfile []byte

//go:embed images/templates/vue/CLAUDE.md
var VueCLAUDEMD []byte

//go:embed images/templates/jupyter/Dockerfile
var JupyterDockerfile []byte

//go:embed images/templates/jupyter/CLAUDE.md
var JupyterCLAUDEMD []byte

// GetDockerfile returns the Dockerfile content for a template and agent
func GetDockerfile(templateID, agent string) ([]byte, error) {
	// Base images are agent-specific
	if templateID == "base" {
		switch agent {
		case "claude":
			return BaseClaudeDockerfile, nil
		case "codex":
			return BaseCodexDockerfile, nil
		case "gemini":
			return BaseGeminiDockerfile, nil
		default:
			return nil, fmt.Errorf("unknown agent: %s", agent)
		}
	}

	// Template images are agent-agnostic (use ARG in Dockerfile)
	switch templateID {
	case "nextjs":
		return NextjsDockerfile, nil
	case "vue":
		return VueDockerfile, nil
	case "jupyter":
		return JupyterDockerfile, nil
	default:
		return nil, fmt.Errorf("unknown template: %s", templateID)
	}
}

// GetAgentMD returns the agent instruction file content for a template and agent
func GetAgentMD(templateID, agent string) ([]byte, error) {
	// Base images have agent-specific instruction files
	if templateID == "base" {
		switch agent {
		case "claude":
			return BaseClaudeMD, nil
		case "codex":
			return BaseCodexMD, nil
		case "gemini":
			return BaseGeminiMD, nil
		default:
			return nil, fmt.Errorf("unknown agent: %s", agent)
		}
	}

	// Template images have framework-specific guides (agent-agnostic)
	switch templateID {
	case "nextjs":
		return NextjsCLAUDEMD, nil
	case "vue":
		return VueCLAUDEMD, nil
	case "jupyter":
		return JupyterCLAUDEMD, nil
	default:
		return nil, fmt.Errorf("unknown template: %s", templateID)
	}
}

// GetAllTemplateIDs returns all available template IDs (excluding base)
func GetAllTemplateIDs() []string {
	return []string{"nextjs", "vue", "jupyter"}
}

// GetAllAgents returns all supported AI agents
func GetAllAgents() []string {
	return []string{"claude", "codex", "gemini"}
}
