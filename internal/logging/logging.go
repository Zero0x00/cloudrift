// Package logging configures the process-wide structured logger (log/slog). Configure once
// at startup; the rest of the codebase logs via the slog default (slog.Info/Warn/Error).
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Configure sets the default slog logger to emit structured text to stderr at the given
// level. Unrecognized/empty levels default to info. Safe to call once at process startup.
func Configure(level string) {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: ParseLevel(level),
	})))
}

// ParseLevel maps a level string (debug|info|warn|error) to an slog.Level, defaulting to info.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
