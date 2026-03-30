package health

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestWaitForSocksProxyReady(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := WaitForSocksProxy(ctx, ln.Addr().String(), 200*time.Millisecond, 10*time.Millisecond); err != nil {
		t.Fatalf("WaitForSocksProxy ready: %v", err)
	}
}

func TestWaitForSocksProxyCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := WaitForSocksProxy(ctx, "127.0.0.1:1", 200*time.Millisecond, 10*time.Millisecond); err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestWaitForSocksProxyTimeout(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := WaitForSocksProxy(ctx, addr, 100*time.Millisecond, 10*time.Millisecond); err == nil {
		t.Fatal("expected timeout error")
	}
}
