//go:build linux

package client

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClientInitialOpenWrtTunSetupUsesRunContext(t *testing.T) {
	original := ensureClientOpenWrtTunInterfaceContextFunc
	t.Cleanup(func() { ensureClientOpenWrtTunInterfaceContextFunc = original })

	started := make(chan struct{})
	ensureClientOpenWrtTunInterfaceContextFunc = func(ctx context.Context, _, _ string) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ensureClientTunSetup(ctx, RunOptions{
			TunName: "xp2pc0",
			TunAddr: "10.0.0.2/30",
			TunMTU:  1500,
		})
	}()
	<-started
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("OpenWrt TUN setup error = %v, want context cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client OpenWrt TUN setup did not observe run cancellation")
	}
}

func TestTunRouteRefreshStopCancelsAndJoinsActiveRefresh(t *testing.T) {
	previous := refreshClientLiveTunRoutesFunc
	t.Cleanup(func() { refreshClientLiveTunRoutesFunc = previous })
	started := make(chan struct{})
	exited := make(chan struct{})
	refreshClientLiveTunRoutesFunc = func(ctx context.Context, _ string, _ string, _ string, _ int) {
		close(started)
		<-ctx.Done()
		close(exited)
	}
	stop := startTunRouteRefreshLoop(context.Background(), "", RunOptions{TunEnabled: true})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("route refresh did not start")
	}
	stop()
	select {
	case <-exited:
	default:
		t.Fatal("route refresh stop returned before worker exit")
	}
}
