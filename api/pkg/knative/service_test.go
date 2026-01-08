package knative

import (
	"testing"
)

func TestBuildInitContainers(t *testing.T) {
	manager := &ServiceManager{}

	tests := []struct {
		name      string
		req       *CreateServiceRequest
		wantCount int
		wantNames []string
	}{
		{
			name: "no init containers",
			req: &CreateServiceRequest{
				Persistent: false,
			},
			wantCount: 0,
			wantNames: []string{},
		},
		{
			name: "fix-permissions for persistent storage",
			req: &CreateServiceRequest{
				Persistent: true,
			},
			wantCount: 1,
			wantNames: []string{"fix-permissions"},
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
		name        string
		req         *CreateServiceRequest
		wantEnvVars []string
	}{
		{
			name: "minimal environment",
			req: &CreateServiceRequest{
				VibespaceID: "abc123",
				Name:        "My Vibespace",
				ProjectName: "my-vibespace",
				ClaudeID:    "1",
				Env:         map[string]string{},
			},
			wantEnvVars: []string{
				"VIBESPACE_ID",
				"VIBESPACE_PROJECT",
				"VIBESPACE_CLAUDE_ID",
				"NATS_URL",
			},
		},
		{
			name: "with custom env vars",
			req: &CreateServiceRequest{
				VibespaceID: "abc123",
				Name:        "My Vibespace",
				ProjectName: "my-vibespace",
				ClaudeID:    "2",
				Env: map[string]string{
					"ANTHROPIC_API_KEY": "sk-xxx",
				},
			},
			wantEnvVars: []string{
				"ANTHROPIC_API_KEY",
				"VIBESPACE_ID",
				"VIBESPACE_PROJECT",
				"VIBESPACE_CLAUDE_ID",
				"NATS_URL",
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
		})
	}
}

