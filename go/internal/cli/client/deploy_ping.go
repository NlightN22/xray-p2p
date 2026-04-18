package clientcmd

import (
	"context"
	"fmt"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
)

func waitForPing(ctx context.Context, host string, opts ping.Options, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := ping.Run(ctx, host, opts); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return fmt.Errorf("ping timeout after %s", timeout)
		}
		time.Sleep(1 * time.Second)
	}
}
