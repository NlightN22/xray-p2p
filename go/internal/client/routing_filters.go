package client

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func managedOutboundTags(endpoints []clientEndpointRecord, redirects []redirect.Rule) map[string]struct{} {
	total := len(endpoints) + len(redirects)
	if total == 0 {
		return map[string]struct{}{}
	}
	known := make(map[string]struct{}, total)
	for _, ep := range endpoints {
		if tag := strings.TrimSpace(ep.Tag); tag != "" {
			known[strings.ToLower(tag)] = struct{}{}
		}
	}
	for _, rule := range redirects {
		if tag := strings.TrimSpace(rule.OutboundTag); tag != "" {
			known[strings.ToLower(tag)] = struct{}{}
		}
	}
	return known
}

func filterManagedRules(rules []any, managed map[string]struct{}) []any {
	if len(rules) == 0 {
		return []any{}
	}
	if len(managed) == 0 {
		return rules
	}

	result := make([]any, 0, len(rules))
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			result = append(result, rule)
			continue
		}
		outbound, _ := ruleMap["outboundTag"].(string)
		if _, managed := managed[strings.ToLower(outbound)]; managed {
			continue
		}
		result = append(result, ruleMap)
	}
	return result
}

func filterEndpointBypassRules(rules []any, endpoints []clientEndpointRecord, endpointIPs map[string]fullTunnelEndpointIPs, requireIPs bool) ([]any, error) {
	if len(rules) == 0 || len(endpoints) == 0 {
		return rules, nil
	}
	known := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		host := endpointHost(ep)
		if host == "" {
			return nil, fmt.Errorf("endpoint host is required for full-tunnel routing")
		}
		entry := endpointIPs[strings.ToLower(host)]
		ips := uniqueEndpointIPs(entry)
		if len(ips) == 0 {
			if requireIPs {
				return nil, fmt.Errorf("endpoint %s has no resolved IPs", host)
			}
			key, ok := endpointMatchKey(ep.Address)
			if ok {
				known[key] = struct{}{}
			}
			continue
		}
		for _, ip := range ips {
			known["ip:"+strings.TrimSpace(ip)] = struct{}{}
		}
	}
	if len(known) == 0 {
		return rules, nil
	}
	filtered := make([]any, 0, len(rules))
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			filtered = append(filtered, rule)
			continue
		}
		if isEndpointBypassRule(ruleMap, known) {
			continue
		}
		filtered = append(filtered, ruleMap)
	}
	return filtered, nil
}

func isEndpointBypassRule(rule map[string]any, known map[string]struct{}) bool {
	typ, _ := rule["type"].(string)
	if !strings.EqualFold(strings.TrimSpace(typ), "field") {
		return false
	}
	outbound, _ := rule["outboundTag"].(string)
	if !strings.EqualFold(strings.TrimSpace(outbound), directRandomTag()) {
		return false
	}
	for _, ip := range extractStringSlice(rule["ip"]) {
		if _, ok := known["ip:"+strings.TrimSpace(ip)]; ok {
			return true
		}
	}
	for _, domain := range extractStringSlice(rule["domain"]) {
		normalized := strings.ToLower(strings.TrimSpace(domain))
		if normalized == "" {
			continue
		}
		if !strings.HasPrefix(normalized, "full:") {
			normalized = "full:" + normalized
		}
		if _, ok := known["domain:"+normalized]; ok {
			return true
		}
	}
	return false
}

func filterReverseRules(rules []any, reverse map[string]clientReverseChannel) []any {
	if len(rules) == 0 || len(reverse) == 0 {
		return rules
	}
	known := make(map[string]struct{}, len(reverse))
	for _, channel := range reverse {
		known[strings.ToLower(strings.TrimSpace(channel.Tag))] = struct{}{}
	}
	filtered := make([]any, 0, len(rules))
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			filtered = append(filtered, rule)
			continue
		}
		inbound := extractStringSlice(ruleMap["inboundTag"])
		remove := false
		for _, tag := range inbound {
			if _, ok := known[strings.ToLower(strings.TrimSpace(tag))]; ok {
				remove = true
				break
			}
		}
		if remove {
			continue
		}
		filtered = append(filtered, ruleMap)
	}
	return filtered
}
