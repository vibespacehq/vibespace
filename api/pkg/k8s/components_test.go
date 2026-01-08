package k8s

import (
	"testing"
)

func TestAllComponentsReady(t *testing.T) {
	tests := []struct {
		name       string
		components ClusterComponents
		expected   bool
	}{
		{
			name: "all components ready",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: true, Healthy: true},
				Traefik:  ComponentStatus{Installed: true, Healthy: true},
				Registry: ComponentStatus{Installed: true, Healthy: true},
			},
			expected: true,
		},
		{
			name: "knative not healthy",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: true, Healthy: false},
				Traefik:  ComponentStatus{Installed: true, Healthy: true},
				Registry: ComponentStatus{Installed: true, Healthy: true},
			},
			expected: false,
		},
		{
			name: "traefik not installed",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: true, Healthy: true},
				Traefik:  ComponentStatus{Installed: false, Healthy: false},
				Registry: ComponentStatus{Installed: true, Healthy: true},
			},
			expected: false,
		},
		{
			name: "no components ready",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: false, Healthy: false},
				Traefik:  ComponentStatus{Installed: false, Healthy: false},
				Registry: ComponentStatus{Installed: false, Healthy: false},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.components.AllComponentsReady()
			if result != tt.expected {
				t.Errorf("AllComponentsReady() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestGetMissingComponents(t *testing.T) {
	tests := []struct {
		name       string
		components ClusterComponents
		expected   []string
	}{
		{
			name: "all components ready",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: true, Healthy: true},
				Traefik:  ComponentStatus{Installed: true, Healthy: true},
				Registry: ComponentStatus{Installed: true, Healthy: true},
			},
			expected: []string{},
		},
		{
			name: "knative missing",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: false, Healthy: false},
				Traefik:  ComponentStatus{Installed: true, Healthy: true},
				Registry: ComponentStatus{Installed: true, Healthy: true},
			},
			expected: []string{"knative"},
		},
		{
			name: "multiple components missing",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: false, Healthy: false},
				Traefik:  ComponentStatus{Installed: true, Healthy: false},
				Registry: ComponentStatus{Installed: false, Healthy: false},
			},
			expected: []string{"knative", "traefik", "registry"},
		},
		{
			name: "all components missing",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: false, Healthy: false},
				Traefik:  ComponentStatus{Installed: false, Healthy: false},
				Registry: ComponentStatus{Installed: false, Healthy: false},
			},
			expected: []string{"knative", "traefik", "registry"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.components.GetMissingComponents()
			if len(result) != len(tt.expected) {
				t.Errorf("GetMissingComponents() returned %d components, expected %d", len(result), len(tt.expected))
				return
			}
			for i, comp := range result {
				if comp != tt.expected[i] {
					t.Errorf("GetMissingComponents()[%d] = %q, expected %q", i, comp, tt.expected[i])
				}
			}
		})
	}
}

func TestGetStatusSummary(t *testing.T) {
	tests := []struct {
		name       string
		components ClusterComponents
		expected   string
	}{
		{
			name: "all ready",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: true, Healthy: true},
				Traefik:  ComponentStatus{Installed: true, Healthy: true},
				Registry: ComponentStatus{Installed: true, Healthy: true},
			},
			expected: "All components ready",
		},
		{
			name: "some missing",
			components: ClusterComponents{
				Knative:  ComponentStatus{Installed: false, Healthy: false},
				Traefik:  ComponentStatus{Installed: true, Healthy: true},
				Registry: ComponentStatus{Installed: true, Healthy: true},
			},
			expected: "Missing or unhealthy components: [knative]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.components.GetStatusSummary()
			if result != tt.expected {
				t.Errorf("GetStatusSummary() = %q, expected %q", result, tt.expected)
			}
		})
	}
}
