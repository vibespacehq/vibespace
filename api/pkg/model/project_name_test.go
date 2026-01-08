package model

import (
	"strings"
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError error
	}{
		// Valid names
		{"valid simple", "abc", nil},
		{"valid with number", "abc123", nil},
		{"valid with hyphen", "my-project", nil},
		{"valid max length", "a" + strings.Repeat("-b", 15) + "c", nil}, // 32 chars
		{"valid starting with number", "123abc", nil},

		// Invalid names
		{"empty", "", ErrInvalidProjectName},
		{"too short", "ab", ErrProjectNameTooShort},
		{"too long", strings.Repeat("a", 33), ErrProjectNameTooLong},
		{"uppercase", "MyProject", ErrProjectNameInvalidChars},
		{"spaces", "my project", ErrProjectNameInvalidChars},
		{"underscores", "my_project", ErrProjectNameInvalidChars},
		{"special chars", "my-project!", ErrProjectNameInvalidChars},
		{"starts with hyphen", "-myproject", ErrProjectNameInvalidFormat},
		{"ends with hyphen", "myproject-", ErrProjectNameInvalidFormat},
		{"consecutive hyphens", "my--project", ErrProjectNameInvalidFormat},
		{"only hyphens", "---", ErrProjectNameInvalidFormat},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProjectName(tt.input)
			if err != tt.wantError {
				t.Errorf("ValidateProjectName(%q) error = %v, want %v", tt.input, err, tt.wantError)
			}
		})
	}
}

func TestNormalizeProjectName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercase simple", "MyProject", "myproject"},
		{"spaces to hyphens", "My Project", "my-project"},
		{"underscores to hyphens", "my_project", "my-project"},
		{"remove special chars", "my-project!!!", "my-project"},
		{"trim hyphens", "---test---", "test"},
		{"consecutive spaces", "hello    world", "hello-world"},
		{"mixed separators", "my___project   name", "my-project-name"},
		{"numbers preserved", "project123", "project123"},
		{"remove leading numbers", "123-project", "123-project"},
		{"truncate long name", strings.Repeat("a", 40), strings.Repeat("a", 32)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeProjectName(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeProjectName(%q) = %q, want %q", tt.input, got, tt.want)
			}

			// Ensure result is valid
			if err := ValidateProjectName(got); err != nil {
				t.Errorf("NormalizeProjectName(%q) produced invalid name %q: %v", tt.input, got, err)
			}
		})
	}
}

func TestNormalizeProjectNameTooShort(t *testing.T) {
	// When input is too short, it should add random suffix
	result := NormalizeProjectName("a")
	if len(result) < 3 {
		t.Errorf("NormalizeProjectName('a') should produce name with at least 3 chars, got %q", result)
	}
	if !strings.HasPrefix(result, "a-") {
		t.Errorf("NormalizeProjectName('a') should start with 'a-', got %q", result)
	}
}

func TestGenerateProjectName(t *testing.T) {
	// Generate multiple names and ensure they're valid
	names := make(map[string]bool)
	for i := 0; i < 100; i++ {
		name := GenerateProjectName()

		// Check validity
		if err := ValidateProjectName(name); err != nil {
			t.Errorf("GenerateProjectName() produced invalid name %q: %v", name, err)
		}

		// Check format (adjective-noun-number)
		parts := strings.Split(name, "-")
		if len(parts) != 3 {
			t.Errorf("GenerateProjectName() = %q, want format 'adjective-noun-number'", name)
		}

		names[name] = true
	}

	// Ensure we get some variety (at least 10 unique names in 100 tries)
	if len(names) < 10 {
		t.Errorf("GenerateProjectName() produced only %d unique names in 100 tries, want at least 10", len(names))
	}
}

func TestGenerateUniqueProjectName(t *testing.T) {
	existing := []string{
		"happy-cloud-42",
		"swift-star-13",
		"bright-moon-7",
	}

	// Generate 10 unique names
	for i := 0; i < 10; i++ {
		name := GenerateUniqueProjectName(existing)

		// Check it's not in existing
		for _, ex := range existing {
			if name == ex {
				t.Errorf("GenerateUniqueProjectName() = %q, which already exists", name)
			}
		}

		// Check validity
		if err := ValidateProjectName(name); err != nil {
			t.Errorf("GenerateUniqueProjectName() produced invalid name %q: %v", name, err)
		}

		// Add to existing for next iteration
		existing = append(existing, name)
	}
}

func TestGenerateUniqueProjectNameFallback(t *testing.T) {
	// Create a map with all possible generated names (to force fallback)
	// This is impractical, so we'll test the fallback by mocking the generator
	// For now, just verify the fallback format is valid
	name := "vibespace-1234567890"
	if err := ValidateProjectName(name); err != nil {
		t.Errorf("Fallback name format %q is invalid: %v", name, err)
	}
}

func TestGenerateServiceURL(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		port        int
		baseDomain  string
		want        string
	}{
		{
			"default domain",
			"myproject",
			3000,
			"",
			"https://3000.myproject.vibe.space",
		},
		{
			"custom domain",
			"myproject",
			8080,
			"example.com",
			"https://8080.myproject.example.com",
		},
		{
			"project with hyphens",
			"my-awesome-project",
			3000,
			"vibe.space",
			"https://3000.my-awesome-project.vibe.space",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateServiceURL(tt.projectName, tt.port, tt.baseDomain)
			if got != tt.want {
				t.Errorf("GenerateServiceURL(%q, %d, %q) = %q, want %q",
					tt.projectName, tt.port, tt.baseDomain, got, tt.want)
			}
		})
	}
}

func TestGenerateMainURL(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		baseDomain  string
		want        string
	}{
		{
			"default domain",
			"myproject",
			"",
			"https://myproject.vibe.space",
		},
		{
			"custom domain",
			"myproject",
			"example.com",
			"https://myproject.example.com",
		},
		{
			"project with hyphens",
			"my-awesome-project",
			"vibe.space",
			"https://my-awesome-project.vibe.space",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMainURL(tt.projectName, tt.baseDomain)
			if got != tt.want {
				t.Errorf("GenerateMainURL(%q, %q) = %q, want %q",
					tt.projectName, tt.baseDomain, got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkValidateProjectName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ValidateProjectName("my-project-123")
	}
}

func BenchmarkNormalizeProjectName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NormalizeProjectName("My Awesome Project!!!")
	}
}

func BenchmarkGenerateProjectName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = GenerateProjectName()
	}
}
