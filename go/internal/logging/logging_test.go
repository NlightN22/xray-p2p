package logging

import (
	"bytes"
	"log/slog"
	"os"
	"strings"
	"testing"
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
	if !strings.Contains(out, "service=xp2p") {
		t.Fatalf("expected service attribute in log output, got %q", out)
	}
	if !strings.Contains(out, "msg=started") {
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
	if !strings.Contains(out, "trace_id=abc123") {
		t.Fatalf("expected trace attribute, got %q", out)
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
