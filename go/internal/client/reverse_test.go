package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyClientEndpointConfigAddsReverseRules(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config-client")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	stateFile := filepath.Join(dir, "install-state-client.json")

	endpoint := endpointConfig{
		Hostname:   "server.example",
		Port:       8443,
		User:       "reverse.user",
		Password:   "secret",
		ServerName: "server.example",
	}
	if err := applyClientEndpointConfig(configDir, stateFile, endpoint, true); err != nil {
		t.Fatalf("applyClientEndpointConfig: %v", err)
	}

	routingPath := filepath.Join(configDir, "routing.json")
	doc := loadClientRouting(t, routingPath)
	reverse := doc["reverse"].(map[string]any)
	bridges := reverse["bridges"].([]any)
	if len(bridges) != 1 {
		t.Fatalf("expected 1 reverse bridge, got %d", len(bridges))
	}
	entry := bridges[0].(map[string]any)
	if entry["tag"] != "reverse-userserver-example.rev" || entry["domain"] != "reverse-userserver-example.rev" {
		t.Fatalf("unexpected bridge entry: %+v", entry)
	}

	rules := doc["routing"].(map[string]any)["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("expected 3 routing rules, got %d", len(rules))
	}
	domainRule := rules[0].(map[string]any)
	if domainRule["outboundTag"] != "proxy-server-example" {
		t.Fatalf("unexpected outbound tag: %+v", domainRule)
	}
	if domains := domainRule["domain"].([]any); domains[0] != "full:reverse-userserver-example.rev" {
		t.Fatalf("unexpected domains: %+v", domains)
	}
	directRule := rules[1].(map[string]any)
	if directRule["outboundTag"] != "direct" {
		t.Fatalf("expected direct outbound, got %+v", directRule)
	}
	inbound := directRule["inboundTag"].([]any)
	if inbound[0] != "reverse-userserver-example.rev" {
		t.Fatalf("unexpected inbound tag: %+v", inbound)
	}

	state, err := loadClientInstallState(stateFile)
	if err != nil {
		t.Fatalf("load client state: %v", err)
	}
	if _, ok := state.Reverse["reverse-userserver-example.rev"]; !ok {
		t.Fatalf("expected reverse entry in state, got %v", state.Reverse)
	}
}

func loadClientRouting(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read routing: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse routing: %v", err)
	}
	return doc
}
