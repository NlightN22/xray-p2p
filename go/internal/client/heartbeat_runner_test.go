package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestNewHeartbeatRunnerRequiresSocksTunnel(t *testing.T) {
	installDir := t.TempDir()
	configDir := t.TempDir()
	writeHeartbeatRuntimeMeta(t, configDir)

	_, err := newHeartbeatRunner(installDir, configDir, HeartbeatOptions{})
	if err == nil {
		t.Fatal("expected missing SOCKS tunnel error")
	}
	if !strings.Contains(err.Error(), "SOCKS tunnel is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewHeartbeatRunnerAcceptsSocksTunnel(t *testing.T) {
	installDir := t.TempDir()
	configDir := t.TempDir()
	writeHeartbeatRuntimeMeta(t, configDir)

	runner, err := newHeartbeatRunner(installDir, configDir, HeartbeatOptions{SocksAddress: "127.0.0.1:1080"})
	if err != nil {
		t.Fatalf("newHeartbeatRunner: %v", err)
	}
	if runner.socks != "127.0.0.1:1080" {
		t.Fatalf("unexpected SOCKS address: %q", runner.socks)
	}
}

func TestHeartbeatRunnerMarksFailedTunnelHeartbeatDead(t *testing.T) {
	store, err := heartbeat.NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	runner := &heartbeatRunner{
		store:    store,
		interval: time.Second,
	}
	endpoint := clientEndpointRecord{
		Hostname: "edge.example",
		Tag:      "proxy-edge",
		User:     "alice",
	}

	runner.updateLocalHeartbeat(endpoint, false, 0)

	snapshots := store.Snapshot(time.Now().UTC(), time.Second)
	if len(snapshots) != 1 {
		t.Fatalf("expected one heartbeat snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Alive {
		t.Fatal("failed tunnel heartbeat must mark the endpoint dead")
	}
}

func TestHeartbeatRunnerMarksSuccessfulTunnelHeartbeatAlive(t *testing.T) {
	store, err := heartbeat.NewStore("")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	runner := &heartbeatRunner{
		store:    store,
		interval: time.Second,
	}
	endpoint := clientEndpointRecord{
		Hostname: "edge.example",
		Tag:      "proxy-edge",
		User:     "alice",
	}

	runner.updateLocalHeartbeat(endpoint, true, 17)

	snapshots := store.Snapshot(time.Now().UTC(), time.Second)
	if len(snapshots) != 1 {
		t.Fatalf("expected one heartbeat snapshot, got %d", len(snapshots))
	}
	if !snapshots[0].Alive {
		t.Fatal("successful tunnel heartbeat must mark the endpoint alive")
	}
	if snapshots[0].Entry.LastRTTMillis != 17 {
		t.Fatalf("expected RTT to be recorded, got %d", snapshots[0].Entry.LastRTTMillis)
	}
}

func TestHeartbeatRunnerBackoffSkipsFailedEndpoint(t *testing.T) {
	runner := &heartbeatRunner{
		interval: time.Second,
		timeout:  1500 * time.Millisecond,
	}
	endpoint := clientEndpointRecord{
		Hostname: "edge.example",
		Tag:      "proxy-edge",
		User:     "alice",
	}

	runner.recordHeartbeatFailure(endpoint)

	if !runner.endpointInBackoff(endpoint, time.Now()) {
		t.Fatal("expected endpoint to be skipped while backoff is active")
	}
	if runner.heartbeatBackoffDelay(1) != 1500*time.Millisecond {
		t.Fatalf("expected timeout-sized first backoff, got %s", runner.heartbeatBackoffDelay(1))
	}
}

func TestHeartbeatRunnerBackoffClearsAfterSuccess(t *testing.T) {
	runner := &heartbeatRunner{interval: time.Second}
	endpoint := clientEndpointRecord{Tag: "proxy-edge", User: "alice"}

	runner.recordHeartbeatFailure(endpoint)
	runner.recordHeartbeatSuccess(endpoint)

	if runner.endpointInBackoff(endpoint, time.Now()) {
		t.Fatal("expected success to clear endpoint backoff")
	}
}

func TestHeartbeatRunnerBackoffDelayIsCapped(t *testing.T) {
	runner := &heartbeatRunner{
		interval: 5 * time.Second,
		timeout:  5 * time.Second,
	}

	if got := runner.heartbeatBackoffDelay(8); got != 30*time.Second {
		t.Fatalf("expected capped backoff of 30s, got %s", got)
	}
}

func TestHeartbeatRunnerReusesControlClient(t *testing.T) {
	runner := &heartbeatRunner{
		timeout: time.Second,
		socks:   "127.0.0.1:1080",
	}
	endpoint := clientEndpointRecord{
		Tag:                  "proxy-edge",
		User:                 "alice",
		ServerName:           "edge.example",
		AllowInsecure:        true,
		PinnedPeerCertSHA256: "abc",
	}

	first := runner.heartbeatControlClient(endpoint)
	second := runner.heartbeatControlClient(endpoint)
	if first == nil || first != second {
		t.Fatal("expected heartbeat control client to be reused")
	}
	if len(runner.clients) != 1 {
		t.Fatalf("expected one cached client, got %d", len(runner.clients))
	}
}

func TestHeartbeatRunnerSeparatesControlClientsByTLSSettings(t *testing.T) {
	runner := &heartbeatRunner{
		timeout: time.Second,
		socks:   "127.0.0.1:1080",
	}
	endpoint := clientEndpointRecord{Tag: "proxy-edge", User: "alice", ServerName: "edge.example"}

	first := runner.heartbeatControlClient(endpoint)
	endpoint.PinnedPeerCertSHA256 = "def"
	second := runner.heartbeatControlClient(endpoint)

	if first == second {
		t.Fatal("expected distinct clients for distinct TLS settings")
	}
	if len(runner.clients) != 2 {
		t.Fatalf("expected two cached clients, got %d", len(runner.clients))
	}
}

func TestHeartbeatRunnerPrunesStaleControlClients(t *testing.T) {
	runner := &heartbeatRunner{
		timeout: time.Second,
		socks:   "127.0.0.1:1080",
	}
	active := clientEndpointRecord{Tag: "active", User: "alice"}
	stale := clientEndpointRecord{Tag: "stale", User: "alice"}

	runner.heartbeatControlClient(active)
	runner.heartbeatControlClient(stale)
	runner.pruneHeartbeatControlClients([]clientEndpointRecord{active})

	if len(runner.clients) != 1 {
		t.Fatalf("expected one cached client after prune, got %d", len(runner.clients))
	}
	if runner.clients[heartbeatControlClientKey(active, runner.socks, runner.timeout)] == nil {
		t.Fatal("expected active client to remain cached")
	}
}

func writeHeartbeatRuntimeMeta(t *testing.T, dir string) {
	t.Helper()
	meta := runtimeMeta{
		Desired: runtimeDesired{Endpoints: []runtimeEndpoint{{
			Hostname: "edge.example",
			Tag:      "proxy-edge",
			Port:     443,
			User:     "alice",
		}}},
	}
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal runtime meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, layout.RuntimeMetaFileName), data, 0o644); err != nil {
		t.Fatalf("write runtime meta: %v", err)
	}
}
