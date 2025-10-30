package testutil

import (
	"net"
	"strconv"
	"testing"
	"time"
)

// FreePort returns an unused TCP port on localhost.
func FreePort(tb testing.TB) (string, int) {
	tb.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		tb.Fatalf("failed to allocate listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	return strconv.Itoa(port), port
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
