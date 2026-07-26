package nethttp

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestOwnedClientReusesConnectionAndClosesIt(t *testing.T) {
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
	server.Start()
	defer server.Close()

	client := NewClient(ClientOptions{Timeout: time.Second})
	for range 25 {
		response, err := client.Do(newRequest(t, server.URL))
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
	if got := opened.Load(); got != 1 {
		t.Fatalf("opened connections = %d, want 1", got)
	}
	shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := client.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	eventually(t, time.Second, func() bool { return closed.Load() == opened.Load() })
	response, err := client.Do(newRequest(t, server.URL))
	if response != nil {
		_ = response.Body.Close()
	}
	if !errors.Is(err, ErrClientClosed) {
		t.Fatalf("request after shutdown error = %v, want %v", err, ErrClientClosed)
	}
}

func TestOwnedClientShutdownCancelsActiveRequest(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
	}))
	defer server.Close()

	client := NewClient(ClientOptions{Timeout: 10 * time.Second})
	requestDone := make(chan error, 1)
	go func() {
		response, err := client.Do(newRequest(t, server.URL))
		if err == nil {
			_ = response.Body.Close()
		}
		requestDone <- err
	}()
	<-started

	shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := client.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-requestDone:
		if err == nil {
			t.Fatal("active request was not canceled")
		}
	case <-time.After(time.Second):
		t.Fatal("active request did not finish")
	}
}

func TestOwnedClientShutdownClosesReturnedResponseBody(t *testing.T) {
	started := make(chan struct{})
	finished := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		close(started)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "partial")
		w.(http.Flusher).Flush()
		<-request.Context().Done()
		close(finished)
	}))
	defer server.Close()

	client := NewClient(ClientOptions{Timeout: 10 * time.Second})
	response, err := client.Do(newRequest(t, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	<-started

	shutdownCtx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := client.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("handler did not observe owner cancellation")
	}
	concrete := client.(*ownedClient)
	concrete.mu.Lock()
	defer concrete.mu.Unlock()
	if concrete.active != 0 || len(concrete.bodies) != 0 {
		t.Fatalf("shutdown left active=%d bodies=%d", concrete.active, len(concrete.bodies))
	}
	if _, err := response.Body.Read(make([]byte, 1)); err == nil {
		t.Fatal("returned response body remained readable after shutdown")
	}
}

func newRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func eventually(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied")
}
