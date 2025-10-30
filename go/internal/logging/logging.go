// Package logging provides centralized, structured logging helpers for xp2p.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
)

const (
	defaultServiceName = "xp2p"
	envLogLevel        = "XP2P_LOG_LEVEL"
)

var (
	levelVar  slog.LevelVar
	activeLog atomic.Pointer[slog.Logger]
)

// init configures the default logger based on environment settings so the
// application has a usable logger without additional setup.
func init() {
	initLoggerFromEnv()
}

// Options controls logger configuration.
type Options struct {
	// Level is a textual representation of the desired log level (debug/info/warn/error).
	// When empty, the current level is preserved.
	Level string
	// Output selects the destination for log records. When nil, os.Stderr is used.
	Output io.Writer
}

// Configure allows the caller to adjust the global logger at runtime.
func Configure(opts Options) {
	if strings.TrimSpace(opts.Level) != "" {
		levelVar.Set(parseLevel(opts.Level))
	}
	setLogger(opts.Output)
}

// SetLevel updates the logging level while keeping existing handler configuration.
func SetLevel(level string) {
	levelVar.Set(parseLevel(level))
}

// Logger returns the shared slog.Logger instance.
func Logger() *slog.Logger {
	if log := activeLog.Load(); log != nil {
		return log
	}
	setLogger(nil)
	return activeLog.Load()
}

// With returns a logger extended with additional structured attributes.
func With(args ...any) *slog.Logger {
	return Logger().With(args...)
}

// Debug writes a debug level message.
func Debug(msg string, args ...any) {
	Logger().Debug(msg, args...)
}

// Info writes an info level message.
func Info(msg string, args ...any) {
	Logger().Info(msg, args...)
}

// Warn writes a warning level message.
func Warn(msg string, args ...any) {
	Logger().Warn(msg, args...)
}

// Error writes an error level message.
func Error(msg string, args ...any) {
	Logger().Error(msg, args...)
}

func setLogger(output io.Writer) {
	if output == nil {
		output = os.Stderr
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: &levelVar,
	})
	logger := slog.New(handler).With("service", defaultServiceName)
	activeLog.Store(logger)
	slog.SetDefault(logger)
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

func initLoggerFromEnv() {
	levelVar.Set(parseLevel(os.Getenv(envLogLevel)))
	setLogger(os.Stderr)
}
