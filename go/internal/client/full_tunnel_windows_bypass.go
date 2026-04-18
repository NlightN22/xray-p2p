//go:build windows

package client

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func buildWindowsBypassRoutes(defaults []winnet.Route, ipv4 []string, ipv6 []string) []fullTunnelRoute {
	var routes []fullTunnelRoute
	for _, def := range defaults {
		switch strings.ToLower(def.AddressFamily) {
		case "ipv4":
			for _, ip := range ipv4 {
				dest := fmt.Sprintf("%s/32", ip)
				routes = append(routes, fullTunnelRoute{
					Family:          "ipv4",
					Destination:     dest,
					NextHop:         def.NextHop,
					InterfaceIndex:  def.InterfaceIndex,
					InterfaceLuid:   def.InterfaceLuid,
					RouteMetric:     def.RouteMetric,
					InterfaceMetric: def.InterfaceMetric,
					PolicyStore:     def.PolicyStore,
				})
			}
		case "ipv6":
			for _, ip := range ipv6 {
				dest := fmt.Sprintf("%s/128", ip)
				routes = append(routes, fullTunnelRoute{
					Family:          "ipv6",
					Destination:     dest,
					NextHop:         def.NextHop,
					InterfaceIndex:  def.InterfaceIndex,
					InterfaceLuid:   def.InterfaceLuid,
					RouteMetric:     def.RouteMetric,
					InterfaceMetric: def.InterfaceMetric,
					PolicyStore:     def.PolicyStore,
				})
			}
		}
	}
	return routes
}

func filterBypassIPv4(endpoints []string, tunIfIndex int, verbose bool) []string {
	var filtered []string
	for _, ip := range endpoints {
		route, prefixLen, ok, err := winnet.BestRouteForIP(ip)
		if err != nil {
			logFullTunnelVerbose(verbose, "full-tunnel bypass route lookup failed", "ip", ip, "err", err)
			filtered = append(filtered, ip)
			continue
		}
		if !ok || tunIfIndex == 0 {
			filtered = append(filtered, ip)
			continue
		}
		if route.InterfaceIndex != tunIfIndex && prefixLen > 0 && !isDefaultRoutePrefix(route.DestinationPrefix) {
			logFullTunnelVerbose(verbose, "full-tunnel bypass skipped (direct route exists)", "ip", ip, "route", route.DestinationPrefix, "ifIndex", route.InterfaceIndex)
			continue
		}
		filtered = append(filtered, ip)
	}
	return filtered
}

func filterBypassIPv6(endpoints []string, tunIfIndex int, verbose bool) []string {
	var filtered []string
	for _, ip := range endpoints {
		route, prefixLen, ok, err := winnet.BestRouteForIP(ip)
		if err != nil {
			logFullTunnelVerbose(verbose, "full-tunnel bypass route lookup failed", "ip", ip, "err", err)
			filtered = append(filtered, ip)
			continue
		}
		if !ok || tunIfIndex == 0 {
			filtered = append(filtered, ip)
			continue
		}
		if route.InterfaceIndex != tunIfIndex && prefixLen > 0 && !isDefaultRoutePrefix(route.DestinationPrefix) {
			logFullTunnelVerbose(verbose, "full-tunnel bypass skipped (direct route exists)", "ip", ip, "route", route.DestinationPrefix, "ifIndex", route.InterfaceIndex)
			continue
		}
		filtered = append(filtered, ip)
	}
	return filtered
}

func isDefaultRoutePrefix(dest string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(dest))
	return trimmed == "0.0.0.0/0" || trimmed == "::/0"
}
