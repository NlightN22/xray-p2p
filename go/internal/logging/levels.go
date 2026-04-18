package logging

import (
	"fmt"
	"log/slog"
	"strings"
)

// SetLevel updates the logging level while keeping existing handler configuration.
func SetLevel(level string) {
	levelVar.Set(parseLevel(level))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
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

func NormalizeLevel(level string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "debug", "info", "warn", "error":
		return normalized, nil
	case "":
		return "", fmt.Errorf("log level is empty")
	default:
		return "", fmt.Errorf("invalid log level %q (use debug, info, warn, or error)", level)
	}
}
