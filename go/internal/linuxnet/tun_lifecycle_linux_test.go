//go:build linux

package linuxnet

import (
	"context"
	"errors"
	"testing"
)

func TestEnsureTunAddressReturnsOnCancellationBeforeSetup(t *testing.T) {
	previous := execLookPathFunc
	t.Cleanup(func() { execLookPathFunc = previous })
	execLookPathFunc = func(string) (string, error) { return "/usr/sbin/ip", nil }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := EnsureTunAddressContext(ctx, "xp2p-test", "198.18.0.1/30", 1500)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("EnsureTunAddressContext error = %v, want context canceled", err)
	}
}
