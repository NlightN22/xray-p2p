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

func TestUpdateRoutingConfigUsesDomainRuleForHostname(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example"},
	}

	if err := updateRoutingConfig(path, endpoints, nil, nil); err != nil {
		t.Fatalf("updateRoutingConfig failed: %v", err)
	}

	doc := loadRouting(t, path)
	rules := getRules(t, doc)
	if len(rules) != 1 {
		t.Fatalf("expected 1 routing rule, got %d", len(rules))
	}
	rule := rules[0]
	if got := asStrings(rule["domain"]); len(got) != 1 || got[0] != "full:alpha.example" {
		t.Fatalf("expected domain rule for alpha.example, got %v", got)
	}
	if got := asStrings(rule["ip"]); len(got) != 0 {
		t.Fatalf("did not expect ip field, got %v", got)
	}
}

func TestUpdateRoutingConfigUsesIPRuleForAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "192.0.2.10", Tag: "proxy-alpha", Address: "192.0.2.10"},
	}

	if err := updateRoutingConfig(path, endpoints, nil, nil); err != nil {
		t.Fatalf("updateRoutingConfig failed: %v", err)
	}

	doc := loadRouting(t, path)
	rules := getRules(t, doc)
	if len(rules) != 1 {
		t.Fatalf("expected 1 routing rule, got %d", len(rules))
	}
	rule := rules[0]
	if got := asStrings(rule["ip"]); len(got) != 1 || got[0] != "192.0.2.10" {
		t.Fatalf("expected ip rule for 192.0.2.10, got %v", got)
	}
	if got := asStrings(rule["domain"]); len(got) != 0 {
		t.Fatalf("did not expect domain field, got %v", got)
	}
}

func loadRouting(t *testing.T, path string) map[string]any {
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

func getRules(t *testing.T, doc map[string]any) []map[string]any {
	t.Helper()
	routing, ok := doc["routing"].(map[string]any)
	if !ok {
		t.Fatalf("routing section missing")
	}
	rawRules, _ := routing["rules"].([]any)
	result := make([]map[string]any, 0, len(rawRules))
	for _, raw := range rawRules {
		entry, ok := raw.(map[string]any)
		if ok {
			result = append(result, entry)
		}
	}
	return result
}

func asStrings(raw any) []string {
	values, _ := raw.([]any)
	result := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
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
