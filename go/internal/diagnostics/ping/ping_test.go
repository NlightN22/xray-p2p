package ping

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/testutil"
)

func TestRunHandlesTCPReplies(t *testing.T) {
	setupLogging(t)

	cancel, port := startBackgroundServer(t)
	defer cancel()

	runCtx, runCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer runCancel()

	if err := Run(runCtx, "127.0.0.1", Options{
		Count:   1,
		Timeout: time.Second,
		Proto:   "tcp",
		Port:    port,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunHandlesUDPReplies(t *testing.T) {
	setupLogging(t)

	cancel, port := startBackgroundServer(t)
	defer cancel()

	runCtx, runCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer runCancel()

	if err := Run(runCtx, "127.0.0.1", Options{
		Count:   1,
		Timeout: time.Second,
		Proto:   "udp",
		Port:    port,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunFailsWhenServerUnavailable(t *testing.T) {
	setupLogging(t)

	_, port := testutil.FreePort(t)

	runCtx, runCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer runCancel()

	start := time.Now()
	err := Run(runCtx, "127.0.0.1", Options{
		Count:   1,
		Timeout: 100 * time.Millisecond,
		Proto:   "tcp",
		Port:    port,
	})

	if err == nil {
		t.Fatalf("expected error when server unavailable")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("Run took too long: %s", time.Since(start))
	}
}

func TestReporterInvokedOnSuccess(t *testing.T) {
	setupLogging(t)
	cancel, port := startBackgroundServer(t)
	defer cancel()

	var called atomic.Bool
	reporter := reporterFunc(func(ctx context.Context, conn net.Conn, result Result) error {
		called.Store(true)
		if conn == nil {
			t.Fatalf("expected live connection")
		}
		return nil
	})

	runCtx, runCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer runCancel()

	if err := Run(runCtx, "127.0.0.1", Options{
		Count:    1,
		Timeout:  time.Second,
		Proto:    "tcp",
		Port:     port,
		Reporter: reporter,
		Silent:   true,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called.Load() {
		t.Fatalf("reporter was not invoked")
	}
}

func TestRunValidatesTargetAndProtocol(t *testing.T) {
	setupLogging(t)
	if err := Run(context.Background(), "", Options{}); err == nil {
		t.Fatalf("expected error for missing target")
	}
	if err := Run(context.Background(), "127.0.0.1", Options{Proto: "icmp"}); err == nil {
		t.Fatalf("expected error for unsupported protocol")
	}
}

func TestReporterErrorPropagates(t *testing.T) {
	setupLogging(t)
	cancel, port := startBackgroundServer(t)
	defer cancel()
	reporter := reporterFunc(func(context.Context, net.Conn, Result) error {
		return errors.New("report failure")
	})
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	_, err := pingTCP(context.Background(), addr, time.Second, "", 1, reporter)
	if err == nil || !strings.Contains(err.Error(), "report failure") {
		t.Fatalf("expected reporter error, got %v", err)
	}
}

func TestDialViaSocksInvalidProxy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := dialViaSocks(ctx, "127.0.0.1:9", "127.0.0.1:1", 100*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error when SOCKS proxy is unreachable")
	}
}

func TestDialViaSocksRespectsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := dialViaSocks(ctx, "127.0.0.1:80", "127.0.0.1:1080", time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

type reporterFunc func(context.Context, net.Conn, Result) error

func (fn reporterFunc) Report(ctx context.Context, conn net.Conn, result Result) error {
	return fn(ctx, conn, result)
}

func setupLogging(t *testing.T) {
	t.Helper()
	logging.Configure(logging.Options{Output: io.Discard})
	t.Cleanup(func() {
		logging.Configure(logging.Options{Output: os.Stderr})
	})
}

func startBackgroundServer(t *testing.T) (context.CancelFunc, int) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	portStr, port := testutil.FreePort(t)
	if err := server.StartBackground(ctx, server.Options{Port: portStr}); err != nil {
		cancel()
		t.Fatalf("failed to start background server: %v", err)
	}

	addr := "127.0.0.1:" + portStr
	testutil.WaitForCondition(t, time.Second, func() bool {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	})

	return cancel, port
}
