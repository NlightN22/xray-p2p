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
	for attempt := 0; attempt < maxAttempts; attempt++ {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			tb.Fatalf("failed to allocate listener: %v", err)
		}

		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		pc, err := net.ListenPacket("udp4", addr)
		if err != nil {
			if shouldRetryPort(err) {
				continue
			}
			tb.Fatalf("failed to allocate udp listener on %s: %v", addr, err)
		}
		_ = pc.Close()

		return strconv.Itoa(port), port
	}

	tb.Fatalf("failed to find free TCP/UDP port after %d attempts", maxAttempts)
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
