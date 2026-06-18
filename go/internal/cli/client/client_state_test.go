package clientcmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestSnapshotClientConfiguredStateUsesEndpointsAsSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	configData := []byte(`
[client]
  endpoints = [
    { hostname = "alpha.example", tag = "proxy-alpha", address = "198.51.100.10", port = 443, user = "alice", password = "a", server_name = "alpha.example" },
    { hostname = "beta.example", tag = "proxy-beta", address = "198.51.100.20", port = 443, user = "bob", password = "b", server_name = "beta.example" },
  ]
`)
	if err := os.WriteFile(config.ConfigPath(layout.ClientConfigFileName), configData, 0o644); err != nil {
		t.Fatalf("write client config: %v", err)
	}

	statePath := filepath.Join(dir, layout.ClientHeartbeatStateFileName)
	state := heartbeat.State{Entries: map[string]heartbeat.Entry{
		"proxy-alpha|alice": {
			Tag:           "proxy-alpha",
			Host:          "alpha.example",
			User:          "alice",
			LastSeen:      time.Now().UTC(),
			LastRTTMillis: 25,
			Samples:       1,
		},
		"proxy-stale|old": {
			Tag:  "proxy-stale",
			Host: "stale.example",
			User: "old",
		},
	}}
	if err := heartbeat.Save(statePath, state); err != nil {
		t.Fatalf("save heartbeat state: %v", err)
	}

	snapshots, err := snapshotClientConfiguredState(dir, layout.ClientConfigDir, false, statePath, time.Minute)
	if err != nil {
		t.Fatalf("snapshot client state: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected two configured endpoints, got %d: %+v", len(snapshots), snapshots)
	}
	if snapshots[0].Entry.Tag != "proxy-alpha" || !snapshots[0].Alive {
		t.Fatalf("expected alpha heartbeat to be preserved: %+v", snapshots[0])
	}
	if snapshots[1].Entry.Tag != "proxy-beta" || snapshots[1].Entry.Host != "beta.example" || snapshots[1].Alive {
		t.Fatalf("expected beta placeholder without stale entries: %+v", snapshots[1])
	}
}
