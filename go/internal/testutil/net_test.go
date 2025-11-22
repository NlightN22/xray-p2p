package testutil

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestFreePortReturnsUsablePorts(t *testing.T) {
	portStr, port := FreePort(t)
	if port <= 0 || portStr == "" {
		t.Fatalf("FreePort returned invalid values: %s %d", portStr, port)
	}

	addr := net.JoinHostPort("127.0.0.1", portStr)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen tcp on free port: %v", err)
	}
	ln.Close()
}

func TestWaitForCondition(t *testing.T) {
	counter := 0
	WaitForCondition(t, 200*time.Millisecond, func() bool {
		counter++
		return counter > 2
	})
	if counter <= 2 {
		t.Fatalf("WaitForCondition did not evaluate function")
	}
}

func TestShouldRetryPort(t *testing.T) {
	if !shouldRetryPort(errors.New("bind: forbidden")) {
		t.Fatalf("expected forbidden error to trigger retry")
	}
	opErr := &net.OpError{Err: errors.New("bind: forbidden")}
	if !shouldRetryPort(opErr) {
		t.Fatalf("net.OpError should trigger retry")
	}
	if shouldRetryPort(errors.New("other error")) {
		t.Fatalf("unexpected retry for unrelated error")
	}
}
