//go:build windows || linux

package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func TestCompileIncludesReverseChannels(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)

	doc := map[string]any{
		serverReverseStateKey: map[string]serverReverseChannel{
			"alphaedge-example.rev": {
				UserID: "alpha",
				Host:   "edge.example",
				Tag:    "alphaedge-example.rev",
				Domain: "alphaedge-example.rev",
			},
		},
	}
	if err := writeServerStateDoc(pendingConfigPath(), doc); err != nil {
		t.Fatalf("write reverse state: %v", err)
	}

	compiled := compileDesiredDoc(t, pendingConfigPath(), configDir)
	reverse, ok := compiled["reverse"].(map[string]any)
	if !ok {
		t.Fatalf("expected reverse section in xray.json")
	}
	portals, ok := reverse["portals"].([]any)
	if !ok || len(portals) != 1 {
		t.Fatalf("expected 1 portal, got %v", reverse["portals"])
	}

	rules := extractRoutingRules(t, compiled)
	if !hasRuleWithDomainAndOutbound(rules, "full:alphaedge-example.rev", "alphaedge-example.rev") {
		t.Fatalf("expected reverse routing rule, got %v", rules)
	}
}

func TestCompileOrdersReverseChannelsByTag(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XP2P_CONFIG_ROOT", dir)

	if err := os.WriteFile(config.ConfigPath(layout.ServerConfigFileName), []byte("[server]\n"), 0o644); err != nil {
		t.Fatalf("write server config: %v", err)
	}
	configDir := filepath.Join(dir, layout.ServerConfigDir)
	doc := map[string]any{
		serverReverseStateKey: map[string]serverReverseChannel{
			"bravo.rev": {UserID: "bravo", Host: "edge.example", Tag: "bravo.rev", Domain: "bravo.rev"},
			"alpha.rev": {UserID: "alpha", Host: "edge.example", Tag: "alpha.rev", Domain: "alpha.rev"},
		},
	}
	if err := writeServerStateDoc(pendingConfigPath(), doc); err != nil {
		t.Fatalf("write reverse state: %v", err)
	}

	compiled := compileDesiredDoc(t, pendingConfigPath(), configDir)
	reverse, ok := compiled["reverse"].(map[string]any)
	if !ok {
		t.Fatalf("expected reverse section in xray.json")
	}
	portals, ok := reverse["portals"].([]any)
	if !ok || len(portals) != 2 {
		t.Fatalf("expected 2 portals, got %v", reverse["portals"])
	}
	if got := portals[0].(map[string]any)["tag"]; got != "alpha.rev" {
		t.Fatalf("first portal tag = %v, want alpha.rev", got)
	}
	if got := portals[1].(map[string]any)["tag"]; got != "bravo.rev" {
		t.Fatalf("second portal tag = %v, want bravo.rev", got)
	}
}

func hasRuleWithDomainAndOutbound(rules []any, domain string, outboundTag string) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := rule["outboundTag"].(string)
		if tag != outboundTag {
			continue
		}
		for _, value := range extractStringSlice(rule["domain"]) {
			if value == domain {
				return true
			}
		}
	}
	return false
}
