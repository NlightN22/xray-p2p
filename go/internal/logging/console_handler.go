package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"
)

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
	attrTexts := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		if attr.Key == "service" && service == "" {
			service = attr.Value.String()
			continue
		}
		if text := formatAttr(attr); text != "" {
			attrTexts = append(attrTexts, text)
		}
	}

	message := record.Message

	var b strings.Builder
	b.WriteString(record.Time.UTC().Format(time.RFC3339))
	b.WriteString(" ")
	b.WriteString(strings.ToUpper(record.Level.String()))
	if message != "" {
		b.WriteString(" ")
		b.WriteString(message)
	}
	if len(attrTexts) > 0 {
		if message != "" {
			b.WriteString(". ")
		} else {
			b.WriteString(" ")
		}
		b.WriteString(strings.Join(attrTexts, ". "))
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
	if val == "" {
		return attr.Key
	}
	return attr.Key + ": " + val
}

func attrValueString(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
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
