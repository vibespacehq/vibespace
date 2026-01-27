package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	vserrors "github.com/yagizdagabak/vibespace/pkg/errors"
	"github.com/yagizdagabak/vibespace/pkg/ui"
)

// Output handles all CLI output with support for TTY detection, colors, JSON mode, etc.
type Output struct {
	stdoutTTY bool
	stdinTTY  bool
	noColor   bool
	jsonMode  bool
	plainMode bool
	header    bool  // Include headers in plain mode
	verbosity int   // -1=quiet, 0=normal, 1=verbose

	mu sync.Mutex // protects concurrent writes

	// Lipgloss styles
	styles ui.Styles
}

// OutputConfig holds configuration for creating a new Output instance
type OutputConfig struct {
	JSONMode  bool
	PlainMode bool
	Header    bool
	Verbosity int // -1=quiet, 0=normal, 1=verbose
	NoColor   bool
}

// out is the global Output instance initialized before commands run
var out *Output

// initOutput initializes the global Output instance with the given config
func initOutput(cfg OutputConfig) {
	out = NewOutput(cfg)
}

// getOutput returns the global Output instance, initializing with defaults if needed
func getOutput() *Output {
	if out == nil {
		out = NewOutput(OutputConfig{})
	}
	return out
}

// NewOutput creates a new Output instance with the given configuration
func NewOutput(cfg OutputConfig) *Output {
	stdoutTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	stdinTTY := isatty.IsTerminal(os.Stdin.Fd()) || isatty.IsCygwinTerminal(os.Stdin.Fd())

	// Determine if colors should be disabled
	noColor := cfg.NoColor
	if !noColor {
		// Check environment variables
		if os.Getenv("NO_COLOR") != "" {
			noColor = true
		}
		if os.Getenv("TERM") == "dumb" {
			noColor = true
		}
		// Disable colors when not a TTY (piping to file, etc.)
		if !stdoutTTY {
			noColor = true
		}
	}

	// Plain mode implies no color
	if cfg.PlainMode {
		noColor = true
	}

	// Setup styles based on color mode
	var styles ui.Styles
	if noColor {
		styles = ui.PlainStyles()
	} else {
		styles = ui.NewStyles()
	}

	return &Output{
		stdoutTTY: stdoutTTY,
		stdinTTY:  stdinTTY,
		noColor:   noColor,
		jsonMode:  cfg.JSONMode,
		plainMode: cfg.PlainMode,
		header:    cfg.Header,
		verbosity: cfg.Verbosity,
		styles:    styles,
	}
}

// IsTTY returns true if stdout is a terminal
func (o *Output) IsTTY() bool {
	return o.stdoutTTY
}

// CanPrompt returns true if we can interactively prompt the user
// This requires stdin to be a TTY
func (o *Output) CanPrompt() bool {
	return o.stdinTTY
}

// IsJSONMode returns true if JSON output mode is enabled
func (o *Output) IsJSONMode() bool {
	return o.jsonMode
}

// IsPlainMode returns true if plain output mode is enabled
func (o *Output) IsPlainMode() bool {
	return o.plainMode
}

// IsQuiet returns true if quiet mode is enabled
func (o *Output) IsQuiet() bool {
	return o.verbosity < 0
}

// NoColor returns true if colors are disabled
func (o *Output) NoColor() bool {
	return o.noColor
}

// Header returns true if headers should be included in plain mode
func (o *Output) Header() bool {
	return o.header
}

// Step prints a progress step message with an arrow prefix
func (o *Output) Step(format string, args ...interface{}) {
	if o.jsonMode || o.IsQuiet() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	prefix := ui.StepPrefix(o.noColor)
	if !o.noColor {
		prefix = lipgloss.NewStyle().Foreground(ui.Teal).Render(prefix)
	}
	fmt.Printf("%s %s\n", prefix, msg)
}

// Success prints a success message with a checkmark prefix
func (o *Output) Success(format string, args ...interface{}) {
	if o.jsonMode || o.IsQuiet() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	prefix := ui.SuccessPrefix(o.noColor)
	if !o.noColor {
		prefix = o.styles.Success.Render(prefix)
	}
	fmt.Printf("%s %s\n", prefix, msg)
}

// Warning prints a warning message with a warning prefix
func (o *Output) Warning(format string, args ...interface{}) {
	if o.jsonMode || o.IsQuiet() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	prefix := ui.WarningPrefix(o.noColor)
	if !o.noColor {
		prefix = o.styles.Warning.Render(prefix)
	}
	fmt.Printf("%s %s\n", prefix, msg)
}

