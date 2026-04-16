package client

import (
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestApplyClientEndpointConfigAddsReverseRules(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)
	extensionsDir := filepath.Join(dir, layout.ClientConfigDir)
	configFile := filepath.Clean(config.ConfigPath(layout.ClientConfigFileName))

	endpoint := endpointConfig{
		Hostname:   "server.example",
		Port:       8443,
		User:       "reverse.user",
		Password:   "secret",
		ServerName: "server.example",
	}
	if _, err := applyClientEndpointConfig("", configFile, endpoint, true); err != nil {
		t.Fatalf("applyClientEndpointConfig: %v", err)
	}

	doc := compileDesiredDoc(t, configFile, extensionsDir)
	reverse := doc["reverse"].(map[string]any)
	bridges := reverse["bridges"].([]any)
	if len(bridges) != 1 {
		t.Fatalf("expected 1 reverse bridge, got %d", len(bridges))
	}
	entry := bridges[0].(map[string]any)
	if entry["tag"] != "reverse-userserver-example.rev" || entry["domain"] != "reverse-userserver-example.rev" {
		t.Fatalf("unexpected bridge entry: %+v", entry)
	}

	rules := extractRoutingRules(t, doc)
	domainRule := findRuleWithDomainRule(rules, "full:reverse-userserver-example.rev")
	if domainRule == nil {
		t.Fatalf("expected reverse domain rule, got %+v", rules)
	}
	if domainRule["outboundTag"] != "proxy-server-example" {
		t.Fatalf("unexpected outbound tag: %+v", domainRule)
	}
	expectedDirect := "direct"
	directRule := findRuleWithInboundAndOutbound(rules, "reverse-userserver-example.rev", expectedDirect)
	if directRule == nil {
		t.Fatalf("expected reverse direct rule, got %+v", rules)
	}
	if directRule["outboundTag"] != expectedDirect {
		t.Fatalf("expected %s outbound, got %+v", expectedDirect, directRule)
	}

	state, err := loadClientInstallState(configFile)
	if err != nil {
		t.Fatalf("load client state: %v", err)
	}
	if _, ok := state.Reverse["reverse-userserver-example.rev"]; !ok {
		t.Fatalf("expected reverse entry in state, got %v", state.Reverse)
	}
}

func findRuleWithDomainRule(rules []any, domain string) map[string]any {
	for _, raw := range rules {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		domains, ok := entry["domain"].([]any)
		if !ok {
			continue
		}
		for _, item := range domains {
			if value, ok := item.(string); ok && value == domain {
				return entry
			}
		}
	}
	return nil
}

func findRuleWithInboundAndOutbound(rules []any, inboundTag string, outboundTag string) map[string]any {
	for _, raw := range rules {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if outboundTag != "" {
			if tag, _ := entry["outboundTag"].(string); tag != outboundTag {
				continue
			}
		}
		inbound, ok := entry["inboundTag"].([]any)
		if !ok {
			continue
		}
		for _, item := range inbound {
			if value, ok := item.(string); ok && value == inboundTag {
				return entry
			}
		}
	}
	return nil
}
