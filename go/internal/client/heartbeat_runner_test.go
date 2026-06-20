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

	runner.updateLocalHeartbeat(endpoint, false)

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

	runner.updateLocalHeartbeat(endpoint, true)

	snapshots := store.Snapshot(time.Now().UTC(), time.Second)
	if len(snapshots) != 1 {
		t.Fatalf("expected one heartbeat snapshot, got %d", len(snapshots))
	}
	if !snapshots[0].Alive {
		t.Fatal("successful tunnel heartbeat must mark the endpoint alive")
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
