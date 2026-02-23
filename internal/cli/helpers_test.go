package cli

import "regexp"

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
