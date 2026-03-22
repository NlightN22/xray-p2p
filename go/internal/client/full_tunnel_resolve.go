package client

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const resolverTimeout = 3 * time.Second

func resolveEndpointIPs(ctx context.Context, endpoints []clientEndpointRecord, cache map[string]fullTunnelEndpointIPs) ([]string, []string, map[string]fullTunnelEndpointIPs, error) {
	seen4 := make(map[string]struct{})
	seen6 := make(map[string]struct{})
	var ips4 []string
	var ips6 []string
	resolved := make(map[string]fullTunnelEndpointIPs, len(endpoints))
	now := time.Now().UTC()

	for _, endpoint := range endpoints {
		host := endpointHost(endpoint)
		if host == "" {
			continue
		}
		key := strings.ToLower(host)
		cacheEntry, cacheOK := cache[key]
		if ip := net.ParseIP(host); ip != nil {
			entry := fullTunnelEndpointIPs{ResolvedAt: now}
			if ip4 := ip.To4(); ip4 != nil {
				ipv4 := ip4.String()
				entry.IPv4 = []string{ipv4}
				if _, ok := seen4[ipv4]; !ok {
					seen4[ipv4] = struct{}{}
					ips4 = append(ips4, ipv4)
				}
			} else {
				ipv6 := ip.String()
				entry.IPv6 = []string{ipv6}
				if _, ok := seen6[ipv6]; !ok {
					seen6[ipv6] = struct{}{}
					ips6 = append(ips6, ipv6)
				}
			}
			resolved[key] = entry
			continue
		}

		resolveCtx, cancel := context.WithTimeout(ctx, resolverTimeout)
		addrs, err := net.DefaultResolver.LookupIPAddr(resolveCtx, host)
		cancel()
		if err != nil || len(addrs) == 0 {
			if cacheOK && (len(cacheEntry.IPv4) > 0 || len(cacheEntry.IPv6) > 0) {
				resolved[key] = cacheEntry
				appendUniqueIPs(cacheEntry.IPv4, cacheEntry.IPv6, seen4, seen6, &ips4, &ips6)
				continue
			}
			if err != nil {
				return nil, nil, nil, fmt.Errorf("xp2p: resolve endpoint %s: %w", host, err)
			}
			return nil, nil, nil, fmt.Errorf("xp2p: resolve endpoint %s: no records", host)
		}

		entry := fullTunnelEndpointIPs{ResolvedAt: now}
		for _, addr := range addrs {
			if addr.IP == nil {
				continue
			}
			if ip4 := addr.IP.To4(); ip4 != nil {
				ipv4 := ip4.String()
				entry.IPv4 = append(entry.IPv4, ipv4)
				if _, ok := seen4[ipv4]; !ok {
					seen4[ipv4] = struct{}{}
					ips4 = append(ips4, ipv4)
				}
				continue
			}
			ipv6 := addr.IP.String()
			entry.IPv6 = append(entry.IPv6, ipv6)
			if _, ok := seen6[ipv6]; !ok {
				seen6[ipv6] = struct{}{}
				ips6 = append(ips6, ipv6)
			}
		}
		resolved[key] = entry
	}

	return ips4, ips6, resolved, nil
}

func appendUniqueIPs(ipv4 []string, ipv6 []string, seen4 map[string]struct{}, seen6 map[string]struct{}, target4 *[]string, target6 *[]string) {
	for _, ip := range ipv4 {
		trimmed := strings.TrimSpace(ip)
		if trimmed == "" {
			continue
		}
		if _, ok := seen4[trimmed]; ok {
			continue
		}
		seen4[trimmed] = struct{}{}
		*target4 = append(*target4, trimmed)
	}
	for _, ip := range ipv6 {
		trimmed := strings.TrimSpace(ip)
		if trimmed == "" {
			continue
		}
		if _, ok := seen6[trimmed]; ok {
			continue
		}
		seen6[trimmed] = struct{}{}
		*target6 = append(*target6, trimmed)
	}
}
