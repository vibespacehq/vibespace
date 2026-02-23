package cli

import "testing"

func TestLevenshtein(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"agent", "agnet", 2},    // transposition
		{"agent", "agen", 1},     // deletion
		{"agent", "agents", 1},   // insertion
		{"agent", "Agent", 1},    // substitution
		{"kitten", "sitting", 3}, // classic example
		{"a", "b", 1},
		{"abc", "xyz", 3},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_"+tt.b, func(t *testing.T) {
			got := levenshtein(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestLevenshteinSymmetric(t *testing.T) {
	pairs := [][2]string{
		{"agent", "agnet"},
		{"config", "confg"},
		{"forward", "froward"},
	}

	for _, p := range pairs {
		ab := levenshtein(p[0], p[1])
		ba := levenshtein(p[1], p[0])
		if ab != ba {
			t.Errorf("levenshtein(%q, %q) = %d but levenshtein(%q, %q) = %d — should be symmetric",
				p[0], p[1], ab, p[1], p[0], ba)
		}
	}
}

func TestSuggestCommand(t *testing.T) {
	commands := []string{"agent", "connect", "exec", "config", "start", "stop", "forward"}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"exact match", "agent", "agent"},
		{"close typo", "agnet", "agent"},
		{"off by one", "agen", "agent"},
		{"added char", "agents", "agent"},
		{"config typo", "confg", "config"},
		{"start typo", "strat", "start"},
		{"forward typo", "froward", "forward"},
		{"too far", "xyzabc", ""},
		{"empty input", "", ""},
		{"completely different", "kubernetes", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggestCommand(tt.input, commands)
			if got != tt.want {
				t.Errorf("suggestCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSuggestCommandEmptyList(t *testing.T) {
	got := suggestCommand("agent", []string{})
	if got != "" {
		t.Errorf("suggestCommand with empty list = %q, want empty", got)
	}
}

func TestSuggestVibespaceCommand(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"agnet", "agent"},
		{"conect", "connect"},
		{"exce", "exec"},
		{"confg", "config"},
		{"strat", "start"},
		{"sotp", "stop"},
		{"forwrd", "forward"},
		{"xyzabc", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := suggestVibespaceCommand(tt.input)
			if got != tt.want {
				t.Errorf("suggestVibespaceCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
