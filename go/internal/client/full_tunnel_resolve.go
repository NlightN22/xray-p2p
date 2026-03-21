package client

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

const resolverTimeout = 3 * time.Second

func resolveEndpointIPs(ctx context.Context, endpoints []clientEndpointRecord) ([]string, []string, error) {
	seen4 := make(map[string]struct{})
	seen6 := make(map[string]struct{})
	var ips4 []string
	var ips6 []string

	for _, endpoint := range endpoints {
		host := strings.TrimSpace(endpoint.Hostname)
		if host == "" {
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			if ip4 := ip.To4(); ip4 != nil {
				ipv4 := ip4.String()
				if _, ok := seen4[ipv4]; !ok {
					seen4[ipv4] = struct{}{}
					ips4 = append(ips4, ipv4)
				}
			} else {
				ipv6 := ip.String()
				if _, ok := seen6[ipv6]; !ok {
					seen6[ipv6] = struct{}{}
					ips6 = append(ips6, ipv6)
				}
			}
			continue
		}

		resolveCtx, cancel := context.WithTimeout(ctx, resolverTimeout)
		addrs, err := net.DefaultResolver.LookupIPAddr(resolveCtx, host)
		cancel()
		if err != nil {
			return nil, nil, fmt.Errorf("xp2p: resolve endpoint %s: %w", host, err)
		}
		if len(addrs) == 0 {
			return nil, nil, fmt.Errorf("xp2p: resolve endpoint %s: no records", host)
		}
		for _, addr := range addrs {
			if addr.IP == nil {
				continue
			}
			if ip4 := addr.IP.To4(); ip4 != nil {
				ipv4 := ip4.String()
				if _, ok := seen4[ipv4]; !ok {
					seen4[ipv4] = struct{}{}
					ips4 = append(ips4, ipv4)
				}
				continue
			}
			ipv6 := addr.IP.String()
			if _, ok := seen6[ipv6]; !ok {
				seen6[ipv6] = struct{}{}
				ips6 = append(ips6, ipv6)
			}
		}
	}

	return ips4, ips6, nil
}
