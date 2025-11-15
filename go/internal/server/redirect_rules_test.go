package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerAddRedirectUpdatesStateAndRouting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-server")
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

	statePath := serverStatePath(dir)
	stateDoc := readJSONFile(t, statePath)
	rawRules, ok := stateDoc[serverRedirectRulesKey].([]any)
	if !ok || len(rawRules) != 1 {
		t.Fatalf("expected redirect entry, got %+v", stateDoc[serverRedirectRulesKey])
	}

	routingPath := filepath.Join(configDir, "routing.json")
	routingDoc := readJSONFile(t, routingPath)
	routingObj, _ := routingDoc["routing"].(map[string]any)
	rules := extractInterfaceSlice(routingObj["rules"])
	if len(rules) != 1 {
		t.Fatalf("expected single routing rule, got %d", len(rules))
	}
	ruleMap, _ := rules[0].(map[string]any)
	if got := extractStringSlice(ruleMap["domains"]); len(got) != 1 || got[0] != "svc.example.net" {
		t.Fatalf("unexpected domain routing rule: %+v", ruleMap)
	}
	if tag := ruleMap["outboundTag"]; tag != "alphaedge-example.rev" {
		t.Fatalf("unexpected outbound tag %v", tag)
	}

	records, err := ListRedirects(RedirectListOptions{
		InstallDir: dir,
		ConfigDir:  configDir,
	})
	if err != nil {
		t.Fatalf("ListRedirects failed: %v", err)
	}
	if len(records) != 1 || records[0].Hostname != "edge.example" || records[0].Value != "svc.example.net" || records[0].Type != "domain" {
		t.Fatalf("unexpected redirect records: %+v", records)
	}
}

func TestServerRemoveRedirectCleansState(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-server")
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

	stateDoc := readJSONFile(t, serverStatePath(dir))
	if _, ok := stateDoc[serverRedirectRulesKey]; ok {
		t.Fatalf("expected redirect rules cleared, got %+v", stateDoc[serverRedirectRulesKey])
	}

	routingDoc := readJSONFile(t, filepath.Join(configDir, "routing.json"))
	routingObj, _ := routingDoc["routing"].(map[string]any)
	rules := extractInterfaceSlice(routingObj["rules"])
	if len(rules) != 0 {
		t.Fatalf("expected routing rules cleared, got %+v", rules)
	}
}

func TestServerAddRedirectFailsWithoutReverse(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-server")
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
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("marshal state: %v", err)
	}
	if err := os.WriteFile(serverStatePath(installDir), data, 0o644); err != nil {
		t.Fatalf("write state: %v", err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(data) == 0 {
		return map[string]any{}
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}
