//go:build linux

package nethttp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/xrayguard"
)

func TestServerMassIdleConnectionsReturnResourcesToBaseline(t *testing.T) {
	const clientCount = 64
	connected := make(chan struct{}, clientCount)
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	server := NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		connected <- struct{}{}
		<-release
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
	transports := make([]*http.Transport, 0, clientCount)
	clients := make([]*http.Client, 0, clientCount)
	for range clientCount {
		transport := &http.Transport{}
		transports = append(transports, transport)
		clients = append(clients, &http.Client{Transport: transport, Timeout: 2 * time.Second})
	}
	requestErrors := make(chan error, clientCount)
	var requests sync.WaitGroup
	for _, client := range clients {
		requests.Add(1)
		go func() {
			defer requests.Done()
			response, err := client.Get("http://" + listener.Addr().String())
			if err != nil {
				requestErrors <- err
				return
			}
			_, _ = io.Copy(io.Discard, response.Body)
			requestErrors <- response.Body.Close()
		}()
	}
	connectDeadline := time.After(3 * time.Second)
	for range clientCount {
		select {
		case <-connected:
		case <-connectDeadline:
			t.Fatal("timed out waiting for mass connections")
		}
	}
	if peak := server.Metrics().Peak; peak < clientCount {
		t.Fatalf("server peak connections = %d, want at least %d", peak, clientCount)
	}
	releaseOnce.Do(func() { close(release) })
	requests.Wait()
	close(requestErrors)
	for err := range requestErrors {
		if err != nil {
			t.Fatal(err)
		}
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
