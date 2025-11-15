package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteOutboundsConfigIncludesEndpointsAndFreedom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbounds.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example", Port: 8443, User: "alpha", Password: "secret", ServerName: "alpha.example"},
		{Hostname: "beta.example", Tag: "proxy-beta", Address: "beta.example", Port: 9443, User: "beta", Password: "other", ServerName: "beta.example"},
	}

	if err := writeOutboundsConfig(path, endpoints); err != nil {
		t.Fatalf("writeOutboundsConfig failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read outbounds: %v", err)
	}
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse outbounds: %v", err)
	}
	if len(doc.Outbounds) != 3 {
		t.Fatalf("expected 3 outbounds (2 trojan + freedom), got %d", len(doc.Outbounds))
	}
	if doc.Outbounds[0]["tag"] != "proxy-alpha" || doc.Outbounds[1]["tag"] != "proxy-beta" {
		t.Fatalf("unexpected tags: %+v", doc.Outbounds)
	}
	if doc.Outbounds[2]["tag"] != "direct" {
		t.Fatalf("expected last outbound to be direct, got %+v", doc.Outbounds[2])
	}
}

func TestUpdateRoutingConfigManagesReverseRules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example"},
	}
	reverse := map[string]clientReverseChannel{
		"alphaalpha-example.rev": {UserID: "alpha", Host: "alpha.example", Tag: "alphaalpha-example.rev", Domain: "alphaalpha-example.rev", EndpointTag: "proxy-alpha"},
	}

	if err := updateRoutingConfig(path, endpoints, nil, reverse); err != nil {
		t.Fatalf("updateRoutingConfig failed: %v", err)
	}

	verifyRoutingDocument(t, path, 3, 1)

	// Second update should not duplicate rules/bridges.
	if err := updateRoutingConfig(path, endpoints, nil, reverse); err != nil {
		t.Fatalf("second updateRoutingConfig failed: %v", err)
	}
	verifyRoutingDocument(t, path, 3, 1)
}

func verifyRoutingDocument(t *testing.T, path string, wantRules int, wantBridges int) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read routing: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse routing: %v", err)
	}
	routing, ok := doc["routing"].(map[string]any)
	if !ok {
		t.Fatalf("routing section missing")
	}
	rules, _ := routing["rules"].([]any)
	if len(rules) != wantRules {
		t.Fatalf("expected %d routing rules, got %d", wantRules, len(rules))
	}
	reverseObj, _ := doc["reverse"].(map[string]any)
	bridges, _ := reverseObj["bridges"].([]any)
	if len(bridges) != wantBridges {
		t.Fatalf("expected %d bridges, got %d", wantBridges, len(bridges))
	}
}
