package servercmd

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDeployServerShutdownClosesAndJoinsStalledHandler(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	server := &deployServer{ListenAddr: address, Timeout: time.Hour}
	go func() { done <- server.Run(ctx) }()

	var conn net.Conn
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err = net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if conn == nil {
		t.Fatalf("connect to deploy server: %v", err)
	}
	defer conn.Close()

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context cancellation", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("deploy server retained a stalled handler during shutdown")
	}
}
