package root

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/testutil"
)

func TestSplitListenAddress(t *testing.T) {
	tests := []struct {
		value string
		host  string
		port  string
	}{
		{value: "62022", host: "0.0.0.0", port: "62022"},
		{value: "127.0.0.1:62022", host: "127.0.0.1", port: "62022"},
	}

	for _, tt := range tests {
		host, port, err := splitListenAddress(tt.value)
		if err != nil {
			t.Fatalf("splitListenAddress(%q) returned error: %v", tt.value, err)
		}
		if host != tt.host || port != tt.port {
			t.Fatalf("splitListenAddress(%q) = %s:%s, want %s:%s", tt.value, host, port, tt.host, tt.port)
		}
	}
}

func TestRunDiagCommandHTTPS(t *testing.T) {
	setupDiagTest(t)
	portStr, _ := testutil.FreePort(t)
	addr := net.JoinHostPort("127.0.0.1", portStr)
	certPath, keyPath := diagTLSFiles(t)
	writeDiagRuntime(t)
	cfg := config.Config{
		Server: config.ServerConfig{
			Port:            portStr,
			CertificateFile: certPath,
			KeyFile:         keyPath,
		},
	}
	opts := diagCommandOptions{Listen: addr}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan int, 1)
	go func() {
		resultCh <- runDiagCommand(ctx, cfg, opts)
	}()

	baseURL := "https://" + addr
	client := diagHTTPClient()
	testutil.WaitForCondition(t, time.Second, func() bool {
		resp, err := client.Get(baseURL + controlplane.PathReady)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})

	body := []byte(`{"nonce":"testnonce"}`)
	resp, err := client.Post(baseURL+controlplane.PathPing, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST ping: %v", err)
	}
	defer resp.Body.Close()
	var pong controlplane.PingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pong); err != nil {
		t.Fatalf("decode pong: %v", err)
	}
	if pong.Nonce != "testnonce" {
		t.Fatalf("unexpected nonce: %q", pong.Nonce)
	}

	cancel()
	select {
	case code := <-resultCh:
		if code != 0 {
			t.Fatalf("runDiagCommand returned %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("runDiagCommand did not exit after context cancel")
	}
}

func setupDiagTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	t.Setenv("XP2P_LOG_ROOT", filepath.Join(dir, "logs"))
	logging.Configure(logging.Options{Output: io.Discard})
	t.Cleanup(func() {
		logging.Configure(logging.Options{Output: os.Stderr})
	})
}

func writeDiagRuntime(t *testing.T) {
	t.Helper()
	path := config.LiveConfigPath(filepath.Join(layout.ServerConfigDir, layout.RuntimeMetaFileName))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir live runtime dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"control":{"subscription":{"generation":"test"}}}`), 0o644); err != nil {
		t.Fatalf("write runtime metadata: %v", err)
	}
}

func diagHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   time.Second,
	}
}

func diagTLSFiles(t *testing.T) (string, string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	return filepath.Join(root, "tests", "fixtures", "tls", "integration-cert.pem"),
		filepath.Join(root, "tests", "fixtures", "tls", "integration-key.pem")
}
