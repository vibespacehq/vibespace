package knative

import (
	"testing"

	"vibespace/pkg/model"
)

func TestBuildInitContainers(t *testing.T) {
	manager := &ServiceManager{}

	tests := []struct {
		name          string
		req           *CreateServiceRequest
		wantCount     int
		wantNames     []string
		checkGitClone bool
	}{
		{
			name: "no init containers",
			req: &CreateServiceRequest{
				Persistent: false,
				GithubRepo: "",
			},
			wantCount: 0,
			wantNames: []string{},
		},
		{
			name: "only fix-permissions",
			req: &CreateServiceRequest{
				Persistent: true,
				GithubRepo: "",
			},
			wantCount: 1,
			wantNames: []string{"fix-permissions"},
		},
		{
			name: "fix-permissions and git-clone",
			req: &CreateServiceRequest{
				Persistent: true,
				GithubRepo: "https://github.com/user/repo.git",
			},
			wantCount:     2,
			wantNames:     []string{"fix-permissions", "git-clone"},
			checkGitClone: true,
		},
		{
			name: "only git-clone",
			req: &CreateServiceRequest{
				Persistent: false,
				GithubRepo: "https://github.com/user/repo.git",
			},
			wantCount:     1,
			wantNames:     []string{"git-clone"},
			checkGitClone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.buildInitContainers(tt.req)
			if len(got) != tt.wantCount {
				t.Errorf("buildInitContainers() returned %d containers, want %d", len(got), tt.wantCount)
			}

			// Check container names
			for i, wantName := range tt.wantNames {
				if i >= len(got) {
					t.Errorf("buildInitContainers() missing container at index %d", i)
					continue
				}

				container, ok := got[i].(map[string]interface{})
				if !ok {
					t.Errorf("buildInitContainers() container %d is not a map", i)
					continue
				}

				gotName, ok := container["name"].(string)
				if !ok {
					t.Errorf("buildInitContainers() container %d has no name", i)
					continue
				}

				if gotName != wantName {
					t.Errorf("buildInitContainers() container %d name = %q, want %q", i, gotName, wantName)
				}
			}

			// Check git clone container if requested
			if tt.checkGitClone {
				foundGitClone := false
				for _, c := range got {
					container, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					if name, ok := container["name"].(string); ok && name == "git-clone" {
						foundGitClone = true

						// Check args contain the repo URL
						args, ok := container["args"].([]string)
						if !ok || len(args) == 0 {
							t.Errorf("git-clone container has no args")
						} else if !contains(args[0], tt.req.GithubRepo) {
							t.Errorf("git-clone container args don't contain repo URL: got %q, want to contain %q", args[0], tt.req.GithubRepo)
						}
					}
				}

				if !foundGitClone {
					t.Errorf("buildInitContainers() did not create git-clone container")
				}
			}
		})
	}
}

func TestBuildVolumesAndMounts(t *testing.T) {
	manager := &ServiceManager{}

	tests := []struct {
		name            string
		req             *CreateServiceRequest
		wantVolumes     int
		wantMounts      int
		wantPVCName     string
		wantMountPath   string
	}{
		{
			name: "no volumes",
			req: &CreateServiceRequest{
				Persistent: false,
			},
			wantVolumes: 0,
			wantMounts:  0,
		},
		{
			name: "persistent volume",
			req: &CreateServiceRequest{
				Persistent: true,
				PVCName:    "vibespace-abc123-pvc",
			},
			wantVolumes:   1,
			wantMounts:    1,
			wantPVCName:   "vibespace-abc123-pvc",
			wantMountPath: "/vibespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			volumes, mounts := manager.buildVolumesAndMounts(tt.req)

			if len(volumes) != tt.wantVolumes {
				t.Errorf("buildVolumesAndMounts() returned %d volumes, want %d", len(volumes), tt.wantVolumes)
			}

			if len(mounts) != tt.wantMounts {
				t.Errorf("buildVolumesAndMounts() returned %d mounts, want %d", len(mounts), tt.wantMounts)
			}

			// Check PVC name if persistent
			if tt.req.Persistent && len(volumes) > 0 {
				volume, ok := volumes[0].(map[string]interface{})
				if !ok {
					t.Errorf("buildVolumesAndMounts() volume is not a map")
					return
				}

				pvc, ok := volume["persistentVolumeClaim"].(map[string]interface{})
				if !ok {
					t.Errorf("buildVolumesAndMounts() volume has no persistentVolumeClaim")
					return
				}

				claimName, ok := pvc["claimName"].(string)
				if !ok {
					t.Errorf("buildVolumesAndMounts() PVC has no claimName")
					return
				}

				if claimName != tt.wantPVCName {
					t.Errorf("buildVolumesAndMounts() PVC claimName = %q, want %q", claimName, tt.wantPVCName)
				}
			}

			// Check mount path if persistent
			if tt.req.Persistent && len(mounts) > 0 {
				mount, ok := mounts[0].(map[string]interface{})
				if !ok {
					t.Errorf("buildVolumesAndMounts() mount is not a map")
					return
				}

				mountPath, ok := mount["mountPath"].(string)
				if !ok {
					t.Errorf("buildVolumesAndMounts() mount has no mountPath")
					return
				}

				if mountPath != tt.wantMountPath {
					t.Errorf("buildVolumesAndMounts() mountPath = %q, want %q", mountPath, tt.wantMountPath)
				}
			}
		})
	}
}

