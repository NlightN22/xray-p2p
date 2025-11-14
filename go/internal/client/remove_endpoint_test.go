package client

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestRemoveEndpointUpdatesStateAndConfigs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDirName := layout.ClientConfigDir
	configDirPath := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(configDirPath, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient))
	initial := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname:      "server-a.example",
				Tag:           "proxy-server-a",
				Address:       "198.51.100.10",
				Port:          8443,
				User:          "alice@example.com",
				Password:      "secret-a",
				ServerName:    "server-a.example",
				AllowInsecure: false,
			},
			{
				Hostname:      "server-b.example",
				Tag:           "proxy-server-b",
				Address:       "203.0.113.20",
				Port:          9443,
				User:          "bob@example.com",
				Password:      "secret-b",
				ServerName:    "server-b.example",
				AllowInsecure: true,
			},
		},
		Redirects: []clientRedirectRule{
			{CIDR: "10.50.0.0/16", OutboundTag: "proxy-server-a"},
			{CIDR: "10.60.0.0/16", OutboundTag: "proxy-server-b"},
		},
	}
	if err := initial.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	ctx := context.Background()
	err := RemoveEndpoint(ctx, RemoveEndpointOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		Target:     "server-a.example",
	})
	if err != nil {
		t.Fatalf("RemoveEndpoint failed: %v", err)
	}

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load updated state: %v", err)
	}
	if len(updated.Endpoints) != 1 {
		t.Fatalf("expected 1 endpoint remaining, got %d", len(updated.Endpoints))
	}
	if updated.Endpoints[0].Tag != "proxy-server-b" {
		t.Fatalf("unexpected remaining endpoint %+v", updated.Endpoints[0])
	}
	if len(updated.Redirects) != 1 || updated.Redirects[0].OutboundTag != "proxy-server-b" {
		t.Fatalf("redirects not filtered: %+v", updated.Redirects)
	}

	outboundsPath := filepath.Join(configDirPath, "outbounds.json")
	data, err := os.ReadFile(outboundsPath)
	if err != nil {
		t.Fatalf("read outbounds: %v", err)
	}
	var out struct {
		Outbounds []struct {
			Tag string `json:"tag"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("parse outbounds: %v", err)
	}
	if len(out.Outbounds) != 2 {
		t.Fatalf("expected trojan and direct outbounds, got %d", len(out.Outbounds))
	}
	if out.Outbounds[0].Tag != "proxy-server-b" {
		t.Fatalf("unexpected trojan outbound %s", out.Outbounds[0].Tag)
	}

	routingPath := filepath.Join(configDirPath, "routing.json")
	routing, err := os.ReadFile(routingPath)
	if err != nil {
		t.Fatalf("read routing: %v", err)
	}
	var doc struct {
		Routing struct {
			Rules []struct {
				OutboundTag string   `json:"outboundTag"`
				IP          []string `json:"ip"`
			} `json:"rules"`
		} `json:"routing"`
	}
	if err := json.Unmarshal(routing, &doc); err != nil {
		t.Fatalf("parse routing: %v", err)
	}
	for _, rule := range doc.Routing.Rules {
		if strings.Contains(rule.OutboundTag, "server-a") {
			t.Fatalf("found rule for removed endpoint: %+v", rule)
		}
	}
}

func TestRemoveEndpointRemovesAllWhenNoEndpointsRemain(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDirName := layout.ClientConfigDir
	configDirPath := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(configDirPath, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient))
	initial := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname: "server-only.example",
				Tag:      "proxy-server-only",
				Address:  "192.0.2.10",
				Port:     8443,
				User:     "solo@example.com",
			},
		},
	}
	if err := initial.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	ctx := context.Background()
	err := RemoveEndpoint(ctx, RemoveEndpointOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		Target:     "proxy-server-only",
	})
	if err != nil {
		t.Fatalf("RemoveEndpoint failed: %v", err)
	}

	if _, err := os.Stat(configDirPath); !os.IsNotExist(err) {
		t.Fatalf("config dir should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("state file should be removed, stat err=%v", err)
	}
}
