package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/installstate"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
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
	if len(list) != 2 {
		t.Fatalf("unexpected list result %+v", list)
	}
	foundCustom := false
	foundDefault := false
	for _, rec := range list {
		switch rec.Value {
		case "10.70.0.0/16":
			foundCustom = true
			if rec.Type != "CIDR" || rec.Hostname != "server.example" {
				t.Fatalf("unexpected custom entry %+v", rec)
			}
		case "203.0.113.10/32":
			foundDefault = true
			if rec.Type != "CIDR" || rec.Hostname != "server.example" {
				t.Fatalf("unexpected default entry %+v", rec)
			}
		default:
			t.Fatalf("unexpected list entry %+v", rec)
		}
	}
	if !foundCustom || !foundDefault {
		t.Fatalf("missing expected list entries %+v", list)
	}
}

func TestAddDomainRedirectUpdatesStateAndRouting(t *testing.T) {
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
		Domain:     "App.Service.Example",
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
	if updated.Redirects[0].Domain != "app.service.example" {
		t.Fatalf("unexpected domain %s", updated.Redirects[0].Domain)
	}
	if updated.Redirects[0].CIDR != "" {
		t.Fatalf("expected CIDR to be empty, got %s", updated.Redirects[0].CIDR)
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
				Domains     []string `json:"domains"`
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
	if doc.Routing.Rules[0].OutboundTag != "proxy-server-example" || len(doc.Routing.Rules[0].Domains) != 1 || doc.Routing.Rules[0].Domains[0] != "app.service.example" {
		t.Fatalf("unexpected redirect rule %+v", doc.Routing.Rules[0])
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
	})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("unexpected list result %+v", list)
	}
	foundDomain := false
	foundDefault := false
	for _, rec := range list {
		switch rec.Value {
		case "app.service.example":
			foundDomain = true
			if rec.Type != "domain" {
				t.Fatalf("unexpected domain entry %+v", rec)
			}
		case "203.0.113.10/32":
			foundDefault = true
			if rec.Type != "CIDR" {
				t.Fatalf("unexpected default entry %+v", rec)
			}
		default:
			t.Fatalf("unexpected list entry %+v", rec)
		}
	}
	if !foundDomain || !foundDefault {
		t.Fatalf("missing list entries %+v", list)
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
		Redirects: []redirect.Rule{
			{CIDR: "10.70.0.0/16", OutboundTag: "proxy-server-example"},
			{CIDR: "10.90.0.0/16", OutboundTag: "proxy-server-example"},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}
	if err := updateRoutingConfig(filepath.Join(configDirPath, "routing.json"), state.Endpoints, state.Redirects, state.Reverse); err != nil {
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
	if len(list) != 2 {
		t.Fatalf("expected 2 list entries, got %d", len(list))
	}
	expected := map[string]bool{
		"10.70.0.0/16":    false,
		"203.0.113.10/32": false,
	}
	for _, rec := range list {
		if _, ok := expected[rec.Value]; ok {
			expected[rec.Value] = true
		} else {
			t.Fatalf("unexpected list entry %+v", rec)
		}
	}
	for value, seen := range expected {
		if !seen {
			t.Fatalf("missing list entry for %s", value)
		}
	}
}

func TestRemoveDomainRedirect(t *testing.T) {
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
		Redirects: []redirect.Rule{
			{Domain: "api.example.com", OutboundTag: "proxy-server-example"},
			{CIDR: "10.90.0.0/16", OutboundTag: "proxy-server-example"},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}
	routingPath := filepath.Join(configDirPath, "routing.json")
	if err := updateRoutingConfig(routingPath, state.Endpoints, state.Redirects, state.Reverse); err != nil {
		t.Fatalf("seed routing config: %v", err)
	}

	opts := RedirectRemoveOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		Domain:     "api.example.com",
	}
	if err := RemoveRedirect(opts); err != nil {
		t.Fatalf("RemoveRedirect failed: %v", err)
	}

	updated, err := loadClientInstallState(statePath)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(updated.Redirects) != 1 || updated.Redirects[0].CIDR != "10.90.0.0/16" {
		t.Fatalf("unexpected remaining redirects %+v", updated.Redirects)
	}

	data, err := os.ReadFile(routingPath)
	if err != nil {
		t.Fatalf("read routing: %v", err)
	}
	var doc struct {
		Routing struct {
			Rules []struct {
				Domains     []string `json:"domains"`
				IP          []string `json:"ip"`
				OutboundTag string   `json:"outboundTag"`
			} `json:"rules"`
		} `json:"routing"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse routing: %v", err)
	}
	if len(doc.Routing.Rules) != 2 {
		t.Fatalf("expected redirect and endpoint rules, got %d", len(doc.Routing.Rules))
	}
	for _, rule := range doc.Routing.Rules {
		if len(rule.Domains) > 0 {
			t.Fatalf("found domain rule after removal: %+v", rule)
		}
	}
}

func TestListRedirectsReportsMixedRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDirName := layout.ClientConfigDir
	if err := os.MkdirAll(filepath.Join(dir, configDirName), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Join(dir, installstate.FileNameForKind(installstate.KindClient))
	state := clientInstallState{
		Endpoints: []clientEndpointRecord{
			{Hostname: "server-a.example", Tag: "proxy-server-a"},
			{Hostname: "server-b.example", Tag: "proxy-server-b"},
		},
		Redirects: []redirect.Rule{
			{CIDR: "10.100.0.0/16", OutboundTag: "proxy-server-a"},
			{Domain: "svc.example.net", OutboundTag: "proxy-server-b"},
		},
	}
	if err := state.save(statePath); err != nil {
		t.Fatalf("save state: %v", err)
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
	})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(list))
	}
	if list[0].Type != "CIDR" || list[0].Value != "10.100.0.0/16" || list[0].Hostname != "server-a.example" {
		t.Fatalf("unexpected first entry %+v", list[0])
	}
	if list[1].Type != "domain" || list[1].Value != "svc.example.net" || list[1].Hostname != "server-b.example" {
		t.Fatalf("unexpected second entry %+v", list[1])
	}
}
