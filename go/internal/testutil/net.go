package testutil

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// FreePort returns a TCP port that is also available for UDP on localhost.
func FreePort(tb testing.TB) (string, int) {
	tb.Helper()

	const maxAttempts = 32
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		udpConn, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			lastErr = err
			if shouldRetryPort(err) {
				continue
			}
			tb.Fatalf("failed to allocate udp listener: %v", err)
		}

		port := udpConn.LocalAddr().(*net.UDPAddr).Port
		addr := fmt.Sprintf("127.0.0.1:%d", port)

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			lastErr = err
			_ = udpConn.Close()
			if shouldRetryPort(err) {
				continue
			}
			tb.Fatalf("failed to allocate tcp listener on %s: %v", addr, err)
		}

		_ = ln.Close()
		_ = udpConn.Close()

		return strconv.Itoa(port), port
	}

	tb.Skipf("test environment exhausted TCP/UDP ports: last error %v", lastErr)
	return "", 0
}

func shouldRetryPort(err error) bool {
	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EADDRINUSE) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.EACCES) || errors.Is(opErr.Err, syscall.EADDRINUSE) {
			return true
		}
		if msg := opErr.Err.Error(); strings.Contains(strings.ToLower(msg), "forbidden") {
			return true
		}
	}
	if msg := strings.ToLower(err.Error()); strings.Contains(msg, "forbidden") {
		return true
	}
	return false
}

// WaitForCondition polls fn until it returns true or the timeout expires.
func WaitForCondition(tb testing.TB, timeout time.Duration, fn func() bool) {
	tb.Helper()

	deadline := time.Now().Add(timeout)
	for {
		if fn() {
			return
		}
		if time.Now().After(deadline) {
			tb.Fatalf("condition not met within %s", timeout)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
