package cli

import (
	"context"
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

		handler := &redactingHandler{
			inner: slog.NewJSONHandler(writer, &slog.HandlerOptions{
				Level: level,
			}),
		}
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
	handler := &redactingHandler{
		inner: slog.NewTextHandler(writer, &slog.HandlerOptions{
			Level: level,
		}),
	}
	// Add request ID to all log entries
	logger := slog.New(handler).With(
		"request_id", cfg.RequestID,
		"mode", string(cfg.Mode),
	)
	slog.SetDefault(logger)

	return cleanup
}

// sensitiveKeys lists slog attribute key substrings whose values are redacted.
// Matching is case-insensitive on the lowercased key.
var sensitiveKeys = []string{
	"key",         // publicKey, privateKey, ServerPublicKey, …
	"token",       // access_token, refresh_token, invite token, …
	"secret",      // client_secret, …
	"password",    // any password field
	"credential",  // git-credentials, …
	"fingerprint", // cert fingerprint
	"nonce",       // invite token nonce
	"sha256",      // file checksums (not secret, but unnecessary in logs)
}

// redactValue returns a redacted version of s: first 4 chars + "…[REDACTED]".
// Very short values are fully redacted.
func redactValue(s string) string {
	if len(s) <= 4 {
		return "[REDACTED]"
	}
	return s[:4] + "…[REDACTED]"
}

// isSensitiveKey returns true if the lowercased key contains any sensitive substring.
func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(lower, s) {
			return true
		}
	}
	return false
}

// redactingHandler wraps an slog.Handler and redacts attributes whose keys
// match known sensitive patterns before forwarding to the inner handler.
// This prevents accidental exposure of keys, tokens, fingerprints, and
// other secrets in debug/daemon log files.
type redactingHandler struct {
	inner slog.Handler
}

func (h *redactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *redactingHandler) Handle(ctx context.Context, r slog.Record) error {
	redacted := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		redacted.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, redacted)
}

func (h *redactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return &redactingHandler{inner: h.inner.WithAttrs(out)}
}

func (h *redactingHandler) WithGroup(name string) slog.Handler {
	return &redactingHandler{inner: h.inner.WithGroup(name)}
}

// redactAttr redacts a single attribute if its key is sensitive.
func redactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		out := make([]slog.Attr, len(attrs))
		for i, ga := range attrs {
			out[i] = redactAttr(ga)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	}

	if isSensitiveKey(a.Key) {
		if s := a.Value.String(); s != "" {
			return slog.String(a.Key, redactValue(s))
		}
	}
	return a
}

// newRotatingWriter creates a lumberjack rotating writer
func newRotatingWriter(filename string) *lumberjack.Logger {
	home, _ := os.UserHomeDir()
	logDir := filepath.Join(home, ".vibespace")
	os.MkdirAll(logDir, 0700)

	// For daemon logs, use daemons subdirectory
	if strings.HasSuffix(filename, ".log") && filename != "debug.log" && filename != "tui-debug.log" {
		logDir = filepath.Join(logDir, "daemons")
		os.MkdirAll(logDir, 0700)
	}

	return &lumberjack.Logger{
		Filename:   filepath.Join(logDir, filename),
		MaxSize:    10, // MB
		MaxBackups: 3,
		MaxAge:     7, // days
		Compress:   true,
	}
}