// Fail prints an error message to stderr with an error prefix
func (o *Output) Fail(format string, args ...interface{}) {
	if o.jsonMode {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	prefix := ui.ErrorPrefix(o.noColor)
	if !o.noColor {
		prefix = o.styles.Error.Render(prefix)
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", prefix, msg)
}

// JSON outputs the value as JSON
func (o *Output) JSON(v interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// Confirm prompts the user for yes/no confirmation
// Returns true if user confirms, false otherwise
// If defaultNo is true, pressing Enter defaults to "no"
// If stdin is not a TTY, returns an error
func (o *Output) Confirm(prompt string, defaultNo bool) (bool, error) {
	if !o.CanPrompt() {
		return false, fmt.Errorf("cannot prompt for confirmation (stdin is not a terminal)")
	}

	suffix := " [y/N] "
	if !defaultNo {
		suffix = " [Y/n] "
	}

	o.mu.Lock()
	fmt.Print(prompt + suffix)
	o.mu.Unlock()

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	if response == "" {
		return !defaultNo, nil
	}

	return response == "y" || response == "yes", nil
}

// Color helpers for use in tables and other formatted output

// Green returns the string in success/green color (if colors enabled)
func (o *Output) Green(s string) string {
	if o.noColor {
		return s
	}
	return o.styles.Success.Render(s)
}

// Yellow returns the string in warning/yellow color (if colors enabled)
func (o *Output) Yellow(s string) string {
	if o.noColor {
		return s
	}
	return o.styles.Warning.Render(s)
}

// Red returns the string in error/red color (if colors enabled)
func (o *Output) Red(s string) string {
	if o.noColor {
		return s
	}
	return o.styles.Error.Render(s)
}

// Bold returns the string in bold (if colors enabled)
func (o *Output) Bold(s string) string {
	if o.noColor {
		return s
	}
	return o.styles.Bold.Render(s)
}

// Dim returns the string in dim/faint color (if colors enabled)
func (o *Output) Dim(s string) string {
	if o.noColor {
		return s
	}
	return o.styles.Dim.Render(s)
}

// Teal returns the string in brand teal color (if colors enabled)
func (o *Output) Teal(s string) string {
	if o.noColor {
		return s
	}
	return lipgloss.NewStyle().Foreground(ui.Teal).Render(s)
}

// Pink returns the string in brand pink color (if colors enabled)
func (o *Output) Pink(s string) string {
	if o.noColor {
		return s
	}
	return lipgloss.NewStyle().Foreground(ui.Pink).Render(s)
}

// Orange returns the string in brand orange color (if colors enabled)
func (o *Output) Orange(s string) string {
	if o.noColor {
		return s
	}
	return lipgloss.NewStyle().Foreground(ui.Orange).Render(s)
}

// Table prints a formatted table using ui.NewTable
func (o *Output) Table(headers []string, rows [][]string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.plainMode {
		fmt.Print(ui.PlainTableWithHeader(headers, rows, o.header))
	} else {
		fmt.Println(ui.NewTable(headers, rows, o.noColor))
	}
}

// Package-level convenience functions that use the global Output instance

// printStep prints a progress step message (uses global Output)
func printStep(format string, args ...interface{}) {
	getOutput().Step(format, args...)
}

// printSuccess prints a success message (uses global Output)
func printSuccess(format string, args ...interface{}) {
	getOutput().Success(format, args...)
}

// printWarning prints a warning message (uses global Output)
func printWarning(format string, args ...interface{}) {
	getOutput().Warning(format, args...)
}

// printError prints an error message to stderr (uses global Output)
func printError(format string, args ...interface{}) {
	getOutput().Fail(format, args...)
}

// getErrorHint returns a helpful hint for common errors
func getErrorHint(err error) string {
	switch {
	case errors.Is(err, vserrors.ErrVibespaceNotFound):
		return "Use 'vibespace list' to see available vibespaces"
	case errors.Is(err, vserrors.ErrAgentNotFound):
		return "Use 'vibespace <name> agent list' to see available agents"
	case errors.Is(err, vserrors.ErrClusterNotInitialized):
		return "Run 'vibespace init' to initialize the cluster"
	case errors.Is(err, vserrors.ErrClusterNotRunning):
		return "Run 'vibespace init' to start the cluster"
	case errors.Is(err, vserrors.ErrDaemonNotRunning):
		return "The daemon will auto-start on next command"
	case errors.Is(err, vserrors.ErrForwardNotFound):
		return "Use 'vibespace <name> forward list' to see active forwards"
	case errors.Is(err, vserrors.ErrNoAgents):
		return "Use 'vibespace <name> agent create' to add an agent"
	default:
		return ""
	}
}
