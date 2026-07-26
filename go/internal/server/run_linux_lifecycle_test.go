//go:build linux

package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServerInitialTunSetupUsesRunContext(t *testing.T) {
	original := ensureServerTunInterfaceContextFunc
	t.Cleanup(func() { ensureServerTunInterfaceContextFunc = original })

	started := make(chan struct{})
	exited := make(chan error, 1)
	ensureServerTunInterfaceContextFunc = func(ctx context.Context, _, _ string, _ int) error {
		close(started)
		<-ctx.Done()
		exited <- ctx.Err()
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		_ = ensureServerTunSetup(ctx, RunOptions{
			TunName: "xp2ps0",
			TunAddr: "10.0.0.1/30",
			TunMTU:  1500,
		})
	}()
	<-started
	cancel()

	select {
	case err := <-exited:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("TUN setup error = %v, want context cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server TUN setup did not observe run cancellation")
	}
}
