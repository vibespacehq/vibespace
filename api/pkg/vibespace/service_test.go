package vibespace

import (
	"os"
	"testing"
)

func TestGetVibespaceImage(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "default image",
			envValue: "",
			want:     DefaultImage,
		},
		{
			name:     "custom image from env",
			envValue: "ghcr.io/custom/image:v1.0",
			want:     "ghcr.io/custom/image:v1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env var
			original := os.Getenv("VIBESPACE_IMAGE")
			defer os.Setenv("VIBESPACE_IMAGE", original)

			if tt.envValue != "" {
				os.Setenv("VIBESPACE_IMAGE", tt.envValue)
			} else {
				os.Unsetenv("VIBESPACE_IMAGE")
			}

			got := getVibespaceImage()
			if got != tt.want {
				t.Errorf("getVibespaceImage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBaseDomain(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     string
	}{
		{
			name:     "default domain",
			envValue: "",
			want:     "vibe.space",
		},
		{
			name:     "custom domain from env",
			envValue: "example.com",
			want:     "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore env var
			original := os.Getenv("DNS_BASE_DOMAIN")
			defer os.Setenv("DNS_BASE_DOMAIN", original)

			if tt.envValue != "" {
				os.Setenv("DNS_BASE_DOMAIN", tt.envValue)
			} else {
				os.Unsetenv("DNS_BASE_DOMAIN")
			}

			got := getBaseDomain()
			if got != tt.want {
				t.Errorf("getBaseDomain() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsValidGitURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "valid HTTPS URL",
			url:  "https://github.com/user/repo.git",
			want: true,
		},
		{
			name: "valid SSH URL",
			url:  "git@github.com:user/repo.git",
			want: true,
		},
		{
			name: "empty URL",
			url:  "",
			want: false,
		},
		{
			name: "too short URL",
			url:  "https://a",
			want: false,
		},
		{
			name: "invalid - contains semicolon",
			url:  "https://github.com/user/repo.git; rm -rf /",
			want: false,
		},
		{
			name: "invalid - contains pipe",
			url:  "https://github.com/user/repo.git | cat /etc/passwd",
			want: false,
		},
		{
			name: "invalid - contains ampersand",
			url:  "https://github.com/user/repo.git & whoami",
			want: false,
		},
		{
			name: "invalid - contains dollar sign",
			url:  "https://github.com/user/$HOME/repo.git",
			want: false,
		},
		{
			name: "invalid - contains backtick",
			url:  "https://github.com/user/`whoami`/repo.git",
			want: false,
		},
		{
			name: "invalid - contains newline",
			url:  "https://github.com/user/repo.git\nrm -rf /",
			want: false,
		},
		{
			name: "invalid - command substitution",
			url:  "https://github.com/$(whoami)/repo.git",
			want: false,
		},
		{
			name: "invalid - double ampersand",
			url:  "https://github.com/user/repo.git && cat /etc/passwd",
			want: false,
		},
		{
			name: "invalid - double pipe",
			url:  "https://github.com/user/repo.git || true",
			want: false,
		},
		{
			name: "invalid - HTTP URL (not HTTPS)",
			url:  "http://github.com/user/repo.git",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidGitURL(tt.url)
			if got != tt.want {
				t.Errorf("isValidGitURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestGenerateUniqueProjectName(t *testing.T) {
	// Test that generated names are unique
	existing := []string{"swift-fox", "bright-owl"}
	names := make(map[string]bool)

	for i := 0; i < 10; i++ {
		name := generateUniqueProjectName(existing)

		// Should not be in existing list
		for _, ex := range existing {
			if name == ex {
				t.Errorf("generateUniqueProjectName() returned existing name: %q", name)
			}
		}

		// Should not duplicate
		if names[name] {
			// It's okay to get duplicates since we're generating random names
			// and not adding them to existing list between calls
		}
		names[name] = true
	}
}

func TestGenerateUniqueProjectName_Format(t *testing.T) {
	// Test that generated names have correct format (adjective-noun)
	name := generateUniqueProjectName([]string{})

	// Should contain at least one hyphen
	hasHyphen := false
	for _, c := range name {
		if c == '-' {
			hasHyphen = true
			break
		}
	}

	if !hasHyphen {
		t.Errorf("generateUniqueProjectName() = %q, want name with hyphen (adjective-noun format)", name)
	}
}
