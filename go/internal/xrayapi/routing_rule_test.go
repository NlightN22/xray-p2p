package xrayapi

import (
	"testing"

	routerconfig "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/routerconfig"
)

func TestRoutingRuleFromMapConvertsManagedRule(t *testing.T) {
	rule, err := routingRuleFromMap(map[string]any{
		"type":        "field",
		"ruleTag":     "xp2p-test",
		"outboundTag": "proxy-alpha",
		"domains":     []any{"full:app.example"},
		"ip":          []any{"10.0.0.0/24"},
		"port":        "443",
		"inboundTag":  []any{"socks-in"},
	})
	if err != nil {
		t.Fatalf("routingRuleFromMap: %v", err)
	}
	if rule.GetRuleTag() != "xp2p-test" || rule.GetTag() != "proxy-alpha" {
		t.Fatalf("unexpected tags: %+v", rule)
	}
	if len(rule.GetDomain()) != 1 || rule.GetDomain()[0].GetType() != routerconfig.Domain_Full || rule.GetDomain()[0].GetValue() != "app.example" {
		t.Fatalf("unexpected domains: %+v", rule.GetDomain())
	}
	if len(rule.GetGeoip()) != 1 || len(rule.GetGeoip()[0].GetCidr()) != 1 || rule.GetGeoip()[0].GetCidr()[0].GetPrefix() != 24 {
		t.Fatalf("unexpected geoip: %+v", rule.GetGeoip())
	}
	if rule.GetPortList() == nil || len(rule.GetPortList().GetRange()) != 1 || rule.GetPortList().GetRange()[0].GetFrom() != 443 {
		t.Fatalf("unexpected port list: %+v", rule.GetPortList())
	}
	if len(rule.GetInboundTag()) != 1 || rule.GetInboundTag()[0] != "socks-in" {
		t.Fatalf("unexpected inbound tags: %+v", rule.GetInboundTag())
	}
}

func TestRoutingRuleFromMapRejectsUnsupportedRule(t *testing.T) {
	_, err := routingRuleFromMap(map[string]any{
		"type":          "field",
		"ruleTag":       "xp2p-test",
		"outboundTag":   "proxy-alpha",
		"balancerTag":   "balancer",
		"domainMatcher": "mph",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
