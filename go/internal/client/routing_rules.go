package client

import (
	"fmt"
	"net"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/xrayrule"
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
		"ruleTag":     xrayrule.EndpointBypass("client", ep.Tag, ep.Address),
		"outboundTag": directRandomTag(),
	}
	if net.ParseIP(ep.Address) != nil {
		rule["ip"] = []string{ep.Address}
	} else {
		rule["domain"] = []string{"full:" + ep.Address}
	}
	return rule
}

func endpointBypassRules(endpoints []clientEndpointRecord, endpointIPs map[string]fullTunnelEndpointIPs, requireIPs bool) ([]any, error) {
	if len(endpoints) == 0 {
		return []any{}, nil
	}
	rules := make([]any, 0, len(endpoints))
	for _, endpoint := range endpoints {
		host := endpointHost(endpoint)
		if host == "" {
			return nil, fmt.Errorf("endpoint host is required for full-tunnel routing")
		}
		entry := endpointIPs[strings.ToLower(host)]
		ips := uniqueEndpointIPs(entry)
		if len(ips) == 0 {
			if requireIPs {
				return nil, fmt.Errorf("endpoint %s has no resolved IPs", host)
			}
			rules = append(rules, endpointBypassRule(endpoint))
			continue
		}
		rules = append(rules, map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.EndpointBypass("client", endpoint.Tag, strings.Join(ips, ",")),
			"ip":          ips,
			"outboundTag": directRandomTag(),
		})
	}
	return rules, nil
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
		"ruleTag":     xrayrule.FullTunnel("client", trimmed),
		"ip":          []string{"0.0.0.0/0", "::/0"},
		"outboundTag": trimmed,
	}
}

func uniqueEndpointIPs(entry fullTunnelEndpointIPs) []string {
	seen := make(map[string]struct{})
	ips := make([]string, 0, len(entry.IPv4)+len(entry.IPv6))
	for _, ip := range append(entry.IPv4, entry.IPv6...) {
		trimmed := strings.TrimSpace(ip)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		ips = append(ips, trimmed)
	}
	return ips
}
