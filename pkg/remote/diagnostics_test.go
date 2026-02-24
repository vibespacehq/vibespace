package remote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckKubeconfig(t *testing.T) {
	t.Run("valid kubeconfig", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)

		vsDir := filepath.Join(dir, ".vibespace")
		os.MkdirAll(vsDir, 0755)
		kubeconfig := filepath.Join(vsDir, RemoteKubeconfig)
		os.WriteFile(kubeconfig, []byte("apiVersion: v1\nkind: Config"), 0600)

		result := checkKubeconfig()
		if !result.Status {
			t.Errorf("expected valid kubeconfig, got: %s", result.Message)
		}
	})

	t.Run("missing kubeconfig", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)

		// Create .vibespace dir but no kubeconfig file
		os.MkdirAll(filepath.Join(dir, ".vibespace"), 0755)

		result := checkKubeconfig()
		if result.Status {
			t.Error("expected failure for missing kubeconfig")
		}
	})
}
