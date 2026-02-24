package cli

import "github.com/vibespacehq/vibespace/pkg/ui"

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	return ui.StripAnsi(s)
}
