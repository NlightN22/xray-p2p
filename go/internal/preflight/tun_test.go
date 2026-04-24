package preflight

import (
	"context"
	"strings"
	"testing"
)

func TestCheckTunDisabledIsNoop(t *testing.T) {
	if err := CheckTun(context.Background(), TunConfig{Enabled: false}); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestErrTunUnavailableFormatting(t *testing.T) {
	err := ErrTunUnavailable{
		OS:     "windows",
		Reason: "missing dependency",
		Hint:   "install it",
	}
	msg := err.Error()
	if !strings.Contains(msg, "tun is unavailable on windows") {
		t.Fatalf("unexpected error message: %q", msg)
	}
	if !strings.Contains(msg, "hint: install it") {
		t.Fatalf("unexpected error message: %q", msg)
	}
}
