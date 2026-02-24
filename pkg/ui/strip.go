package ui

import "regexp"

// ansiRegex matches all ANSI CSI escape sequences (SGR, cursor movement, erase, etc.).
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// StripAnsi removes ANSI escape sequences from a string.
func StripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
