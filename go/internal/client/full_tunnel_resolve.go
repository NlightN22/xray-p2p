package client

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	resolverTimeout  = 3 * time.Second
	endpointCacheTTL = 10 * time.Minute
)

var lookupIPAddrs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

func resolveEndpointPrimaryAddress(ctx context.Context, host string) (string, error) {
	if net.ParseIP(host) != nil {
		return host, nil
	}
	resolveCtx := ctx
	if resolveCtx == nil {
		resolveCtx = context.Background()
	}
	resolveCtx, cancel := context.WithTimeout(resolveCtx, resolverTimeout)
	addrs, err := lookupIPAddrs(resolveCtx, host)
	cancel()
	if err != nil {
		return "", fmt.Errorf("resolve endpoint %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return "", fmt.Errorf("resolve endpoint %s: no records", host)
	}
	for _, addr := range addrs {
		if addr.IP == nil {
			continue
		}
		if ip4 := addr.IP.To4(); ip4 != nil {
			return ip4.String(), nil
		}
	}
	for _, addr := range addrs {
		if addr.IP == nil {
			continue
		}
		return addr.IP.String(), nil
	}
	return "", fmt.Errorf("resolve endpoint %s: no records", host)
}

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
				entry.IPv4 = []string{ip4.String()}
			} else {
				entry.IPv6 = []string{ip.String()}
			}
			entry = normalizeEndpointEntry(entry, cacheEntry, cacheOK)
			appendUniqueIPs(entry.IPv4, entry.IPv6, seen4, seen6, &ips4, &ips6)
			resolved[key] = entry
			continue
		}

		resolveCtx, cancel := context.WithTimeout(ctx, resolverTimeout)
		addrs, err := lookupIPAddrs(resolveCtx, host)
		cancel()
		if err != nil || len(addrs) == 0 {
			if cacheOK && (len(cacheEntry.IPv4) > 0 || len(cacheEntry.IPv6) > 0) {
				resolved[key] = cacheEntry
				appendUniqueIPs(cacheEntry.IPv4, cacheEntry.IPv6, seen4, seen6, &ips4, &ips6)
				continue
			}
			if err != nil {
				return nil, nil, nil, fmt.Errorf("resolve endpoint %s: %w", host, err)
			}
			return nil, nil, nil, fmt.Errorf("resolve endpoint %s: no records", host)
		}

		entry := fullTunnelEndpointIPs{ResolvedAt: now}
		for _, addr := range addrs {
			if addr.IP == nil {
				continue
			}
			if ip4 := addr.IP.To4(); ip4 != nil {
				ipv4 := ip4.String()
				entry.IPv4 = append(entry.IPv4, ipv4)
				continue
			}
			ipv6 := addr.IP.String()
			entry.IPv6 = append(entry.IPv6, ipv6)
		}
		entry = normalizeEndpointEntry(entry, cacheEntry, cacheOK)
		appendUniqueIPs(entry.IPv4, entry.IPv6, seen4, seen6, &ips4, &ips6)
		resolved[key] = entry
	}

	return ips4, ips6, resolved, nil
}

func resolveEndpointIPMap(ctx context.Context, endpoints []clientEndpointRecord, cache map[string]fullTunnelEndpointIPs) (map[string]fullTunnelEndpointIPs, error) {
	_, _, resolved, err := resolveEndpointIPs(ctx, endpoints, cache)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func resolveEndpointIPMapWithCache(ctx context.Context, endpoints []clientEndpointRecord) (map[string]fullTunnelEndpointIPs, error) {
	cache, err := loadFullTunnelEndpointCache()
	if err != nil {
		return nil, err
	}
	if cacheHasEndpoints(endpoints, cache) && cacheFresh(cache) {
		return cache, nil
	}
	resolved, err := resolveEndpointIPMap(ctx, endpoints, cache)
	if err != nil && cacheHasEndpoints(endpoints, cache) {
		return cache, nil
	}
	return resolved, err
}

func cacheHasEndpoints(endpoints []clientEndpointRecord, cache map[string]fullTunnelEndpointIPs) bool {
	if len(endpoints) == 0 || len(cache) == 0 {
		return false
	}
	for _, endpoint := range endpoints {
		host := endpointHost(endpoint)
		if host == "" {
			continue
		}
		entry, ok := cache[strings.ToLower(host)]
		if !ok {
			return false
		}
		if len(entry.IPv4) == 0 && len(entry.IPv6) == 0 {
			return false
		}
	}
	return true
}

func cacheFresh(cache map[string]fullTunnelEndpointIPs) bool {
	if len(cache) == 0 {
		return false
	}
	now := time.Now().UTC()
	for _, entry := range cache {
		if entry.ResolvedAt.IsZero() {
			return false
		}
		if now.Sub(entry.ResolvedAt) > endpointCacheTTL {
			return false
		}
	}
	return true
}

func endpointCacheNeedsUpdate(existing map[string]fullTunnelEndpointIPs, resolved map[string]fullTunnelEndpointIPs) bool {
	if len(resolved) == 0 {
		return false
	}
	if len(existing) == 0 {
		return true
	}
	for key, entry := range resolved {
		cached, ok := existing[key]
		if !ok {
			return true
		}
		if !sameIPSet(entry.IPv4, cached.IPv4) || !sameIPSet(entry.IPv6, cached.IPv6) {
			return true
		}
	}
	return false
}
