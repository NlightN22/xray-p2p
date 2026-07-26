package nethttp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestNewServerAppliesContainmentDefaults(t *testing.T) {
	server := NewServer(http.NotFoundHandler(), ServerOptions{})
	if server.ReadHeaderTimeout != DefaultServerReadHeaderTimeout ||
		server.ReadTimeout != DefaultServerReadTimeout ||
		server.WriteTimeout != DefaultServerWriteTimeout ||
		server.IdleTimeout != DefaultServerIdleTimeout {
		t.Fatalf("unexpected server timeouts: %#v", server.Server)
	}
}

func TestServerReusesKeepAliveAndClosesIdleConnection(t *testing.T) {
	server := NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}), ServerOptions{IdleTimeout: 80 * time.Millisecond})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()

	transport := &http.Transport{}
	client := &http.Client{Transport: transport, Timeout: time.Second}
	for range 2 {
		response, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}
	if got := server.Metrics().New; got != 1 {
		t.Fatalf("keep-alive opened %d connections, want 1", got)
	}
	legacyRequest, err := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	legacyRequest.Close = true
	legacyResponse, err := client.Do(legacyRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, legacyResponse.Body)
	_ = legacyResponse.Body.Close()
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if got := server.Metrics().New; got != 2 {
		t.Fatalf("mixed keep-alive/legacy clients opened %d connections, want 2", got)
	}
	deadline := time.Now().Add(time.Second)
	for server.Metrics().Current != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := server.Metrics().Current; got != 0 {
		t.Fatalf("idle connection remained open: current=%d", got)
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

func TestServerWriteTimeoutReleasesSlowResponseConsumer(t *testing.T) {
	handlerDone := make(chan struct{})
	server := NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		defer close(handlerDone)
		chunk := make([]byte, 64*1024)
		for range 1024 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}), ServerOptions{WriteTimeout: 50 * time.Millisecond})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: test\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handlerDone:
	case <-time.After(2 * time.Second):
		t.Fatal("slow response consumer retained the handler past WriteTimeout")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.ShutdownOwned(stopCtx, listener); err != nil {
		t.Fatal(err)
	}
}

func TestServerReadHeaderTimeoutClosesIncompleteHeader(t *testing.T) {
	server := NewServer(http.NotFoundHandler(), ServerOptions{ReadHeaderTimeout: 50 * time.Millisecond})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve(listener)
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost:"); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("incomplete header connection remained readable")
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.ShutdownOwned(stopCtx, listener); err != nil {
		t.Fatal(err)
	}
}

func TestShutdownOwnedReturnsListenerCloseError(t *testing.T) {
	server := NewServer(http.NotFoundHandler(), ServerOptions{})
	want := errors.New("listener close failed")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := server.ShutdownOwned(ctx, errorListener{closeErr: want})
	if !errors.Is(err, want) {
		t.Fatalf("ShutdownOwned error = %v, want %v", err, want)
	}
}

type errorListener struct{ closeErr error }

func (errorListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (l errorListener) Close() error            { return l.closeErr }
func (errorListener) Addr() net.Addr            { return testAddr("listener") }

type testAddr string

func (a testAddr) Network() string { return string(a) }
func (a testAddr) String() string  { return string(a) }

func TestServerMetricsTrackConnectionStatesWithoutRemoteLabels(t *testing.T) {
	server := NewServer(http.NotFoundHandler(), ServerOptions{})
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	server.observeConnState(left, http.StateNew)
	server.observeConnState(left, http.StateActive)
	server.observeConnState(left, http.StateIdle)
	metrics := server.Metrics()
	if metrics.New != 1 || metrics.Active != 0 || metrics.Idle != 1 || metrics.Current != 1 || metrics.Peak != 1 {
		t.Fatalf("unexpected live metrics: %#v", metrics)
	}
	server.observeConnState(left, http.StateClosed)
	metrics = server.Metrics()
	if metrics.Closed != 1 || metrics.Current != 0 || metrics.Idle != 0 {
		t.Fatalf("unexpected closed metrics: %#v", metrics)
	}
}
