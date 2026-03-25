package common

import (
	"context"
	"log/slog"
	"testing"

	"github.com/spf13/cobra"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func TestLogLevelFromFlags(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("log-level", "", "")

	got, ok, err := LogLevelFromFlags(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected flag to be unset")
	}
	if got != "" {
		t.Fatalf("expected empty value, got %q", got)
	}

	if err := cmd.Flags().Set("log-level", "debug"); err != nil {
		t.Fatalf("set flag: %v", err)
	}
	got, ok, err = LogLevelFromFlags(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected flag to be set")
	}
	if got != "debug" {
		t.Fatalf("expected debug, got %q", got)
	}
}

func TestApplyProcessLogLevel(t *testing.T) {
	t.Setenv(logging.EnvLogLevel, "info")
	logging.SetLevel("info")

	if err := ApplyProcessLogLevel("debug"); err != nil {
		t.Fatalf("ApplyProcessLogLevel: %v", err)
	}
	if got := logging.Logger().Enabled(context.Background(), slog.LevelDebug); !got {
		t.Fatalf("expected debug to be enabled")
	}
	if got := logging.Logger().Enabled(context.Background(), slog.LevelInfo); !got {
		t.Fatalf("expected info to be enabled")
	}
	if env := logging.EnvLogLevel; env == "" {
		t.Fatalf("expected env name")
	}
}
