package client

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestRemoveEndpointUpdatesStateAndConfigs(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	extensionsDir := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
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
		Redirects: []redirect.Rule{
			{CIDR: "10.50.0.0/16", OutboundTag: "proxy-server-a"},
			{Domain: "svc.server-a.example", OutboundTag: "proxy-server-a"},
			{CIDR: "10.60.0.0/16", OutboundTag: "proxy-server-b"},
			{Domain: "svc.server-b.example", OutboundTag: "proxy-server-b"},
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
	if len(updated.Redirects) != 2 {
		t.Fatalf("unexpected redirects after filtering: %+v", updated.Redirects)
	}
	for _, rule := range updated.Redirects {
		if rule.OutboundTag != "proxy-server-b" {
			t.Fatalf("redirects not filtered: %+v", updated.Redirects)
		}
	}

	doc := compileDesiredDoc(t, statePath, extensionsDir)
	outbounds, ok := doc["outbounds"].([]any)
	if !ok {
		t.Fatalf("expected outbounds array, got %T", doc["outbounds"])
	}
	if len(outbounds) != 2 {
		t.Fatalf("expected 2 outbounds, got %d", len(outbounds))
	}
	first, _ := outbounds[0].(map[string]any)
	if tag, _ := first["tag"].(string); tag != "proxy-server-b" {
		t.Fatalf("unexpected first outbound tag %q", tag)
	}

	rules := extractRoutingRules(t, doc)
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		outbound, _ := rule["outboundTag"].(string)
		if strings.Contains(outbound, "server-a") {
			t.Fatalf("found rule for removed endpoint: %+v", rule)
		}
	}
}

func TestRemoveEndpointRemovesAllWhenNoEndpointsRemain(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
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

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(updated.Endpoints) != 0 {
		t.Fatalf("expected no endpoints remaining, got %d", len(updated.Endpoints))
	}
}
