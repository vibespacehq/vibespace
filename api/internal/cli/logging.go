package cli

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LogMode determines the logging behavior
type LogMode string

const (
	LogModeCLI    LogMode = "cli"    // Interactive CLI commands
	LogModeTUI    LogMode = "tui"    // Terminal UI
	LogModeDaemon LogMode = "daemon" // Background daemon
)

// LogConfig holds logging configuration
type LogConfig struct {
	Mode      LogMode
	Name      string // For daemon: vibespace name. For CLI/TUI: ignored
	RequestID string // Correlation ID for this operation
}

// setupLogging configures slog based on the mode
// Returns a cleanup function that should be deferred
func setupLogging(cfg LogConfig) func() {
	// Generate request ID if not provided
	if cfg.RequestID == "" {
		cfg.RequestID = uuid.NewString()[:8]
	}

	// Check if debug mode is enabled (for CLI/TUI)
	debugEnabled := os.Getenv("VIBESPACE_DEBUG") != ""

	// Determine log level
	level := slog.LevelInfo
	if lvl := os.Getenv("VIBESPACE_LOG_LEVEL"); lvl != "" {
		switch strings.ToLower(lvl) {
		case "debug":
			level = slog.LevelDebug
		case "warn", "warning":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	// For CLI/TUI in debug mode, always use debug level
	if cfg.Mode != LogModeDaemon && debugEnabled {
		level = slog.LevelDebug
	}

	var writer io.Writer
	var cleanup func()

	switch cfg.Mode {
	case LogModeDaemon:
		// Daemon always logs to file with JSON format
		w := newRotatingWriter(cfg.Name + ".log")
		writer = w
		cleanup = func() { w.Close() }

		handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: level,
		})
		// Add default attributes for daemon
		logger := slog.New(handler).With(
			"vibespace", cfg.Name,
			"mode", "daemon",
		)
		slog.SetDefault(logger)
		return cleanup

	case LogModeCLI:
		if !debugEnabled {
			// Discard logs when not debugging
			slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
			return func() {}
		}
		w := newRotatingWriter("debug.log")
		writer = w
		cleanup = func() { w.Close() }

	case LogModeTUI:
		if !debugEnabled {
			// Discard logs when not debugging
			slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
			return func() {}
		}
		w := newRotatingWriter("tui-debug.log")
		writer = w
		cleanup = func() { w.Close() }
	}

	// CLI/TUI use text format for readability
	handler := slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: level,
	})
	// Add request ID to all log entries
	logger := slog.New(handler).With(
		"request_id", cfg.RequestID,
		"mode", string(cfg.Mode),
	)
	slog.SetDefault(logger)

	return cleanup
}

// newRotatingWriter creates a lumberjack rotating writer
func newRotatingWriter(filename string) *lumberjack.Logger {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".vibespace")
	os.MkdirAll(logDir, 0755)

	// For daemon logs, use daemons subdirectory
	if strings.HasSuffix(filename, ".log") && filename != "debug.log" && filename != "tui-debug.log" {
		logDir = filepath.Join(logDir, "daemons")
		os.MkdirAll(logDir, 0755)
	}

	return &lumberjack.Logger{
		Filename:   filepath.Join(logDir, filename),
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     7, // days
		Compress:   true,
	}
}