func TestBuildEnvironment(t *testing.T) {
	manager := &ServiceManager{}

	tests := []struct {
		name             string
		req              *CreateServiceRequest
		wantEnvVars      []string
		checkAgentVar    bool
		wantAgent        string
	}{
		{
			name: "minimal environment",
			req: &CreateServiceRequest{
				VibespaceID: "abc123",
				Name:        "My Vibespace",
				ProjectName: "my-vibespace",
				Template:    "nextjs",
				Ports: model.Ports{
					Code:    8080,
					Preview: 3000,
					Prod:    3001,
				},
				Env: map[string]string{},
			},
			wantEnvVars: []string{
				"VIBESPACE_ID",
				"VIBESPACE_NAME",
				"VIBESPACE_PROJECT_NAME",
				"VIBESPACE_TEMPLATE",
				"VIBESPACE_CODE_PORT",
				"VIBESPACE_PREVIEW_PORT",
				"VIBESPACE_PROD_PORT",
			},
		},
		{
			name: "with agent",
			req: &CreateServiceRequest{
				VibespaceID: "abc123",
				Name:        "My Vibespace",
				ProjectName: "my-vibespace",
				Template:    "nextjs",
				Agent:       "claude",
				Ports: model.Ports{
					Code:    8080,
					Preview: 3000,
					Prod:    3001,
				},
				Env: map[string]string{},
			},
			wantEnvVars: []string{
				"VIBESPACE_ID",
				"VIBESPACE_NAME",
				"VIBESPACE_PROJECT_NAME",
				"VIBESPACE_TEMPLATE",
				"VIBESPACE_AGENT",
				"VIBESPACE_CODE_PORT",
				"VIBESPACE_PREVIEW_PORT",
				"VIBESPACE_PROD_PORT",
			},
			checkAgentVar: true,
			wantAgent:     "claude",
		},
		{
			name: "with custom env vars",
			req: &CreateServiceRequest{
				VibespaceID: "abc123",
				Name:        "My Vibespace",
				ProjectName: "my-vibespace",
				Template:    "nextjs",
				Ports: model.Ports{
					Code:    8080,
					Preview: 3000,
					Prod:    3001,
				},
				Env: map[string]string{
					"CUSTOM_VAR":    "value1",
					"ANOTHER_VAR":   "value2",
				},
			},
			wantEnvVars: []string{
				"CUSTOM_VAR",
				"ANOTHER_VAR",
				"VIBESPACE_ID",
				"VIBESPACE_NAME",
				"VIBESPACE_PROJECT_NAME",
				"VIBESPACE_TEMPLATE",
				"VIBESPACE_CODE_PORT",
				"VIBESPACE_PREVIEW_PORT",
				"VIBESPACE_PROD_PORT",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := manager.buildEnvironment(tt.req)

			// Check all expected env vars are present
			foundVars := make(map[string]string)
			for _, envVar := range got {
				envMap, ok := envVar.(map[string]interface{})
				if !ok {
					t.Errorf("buildEnvironment() env var is not a map")
					continue
				}

				name, ok := envMap["name"].(string)
				if !ok {
					t.Errorf("buildEnvironment() env var has no name")
					continue
				}

				value, ok := envMap["value"].(string)
				if !ok {
					t.Errorf("buildEnvironment() env var %q has no value", name)
					continue
				}

				foundVars[name] = value
			}

			// Check all expected vars are present
			for _, wantVar := range tt.wantEnvVars {
				if _, found := foundVars[wantVar]; !found {
					t.Errorf("buildEnvironment() missing env var %q", wantVar)
				}
			}

			// Check agent value if requested
			if tt.checkAgentVar {
				if agentValue, found := foundVars["VIBESPACE_AGENT"]; !found {
					t.Errorf("buildEnvironment() missing VIBESPACE_AGENT")
				} else if agentValue != tt.wantAgent {
					t.Errorf("buildEnvironment() VIBESPACE_AGENT = %q, want %q", agentValue, tt.wantAgent)
				}
			}

			// Verify port values (single-port Caddy architecture)
			// Caddy listens on 8080, routes internally to code:8081, preview:3000, prod:3001
			if caddyPort, found := foundVars["CADDY_PORT"]; found {
				if caddyPort != "8080" {
					t.Errorf("buildEnvironment() CADDY_PORT = %q, want %q", caddyPort, "8080")
				}
			}

			if codePort, found := foundVars["VIBESPACE_CODE_PORT"]; found {
				wantPort := "8081" // Internal code-server port (Caddy proxies 8080 → 8081)
				if codePort != wantPort {
					t.Errorf("buildEnvironment() VIBESPACE_CODE_PORT = %q, want %q", codePort, wantPort)
				}
			}

			if previewPort, found := foundVars["VIBESPACE_PREVIEW_PORT"]; found {
				wantPort := "3000" // Internal preview server port
				if previewPort != wantPort {
					t.Errorf("buildEnvironment() VIBESPACE_PREVIEW_PORT = %q, want %q", previewPort, wantPort)
				}
			}

			if prodPort, found := foundVars["VIBESPACE_PROD_PORT"]; found {
				wantPort := "3001" // Internal production server port
				if prodPort != wantPort {
					t.Errorf("buildEnvironment() VIBESPACE_PROD_PORT = %q, want %q", prodPort, wantPort)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		hasSubstring(s, substr))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
