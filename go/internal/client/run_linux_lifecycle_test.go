//go:build linux

package client

import (
	"context"
	"testing"
	"time"
)

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
