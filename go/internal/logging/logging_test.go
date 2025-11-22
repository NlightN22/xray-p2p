package logging

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoggerIncludesServiceAttribute(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{Level: "info", Output: &buf})
	t.Cleanup(func() {
		SetLevel("info")
		Configure(Options{Output: os.Stderr})
	})

	Logger().Info("started")
	out := buf.String()
	if !strings.Contains(out, "INFO xp2p: started") {
		t.Fatalf("expected message in log output, got %q", out)
	}
}

func TestSetLevelControlsEmission(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{Level: "info", Output: &buf})
	t.Cleanup(func() {
		SetLevel("info")
		Configure(Options{Output: os.Stderr})
	})

	Debug("hidden debug message")
	if buf.Len() != 0 {
		t.Fatalf("expected no debug output at info level, got %q", buf.String())
	}

	SetLevel("debug")
	Debug("visible debug message")
	if !strings.Contains(buf.String(), "visible debug message") {
		t.Fatalf("expected debug message after level change, got %q", buf.String())
	}
}

func TestWithAddsAttributes(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{Level: "info", Output: &buf})
	t.Cleanup(func() {
		SetLevel("info")
		Configure(Options{Output: os.Stderr})
	})

	log := With("trace_id", "abc123")
	log.Info("step")

	out := buf.String()
	if !strings.Contains(out, ". abc123") {
		t.Fatalf("expected attribute appended as value, got %q", out)
	}
}

func TestParseLevelDefaultsToInfo(t *testing.T) {
	if lvl := parseLevel("invalid"); lvl != slog.LevelInfo {
		t.Fatalf("expected info for invalid input, got %v", lvl)
	}
}

func TestConfigureRespectsEnvOnInit(t *testing.T) {
	oldLogger := Logger()
	oldLevel := levelVar.Level()

	t.Cleanup(func() {
		levelVar.Set(oldLevel)
		activeLog.Store(oldLogger)
		slog.SetDefault(oldLogger)
		Configure(Options{Output: os.Stderr})
	})

	t.Setenv(envLogLevel, "error")
	SetLevel("info")
	initLoggerFromEnv()

	if got := levelVar.Level(); got != slog.LevelError {
		t.Fatalf("expected level error from env var, got %v", got)
	}
}

func TestConfigureJSONFormatWritesStructuredOutput(t *testing.T) {
	var buf bytes.Buffer
	Configure(Options{Level: "debug", Output: &buf, Format: FormatJSON})
	t.Cleanup(func() {
		SetLevel("info")
		Configure(Options{Output: os.Stderr, Format: FormatText})
	})

	Logger().Info("json-line", "trace_id", "42")
	out := buf.String()
	if !strings.Contains(out, `"msg":"json-line"`) || !strings.Contains(out, `"trace_id":"42"`) {
		t.Fatalf("expected JSON output, got %q", out)
	}
}

func TestConsoleHandlerFormatsGroups(t *testing.T) {
	var buf bytes.Buffer
	var lvl slog.LevelVar
	lvl.Set(slog.LevelDebug)
	handler := newConsoleHandler(&buf, &lvl)

	record := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "hello", 0)
	record.AddAttrs(
		slog.String("user", "alpha beta"),
		slog.Group("meta", slog.String("id", "7"), slog.String("span", "root")),
	)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("handle failed: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, ". alpha beta") {
		t.Fatalf("expected attribute value, got %q", out)
	}
	if !strings.Contains(out, ". 7") || !strings.Contains(out, ". root") {
		t.Fatalf("expected group values appended, got %q", out)
	}
}

func TestConsoleHandlerTrimsServicePrefixAndFormatsErrors(t *testing.T) {
	var buf bytes.Buffer
	var lvl slog.LevelVar
	lvl.Set(slog.LevelError)
	handler := newConsoleHandler(&buf, &lvl)

	record := slog.NewRecord(time.Unix(0, 0), slog.LevelError, "xp2p server install: failed to resolve public host", 0)
	record.AddAttrs(
		slog.String("service", "xp2p"),
		slog.String("err", "netutil: unable to detect public host"),
	)

	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("handle failed: %v", err)
	}

	out := buf.String()
	if strings.Count(out, "xp2p") != 1 {
		t.Fatalf("expected single service prefix, got %q", out)
	}
	if !strings.Contains(out, "server install: failed to resolve public host. netutil: unable to detect public host") {
		t.Fatalf("expected attributes appended as sentences, got %q", out)
	}
	if strings.Contains(out, "err=") {
		t.Fatalf("err attribute should not include key, got %q", out)
	}
}

func TestSplitHandlerRoutesLevels(t *testing.T) {
	low := &recordingHandler{}
	high := &recordingHandler{}
	handler := newSplitHandler(low, high, slog.LevelWarn)

	info := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "info", 0)
	errRec := slog.NewRecord(time.Unix(0, 0), slog.LevelError, "error", 0)

	if err := handler.Handle(context.Background(), info); err != nil {
		t.Fatalf("handle info: %v", err)
	}
	if err := handler.Handle(context.Background(), errRec); err != nil {
		t.Fatalf("handle error: %v", err)
	}

	if !strings.Contains(low.buf.String(), "info") {
		t.Fatalf("info record should go to low handler, got %q", low.buf.String())
	}
	if !strings.Contains(high.buf.String(), "error") {
		t.Fatalf("error record should go to high handler, got %q", high.buf.String())
	}
}

type recordingHandler struct {
	buf strings.Builder
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *recordingHandler) Handle(_ context.Context, record slog.Record) error {
	h.buf.WriteString(record.Message)
	h.buf.WriteString("\n")
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *recordingHandler) WithGroup(string) slog.Handler {
	return h
}
