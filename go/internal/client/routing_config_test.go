package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func TestWriteOutboundsConfigIncludesEndpointsAndFreedom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbounds.json")
	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example", Port: 8443, User: "alpha", Password: "secret", ServerName: "alpha.example"},
		{Hostname: "beta.example", Tag: "proxy-beta", Address: "beta.example", Port: 9443, User: "beta", Password: "other", ServerName: "beta.example"},
	}

	if err := writeOutboundsConfig(path, xrayconfig.DefaultClientConfig().DirectOutbound, endpoints, nil, false); err != nil {
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
	expected := 3
	if runtime.GOOS == "windows" {
		expected = 4
	}
	if len(doc.Outbounds) != expected {
		t.Fatalf("expected %d outbounds, got %d", expected, len(doc.Outbounds))
	}
	if doc.Outbounds[0]["tag"] != "proxy-alpha" || doc.Outbounds[1]["tag"] != "proxy-beta" {
		t.Fatalf("unexpected tags: %+v", doc.Outbounds)
	}
	if runtime.GOOS == "windows" {
		if doc.Outbounds[2]["tag"] != "direct-random" || doc.Outbounds[3]["tag"] != "direct-udp" {
			t.Fatalf("unexpected direct outbounds: %+v", doc.Outbounds[2:])
		}
	} else if doc.Outbounds[2]["tag"] != "direct" {
		t.Fatalf("expected last outbound to be direct, got %+v", doc.Outbounds[2])
	}
}

func TestWriteOutboundsConfigPreservesUserOutbounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outbounds.json")
	directTag := "direct"
	if runtime.GOOS == "windows" {
		directTag = "direct-random"
	}
	existing := map[string]any{
		"outbounds": []any{
			map[string]any{
				"tag":      "user-proxy",
				"protocol": "socks",
			},
			map[string]any{
				"tag":         directTag,
				"protocol":    "freedom",
				"sendThrough": "192.0.2.50",
			},
		},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal existing: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	endpoints := []clientEndpointRecord{
		{Hostname: "alpha.example", Tag: "proxy-alpha", Address: "alpha.example", Port: 8443, User: "alpha", Password: "secret", ServerName: "alpha.example"},
	}
	cfg := xrayconfig.DefaultClientConfig().DirectOutbound
	cfg.SendThrough = "10.0.0.5"
	if err := writeOutboundsConfig(path, cfg, endpoints, nil, false); err != nil {
		t.Fatalf("writeOutboundsConfig failed: %v", err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read outbounds: %v", err)
	}
	var doc struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal(updated, &doc); err != nil {
		t.Fatalf("parse outbounds: %v", err)
	}

	var hasUser, hasDirect, hasProxy bool
	var directSendThrough string
	for _, outbound := range doc.Outbounds {
		tag, _ := outbound["tag"].(string)
		switch tag {
		case "user-proxy":
			hasUser = true
		case directTag:
			hasDirect = true
			directSendThrough, _ = outbound["sendThrough"].(string)
		case "proxy-alpha":
			hasProxy = true
		case "direct-udp":
			if runtime.GOOS == "windows" {
				hasDirect = true
			}
		}
	}
	if !hasUser || !hasProxy || !hasDirect {
		t.Fatalf("expected user, proxy, and direct outbounds, got %+v", doc.Outbounds)
	}
	if runtime.GOOS != "windows" && directSendThrough != "10.0.0.5" {
		t.Fatalf("expected sendThrough to be updated, got %q", directSendThrough)
	}
	if runtime.GOOS == "windows" && directSendThrough != "" {
		t.Fatalf("expected direct-random to omit sendThrough, got %q", directSendThrough)
	}
	if tag, _ := doc.Outbounds[0]["tag"].(string); tag != "user-proxy" {
		t.Fatalf("expected user outbound to stay first, got %+v", doc.Outbounds[0])
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
	rule := findRuleWithDomain(rules, "full:alpha.example")
	if rule == nil {
		t.Fatalf("expected domain rule for alpha.example, got %+v", rules)
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
