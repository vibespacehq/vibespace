package template

import (
	"testing"
)

func TestGetAllSupportFiles(t *testing.T) {
	files := GetAllSupportFiles()

	// Expected files
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

	// Check all expected files exist
	for _, name := range expectedFiles {
		content, ok := files[name]
		if !ok {
			t.Errorf("GetAllSupportFiles() missing expected file: %s", name)
			continue
		}
		if len(content) == 0 {
			t.Errorf("GetAllSupportFiles()[%s] is empty", name)
		}
	}

	// Check count matches
	if len(files) != len(expectedFiles) {
		t.Errorf("GetAllSupportFiles() returned %d files, want %d", len(files), len(expectedFiles))
	}
}

func TestEmbeddedSupportFilesNotEmpty(t *testing.T) {
	files := GetAllSupportFiles()

	for name, content := range files {
		if len(content) == 0 {
			t.Errorf("Support file %s is empty (check go:embed directive)", name)
		}
	}
}
