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
