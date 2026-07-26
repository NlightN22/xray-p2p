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

func TestControlHTTPPoolPrunesChangedTLSPolicy(t *testing.T) {
	pool := newControlHTTPPool(time.Second, "")
	endpoint := clientEndpointRecord{Hostname: "edge.example", ServerName: "edge.example"}
	first := pool.client(endpoint)
	endpoint.AllowInsecure = true
	second := pool.client(endpoint)
	if first == second {
		t.Fatal("TLS policy change reused the previous client")
	}
	if len(pool.clients) != 2 {
		t.Fatalf("client count = %d, want 2 before prune", len(pool.clients))
	}
	if err := pool.prune(t.Context(), []clientEndpointRecord{endpoint}); err != nil {
		t.Fatal(err)
	}
	if len(pool.clients) != 1 {
		t.Fatalf("client count = %d, want 1 after prune", len(pool.clients))
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
