//go:build linux

package nethttp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
)

func TestServerMassIdleConnectionsReturnResourcesToBaseline(t *testing.T) {
	server := NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}), ServerOptions{IdleTimeout: 100 * time.Millisecond})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()

	collector := xrayguard.DefaultCollector()
	baseline, err := collector.Sample(context.Background(), os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	const clientCount = 64
	transports := make([]*http.Transport, 0, clientCount)
	for range clientCount {
		transport := &http.Transport{}
		transports = append(transports, transport)
		client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
		response, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	if peak := server.Metrics().Peak; peak < clientCount {
		t.Fatalf("server peak connections = %d, want at least %d", peak, clientCount)
	}

	deadline := time.Now().Add(3 * time.Second)
	for server.Metrics().Current != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	after, err := collector.Sample(context.Background(), os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if current := server.Metrics().Current; current != 0 {
		t.Fatalf("mass idle connections remained open: current=%d", current)
	}
	if after.FDCount > baseline.FDCount+4 {
		t.Fatalf("file descriptors did not return near baseline: before=%d after=%d", baseline.FDCount, after.FDCount)
	}
	if after.EstablishedTCPCount > baseline.EstablishedTCPCount+2 {
		t.Fatalf("established TCP did not return near baseline: before=%d after=%d", baseline.EstablishedTCPCount, after.EstablishedTCPCount)
	}

	for _, transport := range transports {
		transport.CloseIdleConnections()
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.ShutdownOwned(stopCtx, listener); err != nil {
		t.Fatal(err)
	}
	if err := <-serveDone; !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve error = %v", err)
	}
}
