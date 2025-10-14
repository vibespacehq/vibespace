package k8s

import (
	"testing"
)

func TestIsLocalCluster(t *testing.T) {
	tests := []struct {
		name     string
		context  string
		expected bool
	}{
		{
			name:     "k3d cluster",
			context:  "k3d-my-cluster",
			expected: true,
		},
		{
			name:     "minikube",
			context:  "minikube",
			expected: true,
		},
		{
			name:     "docker-desktop",
			context:  "docker-desktop",
			expected: true,
		},
		{
			name:     "rancher-desktop",
			context:  "rancher-desktop",
			expected: true,
		},
		{
			name:     "kind cluster",
			context:  "kind-my-cluster",
			expected: true,
		},
		{
			name:     "colima",
			context:  "colima",
			expected: true,
		},
		{
			name:     "remote AWS cluster",
			context:  "arn:aws:eks:us-west-2:123456789012:cluster/prod-cluster",
			expected: false,
		},
		{
			name:     "remote GKE cluster",
			context:  "gke_project-id_us-central1_cluster-name",
			expected: false,
		},
		{
			name:     "remote AKS cluster",
			context:  "azure-cluster",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalCluster(tt.context)
			if result != tt.expected {
				t.Errorf("isLocalCluster(%q) = %v, expected %v", tt.context, result, tt.expected)
			}
		})
	}
}

func TestIsContextRemote(t *testing.T) {
	tests := []struct {
		name     string
		context  string
		expected bool
	}{
		{
			name:     "local k3d cluster",
			context:  "k3d-my-cluster",
			expected: false,
		},
		{
			name:     "local minikube",
			context:  "minikube",
			expected: false,
		},
		{
			name:     "remote AWS cluster",
			context:  "arn:aws:eks:us-west-2:123456789012:cluster/prod-cluster",
			expected: true,
		},
		{
			name:     "remote GKE cluster",
			context:  "gke_project-id_us-central1_cluster-name",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsContextRemote(tt.context)
			if result != tt.expected {
				t.Errorf("IsContextRemote(%q) = %v, expected %v", tt.context, result, tt.expected)
			}
		})
	}
}
