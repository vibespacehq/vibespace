package vibespace

import (
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{
		"ab",
		"my-project",
		"test123",
		"a1",
		"abc-def-ghi",
		"a-b",
	}

	for _, name := range valid {
		t.Run("valid/"+name, func(t *testing.T) {
			if err := ValidateName(name); err != nil {
				t.Errorf("ValidateName(%q) = %v, want nil", name, err)
			}
		})
	}

	invalid := []struct {
		name    string
		wantMsg string
	}{
		{"", "empty"},
		{"a", "at least 2"},
		{strings.Repeat("a", 64), "exceed 63"},
		{"Abc", "lowercase letter"},
		{"1abc", "start with a lowercase"},
		{"abc-", "end with a lowercase letter or number"},
		{"ab--cd", "consecutive hyphens"},
		{"ab_cd", "only contain lowercase"},
		{"ab cd", "only contain lowercase"},
		{"AB", "start with a lowercase"},
	}

	for _, tt := range invalid {
		t.Run("invalid/"+tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if err == nil {
				t.Errorf("ValidateName(%q) = nil, want error containing %q", tt.name, tt.wantMsg)
				return
			}
			if !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("ValidateName(%q) = %q, want error containing %q", tt.name, err.Error(), tt.wantMsg)
			}
		})
	}
}
