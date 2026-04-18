package client

import (
	"strings"
)

func appendUniqueIPs(ipv4 []string, ipv6 []string, seen4 map[string]struct{}, seen6 map[string]struct{}, target4 *[]string, target6 *[]string) {
	for _, ip := range ipv4 {
		trimmed := normalizeIPKey(ip)
		if trimmed == "" {
			continue
		}
		if _, ok := seen4[trimmed]; ok {
			continue
		}
		seen4[trimmed] = struct{}{}
		*target4 = append(*target4, strings.TrimSpace(ip))
	}
	for _, ip := range ipv6 {
		trimmed := normalizeIPKey(ip)
		if trimmed == "" {
			continue
		}
		if _, ok := seen6[trimmed]; ok {
			continue
		}
		seen6[trimmed] = struct{}{}
		*target6 = append(*target6, strings.TrimSpace(ip))
	}
}

func normalizeEndpointEntry(entry fullTunnelEndpointIPs, cache fullTunnelEndpointIPs, cacheOK bool) fullTunnelEndpointIPs {
	entry.IPv4 = uniqueOrderedIPs(entry.IPv4)
	entry.IPv6 = uniqueOrderedIPs(entry.IPv6)
	if !cacheOK {
		return entry
	}
	entry.IPv4 = preferCachedOrder(entry.IPv4, cache.IPv4)
	entry.IPv6 = preferCachedOrder(entry.IPv6, cache.IPv6)
	return entry
}

func preferCachedOrder(current []string, cached []string) []string {
	if len(current) == 0 || len(cached) == 0 {
		return current
	}
	if !sameIPSet(current, cached) {
		return current
	}
	return filterIPOrder(cached, current)
}

func filterIPOrder(ordered []string, allow []string) []string {
	allowSet := make(map[string]struct{}, len(allow))
	for _, ip := range allow {
		if key := normalizeIPKey(ip); key != "" {
			allowSet[key] = struct{}{}
		}
	}
	out := make([]string, 0, len(allow))
	for _, ip := range ordered {
		key := normalizeIPKey(ip)
		if key == "" {
			continue
		}
		if _, ok := allowSet[key]; !ok {
			continue
		}
		out = append(out, strings.TrimSpace(ip))
	}
	return out
}

func uniqueOrderedIPs(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, ip := range values {
		key := normalizeIPKey(ip)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, strings.TrimSpace(ip))
	}
	return out
}

func sameIPSet(a []string, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, ip := range a {
		key := normalizeIPKey(ip)
		if key == "" {
			continue
		}
		seen[key]++
	}
	for _, ip := range b {
		key := normalizeIPKey(ip)
		if key == "" {
			continue
		}
		seen[key]--
		if seen[key] < 0 {
			return false
		}
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}

func normalizeIPKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
