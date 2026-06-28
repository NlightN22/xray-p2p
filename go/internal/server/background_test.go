package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/testutil"
)

func TestStartBackgroundServesHTTPSControlPing(t *testing.T) {
	setupBackgroundTestLogging(t)
	cancel, baseURL := startTestControlServer(t, t.TempDir(), true)
	defer cancel()

	client := testControlHTTPClient()
	resp, err := client.Get(baseURL + controlplane.PathReady)
	if err != nil {
		t.Fatalf("GET ready: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ready status = %s", resp.Status)
	}

	body := []byte(`{"nonce":"testnonce"}`)
	resp, err = client.Post(baseURL+controlplane.PathPing, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST ping: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping status = %s", resp.Status)
	}
	var pong controlplane.PingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pong); err != nil {
		t.Fatalf("decode ping response: %v", err)
	}
	if pong.Nonce != "testnonce" {
		t.Fatalf("nonce = %q", pong.Nonce)
	}
}

func TestStartBackgroundServesHTTPSControlPingWithoutRuntimeMetadata(t *testing.T) {
	setupBackgroundTestLogging(t)
	cancel, baseURL := startTestControlServer(t, t.TempDir(), false)
	defer cancel()

	body := []byte(`{"nonce":"standalone"}`)
	resp, err := testControlHTTPClient().Post(baseURL+controlplane.PathPing, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST ping: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping status = %s", resp.Status)
	}
	var pong controlplane.PingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pong); err != nil {
		t.Fatalf("decode ping response: %v", err)
	}
	if pong.Nonce != "standalone" {
		t.Fatalf("nonce = %q", pong.Nonce)
	}
}

func TestHTTPSHeartbeatPayloadIsPersisted(t *testing.T) {
	setupBackgroundTestLogging(t)
	dir := t.TempDir()
	cancel, baseURL := startTestControlServer(t, dir, true)
	defer cancel()

	payload := heartbeat.Payload{
		Tag:       "proxy-test",
		Host:      "edge.example.com",
		ClientIP:  "10.0.0.5",
		Timestamp: time.Now().UTC(),
		RTTMillis: 12,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	resp, err := testControlHTTPClient().Post(baseURL+controlplane.PathHeartbeat, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST heartbeat: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("heartbeat status = %s", resp.Status)
	}

	stateRoot := dir
	if runtime.GOOS == "windows" {
		stateRoot = os.Getenv("XP2P_CONFIG_ROOT")
	}
	statePath := filepath.Join(stateRoot, layout.ServerHeartbeatStateFileName)
	state, err := heartbeat.Load(statePath)
	if err != nil {
		t.Fatalf("Load heartbeat state: %v", err)
	}
	snapshots := state.Snapshot(time.Now(), time.Second)
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if got := snapshots[0].Entry.Host; got != payload.Host {
		t.Fatalf("unexpected host %q", got)
	}
}

func startTestControlServer(t *testing.T, stateDir string, writeRuntime bool) (context.CancelFunc, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("XP2P_CONFIG_ROOT", stateDir)
		t.Setenv("XP2P_LOG_ROOT", filepath.Join(stateDir, "logs"))
	}
	certPath, keyPath := createTestCertificateFiles(t, stateDir, "127.0.0.1")
	liveDir := filepath.Join(stateDir, "live")
	if err := os.MkdirAll(liveDir, 0o755); err != nil {
		t.Fatalf("mkdir live: %v", err)
	}
	if writeRuntime {
		if err := os.WriteFile(filepath.Join(liveDir, layout.RuntimeMetaFileName), []byte(`{"control":{"subscription":{"generation":"test"}}}`), 0o644); err != nil {
			t.Fatalf("write runtime metadata: %v", err)
		}
	}

	portStr, _ := testutil.FreePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	if err := StartBackground(ctx, Options{Port: portStr, InstallDir: stateDir, CertPath: certPath, KeyPath: keyPath, LiveDir: liveDir}); err != nil {
		cancel()
		t.Fatalf("StartBackground returned error: %v", err)
	}
	baseURL := "https://127.0.0.1:" + portStr
	testutil.WaitForCondition(t, time.Second, func() bool {
		resp, err := testControlHTTPClient().Get(baseURL + controlplane.PathReady)
		if err != nil {
			return false
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	})
	return cancel, baseURL
}

func testControlHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		Timeout:   time.Second,
	}
}

func setupBackgroundTestLogging(t *testing.T) {
	t.Helper()
	logging.Configure(logging.Options{Output: io.Discard})
	t.Cleanup(func() {
		logging.Configure(logging.Options{Output: os.Stderr})
	})
}
