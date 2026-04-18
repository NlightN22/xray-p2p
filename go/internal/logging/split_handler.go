package logging

import (
	"context"
	"log/slog"
)

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
