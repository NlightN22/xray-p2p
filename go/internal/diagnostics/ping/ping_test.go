package ping

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	ownedhttp "github.com/NlightN22/xray-p2p/go/internal/nethttp"
	"github.com/NlightN22/xray-p2p/go/internal/server"
	"github.com/NlightN22/xray-p2p/go/internal/testutil"
)

func TestRunHandlesHTTPSReplies(t *testing.T) {
	setupLogging(t)

	cancel, port := startBackgroundServer(t)
	defer cancel()

	runCtx, runCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer runCancel()

	if err := Run(runCtx, "127.0.0.1", Options{
		Count:         1,
		Timeout:       time.Second,
		Port:          port,
		AllowInsecure: true,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunContinuousStopsOnContext(t *testing.T) {
	setupLogging(t)

	cancel, port := startBackgroundServer(t)
	defer cancel()

	runCtx, runCancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer runCancel()

	err := Run(runCtx, "127.0.0.1", Options{
		Timeout:       time.Second,
		Port:          port,
		AllowInsecure: true,
		Continuous:    true,
		Silent:        true,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline, got %v", err)
	}
}

func TestRunFailsWhenServerUnavailable(t *testing.T) {
	setupLogging(t)

	_, port := testutil.FreePort(t)

	runCtx, runCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer runCancel()

	start := time.Now()
	err := Run(runCtx, "127.0.0.1", Options{
		Count:         1,
		Timeout:       100 * time.Millisecond,
		Port:          port,
		AllowInsecure: true,
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
	reporter := reporterFunc(func(ctx context.Context, result Result) error {
		called.Store(true)
		return nil
	})

	runCtx, runCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer runCancel()

	if err := Run(runCtx, "127.0.0.1", Options{
		Count:         1,
		Timeout:       time.Second,
		Port:          port,
		Reporter:      reporter,
		AllowInsecure: true,
		Silent:        true,
	}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called.Load() {
		t.Fatalf("reporter was not invoked")
	}
}

func TestRunValidatesTarget(t *testing.T) {
	setupLogging(t)
	if err := Run(context.Background(), "", Options{}); err == nil {
		t.Fatalf("expected error for missing target")
	}
}

func TestReporterErrorPropagates(t *testing.T) {
	setupLogging(t)
	cancel, port := startBackgroundServer(t)
	defer cancel()
	reporter := reporterFunc(func(context.Context, Result) error {
		return errors.New("report failure")
	})
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	client := ownedhttp.NewClient(ownedhttp.ClientOptions{
		Timeout:   time.Second,
		TLSConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
	})
	defer shutdownHTTPClient(client)
	_, err := pingHTTPS(context.Background(), addr, time.Second, 1, Options{Reporter: reporter, HTTPClient: client})
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

type reporterFunc func(context.Context, Result) error

func (fn reporterFunc) Report(ctx context.Context, result Result) error {
	return fn(ctx, result)
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
	certPath, keyPath := testTLSFiles(t)
	if err := server.StartStandaloneDiagnostics(ctx, server.Options{Port: portStr, CertPath: certPath, KeyPath: keyPath}); err != nil {
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

func testTLSFiles(t *testing.T) (string, string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	return filepath.Join(root, "tests", "fixtures", "tls", "integration-cert.pem"),
		filepath.Join(root, "tests", "fixtures", "tls", "integration-key.pem")
}
