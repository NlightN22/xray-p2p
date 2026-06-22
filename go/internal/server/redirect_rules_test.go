//go:build linux || windows

package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestServerAddRedirectUpdatesStateAndRouting(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"alphaedge-example.rev": {
			UserID: "alpha",
			Host:   "edge.example",
			Tag:    "alphaedge-example.rev",
			Domain: "alphaedge-example.rev",
		},
	}, nil)

	if err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		Domain:     "svc.example.net",
		Hostname:   "edge.example",
	}); err != nil {
		t.Fatalf("AddRedirect failed: %v", err)
	}

	statePath := pendingConfigPath()
	stateDoc := readServerStateDoc(t, statePath)
	rawRules, ok := stateDoc[serverRedirectRulesKey].([]any)
	if !ok || len(rawRules) != 1 {
		t.Fatalf("expected redirect entry, got %+v", stateDoc[serverRedirectRulesKey])
	}

	compiled := compileDesiredDoc(t, pendingConfigPath(), configDir)
	rules := extractRoutingRules(t, compiled)
	if !hasRedirectRule(rules, "alphaedge-example.rev", "svc.example.net", "") {
		t.Fatalf("expected redirect rule for svc.example.net, got %v", rules)
	}
	if rule := findRedirectRule(rules, "alphaedge-example.rev", "svc.example.net", ""); rule["ruleTag"] == "" {
		t.Fatalf("expected redirect ruleTag, got %+v", rule)
	}

	records, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		Pending:    true,
	})
	if err != nil {
		t.Fatalf("ListRedirects failed: %v", err)
	}
	if len(records) != 1 || records[0].Hostname != "edge.example" || records[0].Value != "svc.example.net" || records[0].Type != "domain" {
		t.Fatalf("unexpected redirect records: %+v", records)
	}
}

func TestServerRemoveRedirectCleansState(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"alphaedge-example.rev": {
			UserID: "alpha",
			Host:   "edge.example",
			Tag:    "alphaedge-example.rev",
			Domain: "alphaedge-example.rev",
		},
	}, nil)

	if err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "10.50.0.0/16",
		Hostname:   "edge.example",
	}); err != nil {
		t.Fatalf("AddRedirect failed: %v", err)
	}

	if err := RemoveRedirect(RedirectRemoveOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "10.50.0.0/16",
	}); err != nil {
		t.Fatalf("RemoveRedirect failed: %v", err)
	}

	stateDoc := readServerStateDoc(t, pendingConfigPath())
	if _, ok := stateDoc[serverRedirectRulesKey]; ok {
		t.Fatalf("expected redirect rules cleared, got %+v", stateDoc[serverRedirectRulesKey])
	}

	compiled := compileDesiredDoc(t, pendingConfigPath(), configDir)
	rules := extractRoutingRules(t, compiled)
	if hasRedirectRule(rules, "", "10.50.0.0/16", "10.50.0.0/16") {
		t.Fatalf("expected redirect rule to be removed, got %+v", rules)
	}
}

func TestServerAddRedirectKeepsExistingTargetBindings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"alphaedge-example.rev": {
			UserID: "alpha",
			Host:   "edge.example",
			Tag:    "alphaedge-example.rev",
			Domain: "alphaedge-example.rev",
		},
		"betaedge-example.rev": {
			UserID: "beta",
			Host:   "edge.example",
			Tag:    "betaedge-example.rev",
			Domain: "betaedge-example.rev",
		},
	}, []map[string]any{
		{
			"cidr":         "10.50.0.0/16",
			"outbound_tag": "alphaedge-example.rev",
		},
	})

	if err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "10.50.0.0/16",
		Tag:        "betaedge-example.rev",
	}); err != nil {
		t.Fatalf("AddRedirect failed: %v", err)
	}

	stateDoc := readServerStateDoc(t, pendingConfigPath())
	rawRules, ok := stateDoc[serverRedirectRulesKey].([]any)
	if !ok || len(rawRules) != 2 {
		t.Fatalf("expected two redirect entries, got %+v", stateDoc[serverRedirectRulesKey])
	}
	var sawAlpha, sawBeta bool
	for _, raw := range rawRules {
		rule, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("unexpected redirect entry: %+v", raw)
		}
		switch rule["outbound_tag"] {
		case "alphaedge-example.rev":
			sawAlpha = true
		case "betaedge-example.rev":
			sawBeta = true
		}
	}
	if !sawAlpha || !sawBeta {
		t.Fatalf("expected redirects via alpha and beta, got %+v", rawRules)
	}
}

