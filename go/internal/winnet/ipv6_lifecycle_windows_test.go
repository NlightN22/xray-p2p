//go:build windows

package winnet

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestDisableIPv6BindingRetryCancellationReturnsPromptly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var attempts atomic.Int32
	done := make(chan struct{})
	go func() {
		disableIPv6BindingWithRetry(ctx, "xp2p-test", func(context.Context, string) (ipv6DisableResult, error) {
			attempts.Add(1)
			cancel()
			return ipv6ResultMissing, nil
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("IPv6 retry worker did not join after cancellation")
	}
	if attempts.Load() != 1 {
		t.Fatalf("attempts = %d, want 1", attempts.Load())
	}
}
