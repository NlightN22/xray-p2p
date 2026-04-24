//go:build windows

package preflight

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsTunPreflightMissingWintun(t *testing.T) {
	tempDir := t.TempDir()
	missing := filepath.Join(tempDir, "wintun.dll")

	err := windowsTunPreflight{}.Check(context.Background(), TunConfig{
		Enabled:       true,
		WintunDLLPath: missing,
	})
	var tunErr ErrTunUnavailable
	if !errors.As(err, &tunErr) {
		t.Fatalf("expected ErrTunUnavailable, got %T (%v)", err, err)
	}
	if !strings.Contains(strings.ToLower(tunErr.Reason), "not found") {
		t.Fatalf("unexpected reason: %q", tunErr.Reason)
	}
	if !strings.Contains(strings.ToLower(tunErr.Hint), "wintun.dll") {
		t.Fatalf("unexpected hint: %q", tunErr.Hint)
	}
}

func TestWindowsTunPreflightLoadFailure(t *testing.T) {
	oldLoad := wintunLoadCheckFunc
	t.Cleanup(func() { wintunLoadCheckFunc = oldLoad })

	wintunLoadCheckFunc = func(string) error {
		return errors.New("load failed")
	}

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "wintun.dll")
	if err := os.WriteFile(path, []byte("not-a-dll"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	err := windowsTunPreflight{}.Check(context.Background(), TunConfig{
		Enabled:       true,
		WintunDLLPath: path,
	})
	var tunErr ErrTunUnavailable
	if !errors.As(err, &tunErr) {
		t.Fatalf("expected ErrTunUnavailable, got %T (%v)", err, err)
	}
	if !strings.Contains(strings.ToLower(tunErr.Reason), "cannot be loaded") {
		t.Fatalf("unexpected reason: %q", tunErr.Reason)
	}
}
