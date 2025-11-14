package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestAddRedirectUpdatesStateAndRouting(t *testing.T) {
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
				Hostname: "server.example",
				Tag:      "proxy-server-example",
				Address:  "203.0.113.10",
			},
		},
	}
	if err := initial.save(statePath); err != nil {
		t.Fatalf("save initial state: %v", err)
	}

	opts := RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		CIDR:       "10.70.0.0/16",
		Hostname:   "server.example",
	}
	if err := AddRedirect(opts); err != nil {
		t.Fatalf("AddRedirect failed: %v", err)
	}

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(updated.Redirects) != 1 {
		t.Fatalf("expected 1 redirect, got %d", len(updated.Redirects))
	}
	if updated.Redirects[0].CIDR != "10.70.0.0/16" {
		t.Fatalf("unexpected CIDR %s", updated.Redirects[0].CIDR)
	}
	if updated.Redirects[0].OutboundTag != "proxy-server-example" {
		t.Fatalf("unexpected outbound tag %s", updated.Redirects[0].OutboundTag)
	}

	routingPath := filepath.Join(configDirPath, "routing.json")
	data, err := os.ReadFile(routingPath)
	if err != nil {
		t.Fatalf("read routing: %v", err)
	}

	var doc struct {
		Routing struct {
			Rules []struct {
				Type        string   `json:"type"`
				IP          []string `json:"ip"`
				OutboundTag string   `json:"outboundTag"`
			} `json:"rules"`
		} `json:"routing"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse routing: %v", err)
	}
	if len(doc.Routing.Rules) != 2 {
		t.Fatalf("expected 2 routing rules, got %d", len(doc.Routing.Rules))
	}
	if doc.Routing.Rules[0].OutboundTag != "proxy-server-example" || len(doc.Routing.Rules[0].IP) != 1 || doc.Routing.Rules[0].IP[0] != "10.70.0.0/16" {
		t.Fatalf("unexpected redirect rule %+v", doc.Routing.Rules[0])
	}
	if doc.Routing.Rules[1].OutboundTag != "proxy-server-example" || len(doc.Routing.Rules[1].IP) != 1 || doc.Routing.Rules[1].IP[0] != "203.0.113.10" {
		t.Fatalf("unexpected endpoint rule %+v", doc.Routing.Rules[1])
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
	})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(list) != 1 || list[0].Hostname != "server.example" || list[0].Tag != "proxy-server-example" {
		t.Fatalf("unexpected list result %+v", list)
	}
}

func TestRemoveRedirectByTag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDirName := layout.ClientConfigDir
	configDirPath := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(configDirPath, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient))
	state := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{
				Hostname: "server.example",
				Tag:      "proxy-server-example",
				Address:  "203.0.113.10",
			},
		},
		Redirects: []clientRedirectRule{
			{CIDR: "10.70.0.0/16", OutboundTag: "proxy-server-example"},
			{CIDR: "10.90.0.0/16", OutboundTag: "proxy-server-example"},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := updateRoutingConfig(filepath.Join(configDirPath, "routing.json"), state.Endpoints, state.Redirects); err != nil {
		t.Fatalf("seed routing config: %v", err)
	}

	opts := RedirectRemoveOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		CIDR:       "10.90.0.0/16",
		Tag:        "proxy-server-example",
	}
	if err := RemoveRedirect(opts); err != nil {
		t.Fatalf("RemoveRedirect failed: %v", err)
	}

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(updated.Redirects) != 1 {
		t.Fatalf("expected 1 redirect remaining, got %d", len(updated.Redirects))
	}
	if updated.Redirects[0].CIDR != "10.70.0.0/16" {
		t.Fatalf("unexpected remaining CIDR %s", updated.Redirects[0].CIDR)
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
	})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 list entry, got %d", len(list))
	}
	if list[0].CIDR != "10.70.0.0/16" {
		t.Fatalf("unexpected list entry %+v", list[0])
	}
}
