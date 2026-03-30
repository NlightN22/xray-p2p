package health

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const defaultProbeInterval = 500 * time.Millisecond

// WaitForSocksProxy waits until a TCP listener is available at addr or timeout elapses.
func WaitForSocksProxy(ctx context.Context, addr string, timeout, interval time.Duration) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("socks proxy address is empty")
	}
	if interval <= 0 {
		interval = defaultProbeInterval
	}

	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	var lastErr error
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		conn, err := net.DialTimeout("tcp", addr, interval)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		lastErr = err
		if timeout > 0 && time.Now().After(deadline) {
			return fmt.Errorf("socks proxy %s not ready: %w", addr, lastErr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
