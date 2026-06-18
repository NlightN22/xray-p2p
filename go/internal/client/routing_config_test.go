package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func TestUpdateRoutingConfigManagesReverseRules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example"},
	}
	reverse := map[string]clientReverseChannel{
		"alphaalpha-example.rev": {UserID: "alpha", Host: "alpha.example", Tag: "alphaalpha-example.rev", Domain: "alphaalpha-example.rev", EndpointTag: "proxy-alpha"},
	}

	if err := updateRoutingConfig(path, xrayconfig.DefaultClientConfig().Routing, endpoints, nil, reverse, false, "", nil, false); err != nil {
		t.Fatalf("updateRoutingConfig failed: %v", err)
	}

	verifyRoutingDocument(t, path, 4, 1)

	// Second update should not duplicate rules/bridges.
	if err := updateRoutingConfig(path, xrayconfig.DefaultClientConfig().Routing, endpoints, nil, reverse, false, "", nil, false); err != nil {
		t.Fatalf("second updateRoutingConfig failed: %v", err)
	}
	verifyRoutingDocument(t, path, 4, 1)
}

func TestUpdateRoutingConfigUsesDomainRuleForHostname(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example"},
	}

	if err := updateRoutingConfig(path, xrayconfig.DefaultClientConfig().Routing, endpoints, nil, nil, false, "", nil, false); err != nil {
		t.Fatalf("updateRoutingConfig failed: %v", err)
	}

	doc := loadRouting(t, path)
	rules := getRules(t, doc)
	if len(rules) != 2+windowsRuleBonus() {
		t.Fatalf("expected %d routing rules, got %d", 2+windowsRuleBonus(), len(rules))
	}

	markerIP, err := markerIPForIndex(0)
	if err != nil {
		t.Fatalf("markerIPForIndex failed: %v", err)
	}
	markerCIDR := markerIP + "/32"
	markerIPRule := findRuleWithIP(rules, markerCIDR)
	if markerIPRule == nil {
		t.Fatalf("expected marker ip rule, got %+v", rules)
	}
	if got := fmt.Sprintf("%v", markerIPRule["port"]); got != fmt.Sprintf("%d", DiagnosticsMarkerPort) {
		t.Fatalf("expected marker port %d, got %v", DiagnosticsMarkerPort, got)
	}
	if got, _ := markerIPRule["ruleTag"].(string); got == "" {
		t.Fatalf("expected marker ruleTag, got %+v", markerIPRule)
	}
	rule := findRuleWithDomain(rules, "full:alpha.example")
	if rule == nil {
		t.Fatalf("expected domain rule for alpha.example, got %+v", rules)
	}
	if got, _ := rule["ruleTag"].(string); got == "" {
		t.Fatalf("expected endpoint bypass ruleTag, got %+v", rule)
	}
	if got := asStrings(rule["domain"]); len(got) != 1 || got[0] != "full:alpha.example" {
		t.Fatalf("expected domain rule for alpha.example, got %v", got)
	}
	if got := asStrings(rule["ip"]); len(got) != 0 {
		t.Fatalf("did not expect ip field, got %v", got)
	}
}

func TestUpdateRoutingConfigUsesResolvedEndpointIPsWhenFullEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example"},
	}
	endpointIPs := map[string]fullTunnelEndpointIPs{
		"alpha.example": {IPv4: []string{"203.0.113.10"}},
	}

	if err := updateRoutingConfig(path, xrayconfig.DefaultClientConfig().Routing, endpoints, nil, nil, true, "", endpointIPs, true); err != nil {
		t.Fatalf("updateRoutingConfig failed: %v", err)
	}

	doc := loadRouting(t, path)
	rules := getRules(t, doc)
	if len(rules) != 2+windowsRuleBonus() {
		t.Fatalf("expected %d routing rules, got %d", 2+windowsRuleBonus(), len(rules))
	}

	rule := findRuleWithIP(rules, "203.0.113.10")
	if rule == nil {
		t.Fatalf("expected ip rule for 203.0.113.10, got %+v", rules)
	}
	if got := asStrings(rule["domain"]); len(got) != 0 {
		t.Fatalf("did not expect domain field, got %v", got)
	}
}

