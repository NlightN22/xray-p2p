package servercmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/heartbeat"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestSnapshotServerConfiguredStateUsesUsersAsSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	configData := []byte(`
[server]
  trojan_users = [
    { email = "alice", password = "a" },
    { email = "bob", password = "b" },
  ]

[server.reverse_channels.proxy_alpha]
  user_id = "alice"
  host = "alpha.example"
`)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), configData, 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}

	statePath := filepath.Join(dir, layout.ServerHeartbeatStateFileName)
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

	snapshots, err := snapshotServerConfiguredState(statePath, dir, time.Minute)
	if err != nil {
		t.Fatalf("snapshot server state: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected two configured users, got %d: %+v", len(snapshots), snapshots)
	}
	byUser := make(map[string]heartbeat.Snapshot, len(snapshots))
	for _, snap := range snapshots {
		byUser[snap.Entry.User] = snap
	}
	if snap := byUser["alice"]; !snap.Alive || snap.Entry.Host != "alpha.example" {
		t.Fatalf("expected alice heartbeat to be preserved: %+v", snap)
	}
	if snap := byUser["bob"]; snap.Entry.Tag != "-" || snap.Alive {
		t.Fatalf("expected bob placeholder without stale entries: %+v", snap)
	}
}

func TestSnapshotServerConfiguredStatePrefersVersionedHeartbeatOverLegacy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	configData := []byte(`
[server]
  trojan_users = [
    { email = "alice", password = "a" },
  ]

[server.reverse_channels.proxy_alpha]
  user_id = "alice"
  host = "alpha.example"
`)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), configData, 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}

	now := time.Now().UTC()
	healthy := true
	statePath := filepath.Join(dir, layout.ServerHeartbeatStateFileName)
	state := heartbeat.State{Entries: map[string]heartbeat.Entry{
		"proxy-alpha|alice": {
			Tag:      "proxy-alpha",
			Host:     "alpha.example",
			User:     "alice",
			LastSeen: now.Add(-time.Hour),
		},
		"v1|v1:alpha": {
			Tag:         "proxy-alpha",
			Host:        "alpha.example",
			User:        "alice",
			LastSeen:    now,
			LastSuccess: now,
			Healthy:     &healthy,
			Status:      heartbeat.StatusHealthy,
			EndpointID:  "v1:alpha",
		},
	}}
	if err := heartbeat.Save(statePath, state); err != nil {
		t.Fatalf("save heartbeat state: %v", err)
	}

	for range 20 {
		snapshots, err := snapshotServerConfiguredState(statePath, dir, time.Minute)
		if err != nil {
			t.Fatalf("snapshot server state: %v", err)
		}
		if len(snapshots) != 1 {
			t.Fatalf("snapshot count = %d, want 1", len(snapshots))
		}
		snapshot := snapshots[0]
		if !snapshot.Alive || snapshot.Entry.EndpointID != "v1:alpha" {
			t.Fatalf("legacy heartbeat won over versioned heartbeat: %+v", snapshot)
		}
	}
}
