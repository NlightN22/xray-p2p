package logging

import (
	"io"
	"log/slog"
	"os"
)

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
	var handler slog.Handler
	format, _ := formatVar.Load().(Format)
	if format == "" {
		format = FormatText
	}

	switch format {
	case FormatJSON:
		w := output
		if w == nil {
			w = os.Stdout
		}
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{
			Level: &levelVar,
		})
	default:
		if output == nil {
			infoHandler := newConsoleHandler(os.Stdout, &levelVar)
			errorHandler := newConsoleHandler(os.Stderr, &levelVar)
			handler = newSplitHandler(infoHandler, errorHandler, slog.LevelWarn)
		} else {
			handler = newConsoleHandler(output, &levelVar)
		}
	}

	logger := slog.New(handler).With("service", defaultServiceName)
	activeLog.Store(logger)
	slog.SetDefault(logger)
}