func TestUpdateRoutingConfigUsesIPRuleForAddress(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "192.0.2.10", Tag: "proxy-alpha", Address: "192.0.2.10"},
	}

	if err := updateRoutingConfig(path, xrayconfig.DefaultClientConfig().Routing, endpoints, nil, nil, false, "", nil, false); err != nil {
		t.Fatalf("updateRoutingConfig failed: %v", err)
	}

	doc := loadRouting(t, path)
	rules := getRules(t, doc)
	if len(rules) != 2+windowsRuleBonus() {
		t.Fatalf("expected %d routing rules, got %d", 2+windowsRuleBonus(), len(rules))
	}

	markerIP, err := markerIPForIndex(0)
	if err != nil {
		t.Fatalf("markerIPForIndex failed: %v", err)
	}
	markerCIDR := markerIP + "/32"
	markerIPRule := findRuleWithIP(rules, markerCIDR)
	if markerIPRule == nil {
		t.Fatalf("expected marker ip rule, got %+v", rules)
	}
	if got := fmt.Sprintf("%v", markerIPRule["port"]); got != fmt.Sprintf("%d", DiagnosticsMarkerPort) {
		t.Fatalf("expected marker port %d, got %v", DiagnosticsMarkerPort, got)
	}
	rule := findRuleWithIP(rules, "192.0.2.10")
	if rule == nil {
		t.Fatalf("expected ip rule for 192.0.2.10, got %+v", rules)
	}
	if got := asStrings(rule["ip"]); len(got) != 1 || got[0] != "192.0.2.10" {
		t.Fatalf("expected ip rule for 192.0.2.10, got %v", got)
	}
	if got := asStrings(rule["domain"]); len(got) != 0 {
		t.Fatalf("did not expect domain field, got %v", got)
	}
}

func TestUpdateRoutingConfigAppendsFullTunnelRuleLast(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routing.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example"},
	}
	routingCfg := xrayconfig.DefaultClientConfig().Routing
	routingCfg.Rules = []map[string]any{
		{
			"type":        "field",
			"ip":          []string{"10.0.0.0/8"},
			"outboundTag": "direct",
		},
	}

	if err := updateRoutingConfig(path, routingCfg, endpoints, nil, nil, true, "proxy-alpha", nil, false); err != nil {
		t.Fatalf("updateRoutingConfig failed: %v", err)
	}

	doc := loadRouting(t, path)
	rules := getRules(t, doc)
	if len(rules) == 0 {
		t.Fatalf("expected routing rules, got none")
	}
	last := rules[len(rules)-1]
	if last["outboundTag"] != "proxy-alpha" {
		t.Fatalf("expected full-tunnel rule to use proxy-alpha, got %+v", last)
	}
	if got, _ := last["ruleTag"].(string); got == "" {
		t.Fatalf("expected full-tunnel ruleTag, got %+v", last)
	}
	ipValues := asStrings(last["ip"])
	if len(ipValues) != 2 || ipValues[0] != "0.0.0.0/0" || ipValues[1] != "::/0" {
		t.Fatalf("expected full-tunnel rule to include 0.0.0.0/0 and ::/0, got %v", ipValues)
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

func findRuleWithDomain(rules []map[string]any, domain string) map[string]any {
	for _, rule := range rules {
		for _, value := range asStrings(rule["domain"]) {
			if value == domain {
				return rule
			}
		}
	}
	return nil
}

func findRuleWithIP(rules []map[string]any, ip string) map[string]any {
	for _, rule := range rules {
		for _, value := range asStrings(rule["ip"]) {
			if value == ip {
				return rule
			}
		}
	}
	return nil
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
	expected := wantRules + windowsRuleBonus()
	if len(rules) != expected {
		t.Fatalf("expected %d routing rules, got %d", expected, len(rules))
	}
	reverseObj, _ := doc["reverse"].(map[string]any)
	bridges, _ := reverseObj["bridges"].([]any)
	if len(bridges) != wantBridges {
		t.Fatalf("expected %d bridges, got %d", wantBridges, len(bridges))
	}
}
