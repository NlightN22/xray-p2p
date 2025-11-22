// Package logging provides centralized, structured logging helpers for xp2p.
package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultServiceName = "xp2p"
	envLogLevel        = "XP2P_LOG_LEVEL"
)

var (
	levelVar  slog.LevelVar
	activeLog atomic.Pointer[slog.Logger]
	formatVar atomic.Value
)

// init configures the default logger based on environment settings so the
// application has a usable logger without additional setup.
func init() {
	formatVar.Store(FormatText)
	initLoggerFromEnv()
}

// Options controls logger configuration.
type Options struct {
	// Level is a textual representation of the desired log level (debug/info/warn/error).
	// When empty, the current level is preserved.
	Level string
	// Output selects the destination for log records. When nil, os.Stderr is used.
	Output io.Writer
	// Format selects the output format ("text" or "json"). Empty value keeps current format.
	Format Format
}

// Format describes the logger output format.
type Format string

const (
	// FormatText outputs human-readable log lines.
	FormatText Format = "text"
	// FormatJSON outputs structured JSON records.
	FormatJSON Format = "json"
)

// Configure allows the caller to adjust the global logger at runtime.
func Configure(opts Options) {
	if strings.TrimSpace(opts.Level) != "" {
		levelVar.Set(parseLevel(opts.Level))
	}
	if opts.Format != "" {
		switch opts.Format {
		case FormatJSON:
			formatVar.Store(FormatJSON)
		default:
			formatVar.Store(FormatText)
		}
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
	formatVar.Store(FormatText)
	setLogger(os.Stderr)
}

func newConsoleHandler(w io.Writer, level *slog.LevelVar) slog.Handler {
	return &consoleHandler{
		w:     w,
		level: level,
	}
}

type consoleHandler struct {
	mu     sync.Mutex
	w      io.Writer
	level  *slog.LevelVar
	attrs  []slog.Attr
	groups []string
}

func (h *consoleHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	return lvl >= h.level.Level()
}

func (h *consoleHandler) Handle(_ context.Context, record slog.Record) error {
	if !h.Enabled(context.Background(), record.Level) {
		return nil
	}

	attrs := make([]slog.Attr, 0, len(h.attrs)+int(record.NumAttrs()))
	for _, attr := range h.attrs {
		attrs = appendResolvedAttr(attrs, attr, h.groups)
	}
	record.Attrs(func(a slog.Attr) bool {
		attrs = appendResolvedAttr(attrs, a, h.groups)
		return true
	})

	var service string
	filtered := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		if attr.Key == "service" && service == "" {
			service = attr.Value.String()
			continue
		}
		filtered = append(filtered, attr)
	}

	var b strings.Builder
	b.WriteString(record.Time.UTC().Format(time.RFC3339))
	b.WriteString(" ")
	b.WriteString(strings.ToUpper(record.Level.String()))
	if service != "" {
		b.WriteString(" ")
		b.WriteString(service)
	}
	if record.Message != "" {
		if service != "" {
			b.WriteString(": ")
		} else {
			b.WriteString(" ")
		}
		b.WriteString(record.Message)
	}
	for _, attr := range filtered {
		b.WriteString(" ")
		b.WriteString(formatAttr(attr))
	}
	b.WriteByte('\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write([]byte(b.String()))
	return err
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := h.clone()
	for _, attr := range attrs {
		clone.attrs = appendResolvedAttr(clone.attrs, attr, clone.groups)
	}
	return clone
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	clone := h.clone()
	clone.groups = append(clone.groups, name)
	return clone
}

func (h *consoleHandler) clone() *consoleHandler {
	newAttrs := make([]slog.Attr, len(h.attrs))
	copy(newAttrs, h.attrs)
	newGroups := make([]string, len(h.groups))
	copy(newGroups, h.groups)
	return &consoleHandler{
		w:      h.w,
		level:  h.level,
		attrs:  newAttrs,
		groups: newGroups,
	}
}

func appendResolvedAttr(dst []slog.Attr, attr slog.Attr, groups []string) []slog.Attr {
	attr.Value = attr.Value.Resolve()
	if attr.Value.Kind() == slog.KindGroup {
		sub := attr.Value.Group()
		subGroups := append([]string(nil), groups...)
		if attr.Key != "" {
			subGroups = append(subGroups, attr.Key)
		}
		for _, child := range sub {
			dst = appendResolvedAttr(dst, child, subGroups)
		}
		return dst
	}

	key := attr.Key
	if key == "" && len(groups) > 0 {
		key = strings.Join(groups, ".")
	} else if key != "" && len(groups) > 0 {
		key = strings.Join(append(append([]string(nil), groups...), key), ".")
	}
	attr.Key = key
	return append(dst, attr)
}

func formatAttr(attr slog.Attr) string {
	val := attrValueString(attr.Value)
	if attr.Key == "" {
		return val
	}
	return attr.Key + "=" + val
}

func attrValueString(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if strings.ContainsAny(s, " \t\n\"") {
			return strconv.Quote(s)
		}
		return s
	case slog.KindTime:
		return v.Time().UTC().Format(time.RFC3339)
	case slog.KindInt64, slog.KindUint64, slog.KindFloat64, slog.KindBool, slog.KindDuration:
		return v.String()
	case slog.KindAny:
		return fmt.Sprintf("%v", v.Any())
	default:
		return v.String()
	}
}

type splitHandler struct {
	lowCutoff slog.Level
	low       slog.Handler
	high      slog.Handler
}

func newSplitHandler(lowHandler, highHandler slog.Handler, cutoff slog.Level) slog.Handler {
	return &splitHandler{
		lowCutoff: cutoff,
		low:       lowHandler,
		high:      highHandler,
	}
}

func (h *splitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level >= h.lowCutoff {
		return h.high.Enabled(ctx, level)
	}
	return h.low.Enabled(ctx, level)
}

func (h *splitHandler) Handle(ctx context.Context, record slog.Record) error {
	if record.Level >= h.lowCutoff {
		return h.high.Handle(ctx, record)
	}
	return h.low.Handle(ctx, record)
}

func (h *splitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &splitHandler{
		lowCutoff: h.lowCutoff,
		low:       h.low.WithAttrs(attrs),
		high:      h.high.WithAttrs(attrs),
	}
}

func (h *splitHandler) WithGroup(name string) slog.Handler {
	return &splitHandler{
		lowCutoff: h.lowCutoff,
		low:       h.low.WithGroup(name),
		high:      h.high.WithGroup(name),
	}
}
