package k8s

import (
	"testing"

	"vibespace/pkg/template"
)

func TestBuildConfigMapContainsAllDockerfiles(t *testing.T) {
	// Verify that GetAllDockerfiles returns all expected Dockerfiles
	dockerfiles := template.GetAllDockerfiles()

	expectedDockerfiles := []string{
		"base-claude-Dockerfile",
		"base-codex-Dockerfile",
		"base-gemini-Dockerfile",
		"nextjs-Dockerfile",
		"vue-Dockerfile",
		"jupyter-Dockerfile",
	}

	for _, name := range expectedDockerfiles {
		content, ok := dockerfiles[name]
		if !ok {
			t.Errorf("GetAllDockerfiles() missing expected file: %s", name)
			continue
		}
		if len(content) == 0 {
			t.Errorf("GetAllDockerfiles()[%s] is empty", name)
		}
	}

	if len(dockerfiles) != len(expectedDockerfiles) {
		t.Errorf("GetAllDockerfiles() returned %d files, want %d", len(dockerfiles), len(expectedDockerfiles))
	}
}

func TestBuildConfigMapContainsAllAgentMDs(t *testing.T) {
	// Verify that GetAllAgentMDs returns all expected agent instruction files
	agentMDs := template.GetAllAgentMDs()

	expectedMDs := []string{
		"base-claude-CLAUDE.md",
		"base-codex-AGENT.md",
		"base-gemini-AGENT.md",
		"nextjs-CLAUDE.md",
		"vue-CLAUDE.md",
		"jupyter-CLAUDE.md",
	}

	for _, name := range expectedMDs {
		content, ok := agentMDs[name]
		if !ok {
			t.Errorf("GetAllAgentMDs() missing expected file: %s", name)
			continue
		}
		if len(content) == 0 {
			t.Errorf("GetAllAgentMDs()[%s] is empty", name)
		}
	}

	if len(agentMDs) != len(expectedMDs) {
		t.Errorf("GetAllAgentMDs() returned %d files, want %d", len(agentMDs), len(expectedMDs))
	}
}

func TestBuildConfigMapContainsAllSupportFiles(t *testing.T) {
	// Verify that GetAllSupportFiles returns all expected support files
	supportFiles := template.GetAllSupportFiles()

	expectedFiles := []string{
		"vscode-settings.json",
		"Caddyfile",
		"supervisord.conf",
		"entrypoint.sh",
		"nextjs-preview.sh",
		"nextjs-prod.sh",
		"vue-preview.sh",
		"vue-prod.sh",
		"jupyter-preview.sh",
		"jupyter-prod.sh",
	}

	for _, name := range expectedFiles {
		content, ok := supportFiles[name]
		if !ok {
			t.Errorf("GetAllSupportFiles() missing expected file: %s", name)
			continue
		}
		if len(content) == 0 {
			t.Errorf("GetAllSupportFiles()[%s] is empty", name)
		}
	}

	if len(supportFiles) != len(expectedFiles) {
		t.Errorf("GetAllSupportFiles() returned %d files, want %d", len(supportFiles), len(expectedFiles))
	}
}

func TestBuildJobConstants(t *testing.T) {
	// Verify constants are set correctly
	if BuildConfigMapName != "vibespace-dockerfiles" {
		t.Errorf("BuildConfigMapName = %q, want %q", BuildConfigMapName, "vibespace-dockerfiles")
	}

	if BuildJobName != "vibespace-build-templates" {
		t.Errorf("BuildJobName = %q, want %q", BuildJobName, "vibespace-build-templates")
	}

	if BuildJobTimeout <= 0 {
		t.Errorf("BuildJobTimeout must be positive, got %v", BuildJobTimeout)
	}
}

