package server

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	xnethttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

func TestResourceGrowthDetectorWarnsAfterThreeIncreasingSamples(t *testing.T) {
	var detector resourceGrowthDetector
	if connection, _ := detector.Observe(1, 10); connection {
		t.Fatal("growth warning fired before three samples")
	}
	if connection, _ := detector.Observe(2, 11); connection {
		t.Fatal("growth warning fired before three samples")
	}
	if connection, _ := detector.Observe(3, 12); !connection {
		t.Fatal("growth warning did not fire after three samples")
	}
	if connection, _ := detector.Observe(3, 12); connection {
		t.Fatal("stable resources did not reset growth warning")
	}
}

func TestBackgroundOwnerClosesListenerAfterServeTLSError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	owner := startOwnedHTTPServer(
		context.Background(),
		listener,
		xnethttp.NewServer(http.NotFoundHandler(), xnethttp.ServerOptions{}),
		"missing-cert",
		"missing-key",
		"test server",
		time.Hour,
	)
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := owner.Wait(waitCtx); err == nil {
		t.Fatal("expected ServeTLS certificate error")
	}
	if conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond); err == nil {
		_ = conn.Close()
		t.Fatal("listener remained open after ServeTLS error")
	}
}

func TestResourceGrowthDetectorTracksFDWithoutConnections(t *testing.T) {
	var detector resourceGrowthDetector
	detector.Observe(0, 10)
	detector.Observe(0, 11)
	connection, process := detector.Observe(0, 12)
	if !process {
		t.Fatal("fd-only growth was not detected")
	}
	if connection {
		t.Fatal("process-wide fd growth was attributed to HTTP connections")
	}
}
