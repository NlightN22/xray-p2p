package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func TestAddRedirectUpdatesStateAndRouting(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	configDirPath := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(configDirPath, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
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
	if len(doc.Routing.Rules) != 4+windowsRuleBonus() {
		t.Fatalf("expected %d routing rules, got %d", 4+windowsRuleBonus(), len(doc.Routing.Rules))
	}
	if !hasIPRule(doc.Routing.Rules, "10.70.0.0/16", "proxy-server-example") {
		t.Fatalf("missing redirect rule %+v", doc.Routing.Rules)
	}
	if !hasIPRule(doc.Routing.Rules, "203.0.113.10", "proxy-server-example") {
		t.Fatalf("missing endpoint rule %+v", doc.Routing.Rules)
	}
	if !hasMarkerRule(doc.Routing.Rules) {
		t.Fatalf("missing marker rule %+v", doc.Routing.Rules)
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
	})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("unexpected list result %+v", list)
	}
	if list[0].Value != "10.70.0.0/16" || list[0].Hostname != "server.example" || list[0].Type != "CIDR" {
		t.Fatalf("unexpected redirect entry %+v", list[0])
	}
}

func TestAddDomainRedirectUpdatesStateAndRouting(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	configDirPath := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(configDirPath, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
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
	if len(doc.Routing.Rules) != 4+windowsRuleBonus() {
		t.Fatalf("expected %d routing rules, got %d", 4+windowsRuleBonus(), len(doc.Routing.Rules))
	}
	if !hasDomainRule(doc.Routing.Rules, "app.service.example", "proxy-server-example") {
		t.Fatalf("missing redirect rule %+v", doc.Routing.Rules)
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
	})
	if err != nil {
		t.Fatalf("list redirects: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("unexpected list result %+v", list)
	}
	if list[0].Value != "app.service.example" || list[0].Type != "domain" || list[0].Hostname != "server.example" {
		t.Fatalf("unexpected domain entry %+v", list[0])
	}
}

func TestRemoveRedirectByTag(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	configDirPath := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(configDirPath, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
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
	if err := updateRoutingConfig(filepath.Join(configDirPath, "routing.json"), xrayconfig.DefaultClientConfig().Routing, state.Endpoints, state.Redirects, state.Reverse); err != nil {
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
	if list[0].Value != "10.70.0.0/16" || list[0].Tag != "proxy-server-example" {
		t.Fatalf("unexpected list entry %+v", list[0])
	}
}

func TestRemoveDomainRedirect(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	configDirPath := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(configDirPath, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
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
	if err := updateRoutingConfig(routingPath, xrayconfig.DefaultClientConfig().Routing, state.Endpoints, state.Redirects, state.Reverse); err != nil {
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
	if len(doc.Routing.Rules) != 4+windowsRuleBonus() {
		t.Fatalf("expected %d routing rules, got %d", 4+windowsRuleBonus(), len(doc.Routing.Rules))
	}
	for _, rule := range doc.Routing.Rules {
		if len(rule.Domains) > 0 {
			t.Fatalf("found domain rule after removal: %+v", rule)
		}
	}
}

func hasIPRule(rules []struct {
	Type        string   `json:"type"`
	IP          []string `json:"ip"`
	OutboundTag string   `json:"outboundTag"`
}, ip, tag string) bool {
	for _, rule := range rules {
		if rule.OutboundTag != tag {
			continue
		}
		for _, value := range rule.IP {
			if value == ip {
				return true
			}
		}
	}
	return false
}

func hasDomainRule(rules []struct {
	Type        string   `json:"type"`
	Domains     []string `json:"domains"`
	OutboundTag string   `json:"outboundTag"`
}, domain, tag string) bool {
	for _, rule := range rules {
		if rule.OutboundTag != tag {
			continue
		}
		for _, value := range rule.Domains {
			if value == domain {
				return true
			}
		}
	}
	return false
}

func windowsRuleBonus() int {
	if runtime.GOOS == "windows" {
		return 2
	}
	return 0
}

func hasMarkerRule(rules []struct {
	Type        string   `json:"type"`
	IP          []string `json:"ip"`
	OutboundTag string   `json:"outboundTag"`
}) bool {
	for _, rule := range rules {
		for _, value := range rule.IP {
			if strings.HasPrefix(value, "127.255.") {
				return true
			}
		}
	}
	return false
}

func TestListRedirectsReportsMixedRecords(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	if err := os.MkdirAll(filepath.Join(dir, configDirName), 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	statePath := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))
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
