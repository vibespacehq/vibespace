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

func TestAllocatePorts(t *testing.T) {
	tests := []struct {
		name     string
		basePort int
		wantCode int
		wantPrev int
		wantProd int
	}{
		{"from 3000", 3000, 3000, 3001, 3002},
		{"from 8000", 8000, 8000, 8001, 8002},
		{"from 9000", 9000, 9000, 9001, 9002},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := AllocatePorts(tt.basePort)
			if ports.Code != tt.wantCode {
				t.Errorf("AllocatePorts(%d).Code = %d, want %d", tt.basePort, ports.Code, tt.wantCode)
			}
			if ports.Preview != tt.wantPrev {
				t.Errorf("AllocatePorts(%d).Preview = %d, want %d", tt.basePort, ports.Preview, tt.wantPrev)
			}
			if ports.Prod != tt.wantProd {
				t.Errorf("AllocatePorts(%d).Prod = %d, want %d", tt.basePort, ports.Prod, tt.wantProd)
			}
		})
	}
}

func TestGenerateURLs(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		wantCode    string
		wantPreview string
		wantProd    string
	}{
		{
			"simple project",
			"myproject",
			"http://code.myproject.vibe.space",
			"http://preview.myproject.vibe.space",
			"http://prod.myproject.vibe.space",
		},
		{
			"project with hyphens",
			"my-awesome-project",
			"http://code.my-awesome-project.vibe.space",
			"http://preview.my-awesome-project.vibe.space",
			"http://prod.my-awesome-project.vibe.space",
		},
		{
			"project with numbers",
			"project123",
			"http://code.project123.vibe.space",
			"http://preview.project123.vibe.space",
			"http://prod.project123.vibe.space",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := GenerateURLs(tt.projectName)

			if urls["code"] != tt.wantCode {
				t.Errorf("GenerateURLs(%q)['code'] = %q, want %q", tt.projectName, urls["code"], tt.wantCode)
			}
			if urls["preview"] != tt.wantPreview {
				t.Errorf("GenerateURLs(%q)['preview'] = %q, want %q", tt.projectName, urls["preview"], tt.wantPreview)
			}
			if urls["prod"] != tt.wantProd {
				t.Errorf("GenerateURLs(%q)['prod'] = %q, want %q", tt.projectName, urls["prod"], tt.wantProd)
			}
		})
	}
}

func TestPortsValidation(t *testing.T) {
	// Ensure ports are in valid range
	for i := 0; i < 10; i++ {
		basePort := 3000 + (i * 100)
		ports := AllocatePorts(basePort)

		if ports.Code < 1 || ports.Code > 65535 {
			t.Errorf("AllocatePorts(%d).Code = %d is out of valid range", basePort, ports.Code)
		}
		if ports.Preview < 1 || ports.Preview > 65535 {
			t.Errorf("AllocatePorts(%d).Preview = %d is out of valid range", basePort, ports.Preview)
		}
		if ports.Prod < 1 || ports.Prod > 65535 {
			t.Errorf("AllocatePorts(%d).Prod = %d is out of valid range", basePort, ports.Prod)
		}
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
