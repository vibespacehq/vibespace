package template

import (
	"testing"
)

func TestGetDockerfile(t *testing.T) {
	tests := []struct {
		name        string
		templateID  string
		agent       string
		wantErr     bool
		errContains string
	}{
		// Valid base images
		{
			name:       "base claude",
			templateID: "base",
			agent:      "claude",
			wantErr:    false,
		},
		{
			name:       "base codex",
			templateID: "base",
			agent:      "codex",
			wantErr:    false,
		},
		{
			name:       "base gemini",
			templateID: "base",
			agent:      "gemini",
			wantErr:    false,
		},
		// Valid template images
		{
			name:       "nextjs claude",
			templateID: "nextjs",
			agent:      "claude",
			wantErr:    false,
		},
		{
			name:       "vue codex",
			templateID: "vue",
			agent:      "codex",
			wantErr:    false,
		},
		{
			name:       "jupyter gemini",
			templateID: "jupyter",
			agent:      "gemini",
			wantErr:    false,
		},
		// Invalid template
		{
			name:        "invalid template",
			templateID:  "invalid",
			agent:       "claude",
			wantErr:     true,
			errContains: "invalid template",
		},
		// Invalid agent for base
		{
			name:        "base invalid agent",
			templateID:  "base",
			agent:       "invalid",
			wantErr:     true,
			errContains: "invalid agent",
		},
		// Invalid agent for template
		{
			name:        "nextjs invalid agent",
			templateID:  "nextjs",
			agent:       "invalid",
			wantErr:     true,
			errContains: "invalid agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDockerfile(tt.templateID, tt.agent)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetDockerfile() expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("GetDockerfile() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("GetDockerfile() unexpected error = %v", err)
				return
			}

			if len(got) == 0 {
				t.Errorf("GetDockerfile() returned empty Dockerfile")
			}

			// Verify it's actually a Dockerfile by checking for common directives
			content := string(got)
			if !contains(content, "FROM") {
				t.Errorf("GetDockerfile() result doesn't contain FROM directive")
			}
		})
	}
}

func TestGetAgentMD(t *testing.T) {
	tests := []struct {
		name        string
		templateID  string
		agent       string
		wantErr     bool
		errContains string
	}{
		// Valid base images
		{
			name:       "base claude",
			templateID: "base",
			agent:      "claude",
			wantErr:    false,
		},
		{
			name:       "base codex",
			templateID: "base",
			agent:      "codex",
			wantErr:    false,
		},
		{
			name:       "base gemini",
			templateID: "base",
			agent:      "gemini",
			wantErr:    false,
		},
		// Valid template images
		{
			name:       "nextjs claude",
			templateID: "nextjs",
			agent:      "claude",
			wantErr:    false,
		},
		{
			name:       "vue codex",
			templateID: "vue",
			agent:      "codex",
			wantErr:    false,
		},
		{
			name:       "jupyter gemini",
			templateID: "jupyter",
			agent:      "gemini",
			wantErr:    false,
		},
		// Invalid template
		{
			name:        "invalid template",
			templateID:  "invalid",
			agent:       "claude",
			wantErr:     true,
			errContains: "invalid template",
		},
		// Invalid agent for base
		{
			name:        "base invalid agent",
			templateID:  "base",
			agent:       "invalid",
			wantErr:     true,
			errContains: "invalid agent",
		},
		// Invalid agent for template
		{
			name:        "nextjs invalid agent",
			templateID:  "nextjs",
			agent:       "invalid",
			wantErr:     true,
			errContains: "invalid agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetAgentMD(tt.templateID, tt.agent)

			if tt.wantErr {
				if err == nil {
					t.Errorf("GetAgentMD() expected error, got nil")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("GetAgentMD() error = %v, want error containing %v", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("GetAgentMD() unexpected error = %v", err)
				return
			}

			if len(got) == 0 {
				t.Errorf("GetAgentMD() returned empty markdown")
			}
		})
	}
}

func TestGetAllTemplateIDs(t *testing.T) {
	got := GetAllTemplateIDs()

	expected := []string{"nextjs", "vue", "jupyter"}

	if len(got) != len(expected) {
		t.Errorf("GetAllTemplateIDs() returned %d items, want %d", len(got), len(expected))
	}

	for i, id := range expected {
		if got[i] != id {
			t.Errorf("GetAllTemplateIDs()[%d] = %v, want %v", i, got[i], id)
		}
	}
}

func TestGetAllAgents(t *testing.T) {
	got := GetAllAgents()

	expected := []string{"claude", "codex", "gemini"}

	if len(got) != len(expected) {
		t.Errorf("GetAllAgents() returned %d items, want %d", len(got), len(expected))
	}

	for i, agent := range expected {
		if got[i] != agent {
			t.Errorf("GetAllAgents()[%d] = %v, want %v", i, got[i], agent)
		}
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
