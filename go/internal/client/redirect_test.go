package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func TestAddRedirectUpdatesStateAndRouting(t *testing.T) {

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

	doc := compileDesiredDoc(t, statePath, extensionsDir)
	rules := extractRoutingRules(t, doc)
	if !hasRuleWithIP(rules, "10.70.0.0/16", "proxy-server-example") {
		t.Fatalf("missing redirect rule %+v", rules)
	}
	if !hasRuleWithIP(rules, "203.0.113.10", "direct") {
		t.Fatalf("missing endpoint bypass rule %+v", rules)
	}
	if !hasMarkerRule(rules) {
		t.Fatalf("missing marker rule %+v", rules)
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		Pending:    true,
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
	extensionsDir := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
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

	doc := compileDesiredDoc(t, statePath, extensionsDir)
	rules := extractRoutingRules(t, doc)
	if !hasRuleWithDomains(rules, "app.service.example", "proxy-server-example") {
		t.Fatalf("missing domain redirect rule %+v", rules)
	}
	if !hasMarkerRule(rules) {
		t.Fatalf("missing marker rule %+v", rules)
	}

	list, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDirName,
		Pending:    true,
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
	if err := os.MkdirAll(filepath.Join(dir, configDirName), 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
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
		Pending:    true,
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
	extensionsDir := filepath.Join(dir, configDirName)
	if err := os.MkdirAll(extensionsDir, 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
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

	doc := compileDesiredDoc(t, statePath, extensionsDir)
	rules := extractRoutingRules(t, doc)
	if hasAnyDomainRedirectRule(rules) {
		t.Fatalf("found domain rule after removal: %+v", rules)
	}
}

func compileDesiredDoc(t *testing.T, configPath string, extensionsDir string) map[string]any {
	t.Helper()
	artifacts, err := compileDesired(configPath, extensionsDir)
	if err != nil {
		t.Fatalf("compile desired: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(artifacts.XrayJSON, &doc); err != nil {
		t.Fatalf("parse xray.json: %v", err)
	}
	return doc
}

func extractRoutingRules(t *testing.T, doc map[string]any) []any {
	t.Helper()
	routing, ok := doc["routing"].(map[string]any)
	if !ok {
		t.Fatalf("expected routing section, got %T", doc["routing"])
	}
	rules, ok := routing["rules"].([]any)
	if !ok {
		t.Fatalf("expected routing.rules array, got %T", routing["rules"])
	}
	return rules
}

func hasRuleWithIP(rules []any, ip, tag string) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		outbound, _ := rule["outboundTag"].(string)
		if outbound != tag {
			continue
		}
		for _, value := range extractStringSlice(rule["ip"]) {
			if value == ip {
				return true
			}
		}
	}
	return false
}

func hasRuleWithDomains(rules []any, domain, tag string) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		outbound, _ := rule["outboundTag"].(string)
		if outbound != tag {
			continue
		}
		for _, value := range extractStringSlice(rule["domains"]) {
			if value == domain {
				return true
			}
		}
	}
	return false
}

func hasMarkerRule(rules []any) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, value := range extractStringSlice(rule["ip"]) {
			if strings.HasPrefix(value, "127.255.") {
				return true
			}
		}
	}
	return false
}

func hasAnyDomainRedirectRule(rules []any) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if len(extractStringSlice(rule["domains"])) > 0 {
			return true
		}
	}
	return false
}

func TestListRedirectsReportsMixedRecords(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	configDirName := layout.ClientConfigDir
	if err := os.MkdirAll(filepath.Join(dir, configDirName), 0o755); err != nil {
		t.Fatalf("mkdir extensions dir: %v", err)
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
		Pending:    true,
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
