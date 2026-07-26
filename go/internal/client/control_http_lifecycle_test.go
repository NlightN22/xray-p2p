package client

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
)

func TestControlHTTPPoolPlateausAcrossRotationAndSubscriptionCycles(t *testing.T) {
	var opened atomic.Int64
	var closed atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case controlplane.PathCredentialsRotate, controlplane.PathCredentialsAck:
			var payload controlplane.RotationRequest
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if payload.Action == "challenge" {
				_, _ = io.WriteString(w, `{"nonce":"stable-nonce"}`)
				return
			}
			_, _ = io.WriteString(w, `{"rotation_pending":false}`)
		case controlplane.PathSubscription:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, request)
		}
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			opened.Add(1)
		case http.StateClosed:
			closed.Add(1)
		}
	}
	server.StartTLS()
	defer server.Close()

	endpoint, port := lifecycleEndpoint(t, server.URL)
	pool := newControlHTTPPool(time.Second, "")
	client := pool.client(endpoint)
	for range 50 {
		if _, err := fetchRotation(t.Context(), client, endpoint, port, "credential"); err != nil {
			t.Fatal(err)
		}
		if _, err := fetchSubscriptionConditional(t.Context(), client, endpoint, port, "credential", "stable"); err != nil {
			t.Fatal(err)
		}
		if err := acknowledgeRotation(t.Context(), client, endpoint, port, "credential"); err != nil {
			t.Fatal(err)
		}
	}
	if got := opened.Load(); got > 2 {
		t.Fatalf("opened connections = %d after 50 cycles, want at most 2", got)
	}

	shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := pool.shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	waitForConnectionCount(t, time.Second, func() bool { return closed.Load() == opened.Load() })
}

func TestRequestScopedTransportsReproduceConnectionGrowth(t *testing.T) {
	var opened atomic.Int64
	var closed atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			opened.Add(1)
		case http.StateClosed:
			closed.Add(1)
		}
	}
	server.StartTLS()
	defer server.Close()

	base := server.Client().Transport.(*http.Transport)
	transports := make([]*http.Transport, 0, 25)
	for range 25 {
		transport := base.Clone()
		transports = append(transports, transport)
		client := &http.Client{Transport: transport, Timeout: time.Second}
		response, err := client.Get(server.URL)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			t.Fatal(err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if got := opened.Load(); got != int64(len(transports)) {
		t.Fatalf("request-scoped transports opened %d connections, want %d", got, len(transports))
	}
	for _, transport := range transports {
		transport.CloseIdleConnections()
	}
	waitForConnectionCount(t, time.Second, func() bool { return closed.Load() == opened.Load() })
}

func TestControlHTTPPoolPruneClosesEndpointConnections(t *testing.T) {
	var opened atomic.Int64
	var closed atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			opened.Add(1)
		case http.StateClosed:
			closed.Add(1)
		}
	}
	server.StartTLS()
	defer server.Close()

	endpoint, _ := lifecycleEndpoint(t, server.URL)
	pool := newControlHTTPPool(time.Second, "")
	client := pool.client(endpoint)
	response, err := client.Do(mustRequest(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatal(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if opened.Load() == 0 {
		t.Fatal("test did not establish a TCP connection")
	}
	pruneCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := pool.prune(pruneCtx, nil); err != nil {
		t.Fatal(err)
	}
	if len(pool.clients) != 0 {
		t.Fatalf("client count = %d, want 0 after prune", len(pool.clients))
	}
	waitForConnectionCount(t, time.Second, func() bool { return closed.Load() == opened.Load() })
}

func TestSubscriptionRunnerShutdownClosesActiveResponse(t *testing.T) {
	handlerDone := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "partial")
		w.(http.Flusher).Flush()
		<-request.Context().Done()
		close(handlerDone)
	}))
	defer server.Close()

	endpoint, _ := lifecycleEndpoint(t, server.URL)
	pool := newControlHTTPPool(time.Second, "")
	response, err := pool.client(endpoint).Do(mustRequest(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	runner := subscriptionSyncRunner{timeout: time.Second, clients: pool}
	runner.shutdown()
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("runner shutdown did not cancel the active response")
	}
	if _, err := response.Body.Read(make([]byte, 1)); err == nil {
		t.Fatal("runner shutdown left the active response body open")
	}
}

func lifecycleEndpoint(t *testing.T, rawURL string) (clientEndpointRecord, int) {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	host, rawPort, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatal(err)
	}
	return clientEndpointRecord{
		Hostname:      host,
		User:          "alice",
		AllowInsecure: true,
	}, port
}

func waitForConnectionCount(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("connections did not close before the deadline")
}

func mustRequest(t *testing.T, rawURL string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
