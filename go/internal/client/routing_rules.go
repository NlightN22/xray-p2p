package client

import (
	"net"
	"strings"
)

type routingRuleClass string

const (
	routingRuleEndpointBypass routingRuleClass = "endpoint-bypass"
	routingRuleSystem         routingRuleClass = "system"
	routingRuleRedirect       routingRuleClass = "redirect"
	routingRuleUser           routingRuleClass = "user"
	routingRuleFullTunnel     routingRuleClass = "full-tunnel"
)

func endpointBypassRule(ep clientEndpointRecord) map[string]any {
	rule := map[string]any{
		"type":        "field",
		"outboundTag": directRandomTag(),
	}
	if net.ParseIP(ep.Address) != nil {
		rule["ip"] = []string{ep.Address}
	} else {
		rule["domain"] = []string{"full:" + ep.Address}
	}
	return rule
}

func endpointMatchKey(address string) (string, bool) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", false
	}
	if net.ParseIP(address) != nil {
		return "ip:" + address, true
	}
	return "domain:full:" + strings.ToLower(address), true
}

func fullTunnelRule(tag string) map[string]any {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" {
		return nil
	}
	return map[string]any{
		"type":        "field",
		"ip":          []string{"0.0.0.0/0", "::/0"},
		"outboundTag": trimmed,
	}
}
