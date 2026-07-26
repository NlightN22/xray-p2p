package nethttp

import (
	"net"
	"net/http"
	"testing"
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
