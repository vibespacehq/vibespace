package ui

import "testing"

func TestStripAnsiCSI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"cursor movement", "\x1b[2Jclear\x1b[H", "clear"},
		{"cursor position", "\x1b[10;20Htext", "text"},
		{"erase line", "foo\x1b[Kbar", "foobar"},
		{"scroll up", "\x1b[2Stext", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripAnsi(tt.input)
			if got != tt.expected {
				t.Errorf("StripAnsi(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
