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
	// Validate inputs upfront
	validTemplates := map[string]bool{
		"base":    true,
		"nextjs":  true,
		"vue":     true,
		"jupyter": true,
	}
	validAgents := map[string]bool{
		"claude": true,
		"codex":  true,
		"gemini": true,
	}

	if !validTemplates[templateID] {
		return nil, fmt.Errorf("invalid template: %s (must be one of: base, nextjs, vue, jupyter)", templateID)
	}

	// For non-base templates, validate agent
	if templateID != "base" && !validAgents[agent] {
		return nil, fmt.Errorf("invalid agent: %s (must be one of: claude, codex, gemini)", agent)
	}

	// Base images are agent-specific
	if templateID == "base" {
		if !validAgents[agent] {
			return nil, fmt.Errorf("invalid agent: %s (must be one of: claude, codex, gemini)", agent)
		}

		switch agent {
		case "claude":
			return BaseClaudeDockerfile, nil
		case "codex":
			return BaseCodexDockerfile, nil
		case "gemini":
			return BaseGeminiDockerfile, nil
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
	}

	// Should never reach here due to validation above
	return nil, fmt.Errorf("unknown template: %s", templateID)
}

// GetAgentMD returns the agent instruction file content for a template and agent
func GetAgentMD(templateID, agent string) ([]byte, error) {
	// Validate inputs upfront
	validTemplates := map[string]bool{
		"base":    true,
		"nextjs":  true,
		"vue":     true,
		"jupyter": true,
	}
	validAgents := map[string]bool{
		"claude": true,
		"codex":  true,
		"gemini": true,
	}

	if !validTemplates[templateID] {
		return nil, fmt.Errorf("invalid template: %s (must be one of: base, nextjs, vue, jupyter)", templateID)
	}

	// For non-base templates, validate agent
	if templateID != "base" && !validAgents[agent] {
		return nil, fmt.Errorf("invalid agent: %s (must be one of: claude, codex, gemini)", agent)
	}

	// Base images have agent-specific instruction files
	if templateID == "base" {
		if !validAgents[agent] {
			return nil, fmt.Errorf("invalid agent: %s (must be one of: claude, codex, gemini)", agent)
		}

		switch agent {
		case "claude":
			return BaseClaudeMD, nil
		case "codex":
			return BaseCodexMD, nil
		case "gemini":
			return BaseGeminiMD, nil
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
	}

	// Should never reach here due to validation above
	return nil, fmt.Errorf("unknown template: %s", templateID)
}

// GetAllTemplateIDs returns all available template IDs (excluding base)
func GetAllTemplateIDs() []string {
	return []string{"nextjs", "vue", "jupyter"}
}

// GetAllAgents returns all supported AI agents
func GetAllAgents() []string {
	return []string{"claude", "codex", "gemini"}
}

// GetAllDockerfiles returns all Dockerfiles for building images
// Keys are formatted as "{type}-{agent}-Dockerfile" for base images
// and "{template}-Dockerfile" for template images (which use ARG AGENT)
func GetAllDockerfiles() map[string][]byte {
	return map[string][]byte{
		// Base images (one per agent)
		"base-claude-Dockerfile": BaseClaudeDockerfile,
		"base-codex-Dockerfile":  BaseCodexDockerfile,
		"base-gemini-Dockerfile": BaseGeminiDockerfile,
		// Template images (agent-agnostic, use ARG)
		"nextjs-Dockerfile":  NextjsDockerfile,
		"vue-Dockerfile":     VueDockerfile,
		"jupyter-Dockerfile": JupyterDockerfile,
	}
}

// GetAllAgentMDs returns all agent instruction files
// Keys are formatted as "{type}-{agent}-CLAUDE.md" or "{type}-{agent}-AGENT.md"
func GetAllAgentMDs() map[string][]byte {
	return map[string][]byte{
		// Base images
		"base-claude-CLAUDE.md": BaseClaudeMD,
		"base-codex-AGENT.md":   BaseCodexMD,
		"base-gemini-AGENT.md":  BaseGeminiMD,
		// Template images (framework-specific)
		"nextjs-CLAUDE.md":  NextjsCLAUDEMD,
		"vue-CLAUDE.md":     VueCLAUDEMD,
		"jupyter-CLAUDE.md": JupyterCLAUDEMD,
	}
}