func TestServerAddRedirectByHostReplacesExistingTargetBinding(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"alphaedge-example.rev": {
			UserID: "alpha",
			Host:   "edge.example",
			Tag:    "alphaedge-example.rev",
			Domain: "alphaedge-example.rev",
		},
		"betaedge-example.rev": {
			UserID: "beta",
			Host:   "edge.example",
			Tag:    "betaedge-example.rev",
			Domain: "betaedge-example.rev",
		},
	}, []map[string]any{
		{
			"cidr":         "10.50.0.0/16",
			"outbound_tag": "alphaedge-example.rev",
		},
	})

	if err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "10.50.0.0/16",
		Tag:        "betaedge-example.rev",
		Hostname:   "edge.example",
	}); err != nil {
		t.Fatalf("AddRedirect failed: %v", err)
	}

	stateDoc := readServerStateDoc(t, pendingConfigPath())
	rawRules, ok := stateDoc[serverRedirectRulesKey].([]any)
	if !ok || len(rawRules) != 1 {
		t.Fatalf("expected one redirect entry, got %+v", stateDoc[serverRedirectRulesKey])
	}
	rule, ok := rawRules[0].(map[string]any)
	if !ok || rule["outbound_tag"] != "betaedge-example.rev" {
		t.Fatalf("expected redirect via beta, got %+v", rawRules[0])
	}
}

func TestServerRemoveMissingRedirectDoesNotWriteApplyRequest(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	writeServerStateFile(t, dir, map[string]serverReverseChannel{
		"alphaedge-example.rev": {
			UserID: "alpha",
			Host:   "edge.example",
			Tag:    "alphaedge-example.rev",
			Domain: "alphaedge-example.rev",
		},
	}, []map[string]any{
		{
			"cidr":         "10.50.0.0/16",
			"outbound_tag": "alphaedge-example.rev",
		},
	})

	err := RemoveRedirect(RedirectRemoveOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "10.60.0.0/16",
		Hostname:   "edge.example",
	})
	if err == nil {
		t.Fatalf("expected missing redirect error")
	}
	if _, statErr := os.Stat(config.ApplyRequestPath()); !os.IsNotExist(statErr) {
		t.Fatalf("apply request should not be written for missing redirect: %v", statErr)
	}
}

func TestServerAddRedirectFailsWithoutReverse(t *testing.T) {

	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	writeServerStateFile(t, dir, nil, nil)
	err := AddRedirect(RedirectAddOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
		CIDR:       "10.60.0.0/16",
		Hostname:   "missing.example",
	})
	if err == nil || !strings.Contains(err.Error(), "no reverse portals") {
		t.Fatalf("expected reverse portal error, got %v", err)
	}
}

func writeServerStateFile(t *testing.T, installDir string, reverse map[string]serverReverseChannel, redirects []map[string]any) {
	t.Helper()
	doc := make(map[string]any)
	if len(reverse) > 0 {
		doc[serverReverseStateKey] = reverse
	}
	if len(redirects) > 0 {
		doc[serverRedirectRulesKey] = redirects
	}
	if err := writeServerStateDoc(pendingConfigPath(), doc); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func readServerStateDoc(t *testing.T, path string) map[string]any {
	t.Helper()
	doc, err := loadServerStateDoc(path)
	if err != nil {
		t.Fatalf("read server state: %v", err)
	}
	return doc
}

func hasRedirectRule(rules []any, outboundTag string, domain string, cidr string) bool {
	return findRedirectRule(rules, outboundTag, domain, cidr) != nil
}

func findRedirectRule(rules []any, outboundTag string, domain string, cidr string) map[string]any {
	wantTag := strings.TrimSpace(outboundTag)
	wantDomain := strings.TrimSpace(domain)
	wantCIDR := strings.TrimSpace(cidr)
	for _, raw := range rules {
		ruleMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if wantTag != "" {
			if tag, _ := ruleMap["outboundTag"].(string); tag != wantTag {
				continue
			}
		}
		if wantDomain != "" {
			domains := extractStringSlice(ruleMap["domains"])
			for _, value := range domains {
				if strings.EqualFold(value, wantDomain) || strings.EqualFold(value, "domain:"+wantDomain) {
					return ruleMap
				}
			}
			continue
		}
		if wantCIDR != "" {
			for _, value := range extractStringSlice(ruleMap["ip"]) {
				if strings.EqualFold(value, wantCIDR) {
					return ruleMap
				}
			}
		}
	}
	return nil
}
