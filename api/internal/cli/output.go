package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

// Output handles all CLI output with support for TTY detection, colors, JSON mode, etc.
type Output struct {
	stdoutTTY bool
	stdinTTY  bool
	noColor   bool
	jsonMode  bool
	plainMode bool
	verbosity int // -1=quiet, 0=normal, 1=verbose

	mu sync.Mutex // protects concurrent writes

	// Color functions - set based on noColor
	green  func(a ...interface{}) string
	yellow func(a ...interface{}) string
	red    func(a ...interface{}) string
	cyan   func(a ...interface{}) string
	bold   func(a ...interface{}) string
}

// OutputConfig holds configuration for creating a new Output instance
type OutputConfig struct {
	JSONMode  bool
	PlainMode bool
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

	o := &Output{
		stdoutTTY: stdoutTTY,
		stdinTTY:  stdinTTY,
		noColor:   noColor,
		jsonMode:  cfg.JSONMode,
		plainMode: cfg.PlainMode,
		verbosity: cfg.Verbosity,
	}

	// Setup color functions
	if noColor {
		// No-op functions when colors are disabled
		o.green = fmt.Sprint
		o.yellow = fmt.Sprint
		o.red = fmt.Sprint
		o.cyan = fmt.Sprint
		o.bold = fmt.Sprint
	} else {
		o.green = color.New(color.FgGreen).SprintFunc()
		o.yellow = color.New(color.FgYellow).SprintFunc()
		o.red = color.New(color.FgRed).SprintFunc()
		o.cyan = color.New(color.FgCyan).SprintFunc()
		o.bold = color.New(color.Bold).SprintFunc()
	}

	return o
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


// Step prints a progress step message with a cyan arrow prefix
func (o *Output) Step(format string, args ...interface{}) {
	if o.jsonMode || o.IsQuiet() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", o.cyan("->"), msg)
}

// Success prints a success message with a green checkmark prefix
func (o *Output) Success(format string, args ...interface{}) {
	if o.jsonMode || o.IsQuiet() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", o.green("ok"), msg)
}

// Warning prints a warning message with a yellow warning prefix
func (o *Output) Warning(format string, args ...interface{}) {
	if o.jsonMode || o.IsQuiet() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s\n", o.yellow("!!"), msg)
}

// Fail prints an error message to stderr with a red X prefix
func (o *Output) Fail(format string, args ...interface{}) {
	if o.jsonMode {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", o.red("error"), msg)
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

// Green returns the string wrapped in green color (if colors enabled)
func (o *Output) Green(s string) string {
	return o.green(s)
}

// Yellow returns the string wrapped in yellow color (if colors enabled)
func (o *Output) Yellow(s string) string {
	return o.yellow(s)
}

// Red returns the string wrapped in red color (if colors enabled)
func (o *Output) Red(s string) string {
	return o.red(s)
}


// Bold returns the string wrapped in bold (if colors enabled)
func (o *Output) Bold(s string) string {
	return o.bold(s)
}

// Dim returns the string in dim/faint color (if colors enabled)
func (o *Output) Dim(s string) string {
	if o.noColor || !o.stdoutTTY {
		return s
	}
	return color.New(color.Faint).Sprint(s)
}

// Cyan returns the string in cyan color (if colors enabled)
func (o *Output) Cyan(s string) string {
	if o.noColor || !o.stdoutTTY {
		return s
	}
	return color.CyanString(s)
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
