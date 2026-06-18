package client

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
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
	if rule := findRuleWithIPAndTag(rules, "10.70.0.0/16", "proxy-server-example"); rule["ruleTag"] == "" {
		t.Fatalf("missing redirect ruleTag %+v", rule)
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
	if rule := findRuleWithDomainAndTag(rules, "app.service.example", "proxy-server-example"); rule["ruleTag"] == "" {
		t.Fatalf("missing domain redirect ruleTag %+v", rule)
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
