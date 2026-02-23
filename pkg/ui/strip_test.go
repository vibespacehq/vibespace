package ui

import "regexp"

// ansiRegex matches ANSI SGR escape sequences.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// StripAnsi removes ANSI escape sequences from a string.
// Test-only helper used across ui and cli tests.
func StripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
