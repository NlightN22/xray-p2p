//go:build windows

package client

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
					Family:         "ipv4",
					Destination:    dest,
					NextHop:        def.NextHop,
					InterfaceIndex: def.InterfaceIndex,
					RouteMetric:    def.RouteMetric,
					PolicyStore:    def.PolicyStore,
				})
			}
		case "ipv6":
			for _, ip := range ipv6 {
				dest := fmt.Sprintf("%s/128", ip)
				routes = append(routes, fullTunnelRoute{
					Family:         "ipv6",
					Destination:    dest,
					NextHop:        def.NextHop,
					InterfaceIndex: def.InterfaceIndex,
					RouteMetric:    def.RouteMetric,
					PolicyStore:    def.PolicyStore,
				})
			}
		}
	}
	return routes
}

func syncWindowsBypassRoutes(ctx context.Context, desired []fullTunnelRoute, existing []fullTunnelRoute) error {
	desiredSet := make(map[string]struct{}, len(desired))
	for _, route := range desired {
		key := windowsRouteKey(route)
		desiredSet[key] = struct{}{}
	}
	for _, route := range existing {
		if route.Destination == "" || route.InterfaceIndex == 0 {
			continue
		}
		if _, ok := desiredSet[windowsRouteKey(route)]; ok {
			continue
		}
		if err := winnet.RemoveRoute(ctx, toWindowsRoute(route)); err != nil {
			return err
		}
	}
	for _, route := range desired {
		if err := winnet.ApplyRoute(ctx, toWindowsRoute(route)); err != nil {
			return err
		}
	}
	return nil
}

func removeWindowsBypassRoutes(ctx context.Context, routes []fullTunnelRoute) error {
	for _, route := range routes {
		if route.Destination == "" || route.InterfaceIndex == 0 {
			continue
		}
		if err := winnet.RemoveRoute(ctx, toWindowsRoute(route)); err != nil {
			return err
		}
	}
	return nil
}

func ensureWindowsDefaultRoute(ctx context.Context, tunName string, family string) error {
	if strings.TrimSpace(tunName) == "" {
		return errors.New("xp2p: tun name is required for full-tunnel default route")
	}
	ifIndex, err := winnet.InterfaceIndexByName(ctx, tunName)
	if err != nil {
		return err
	}
	dest := "0.0.0.0/0"
	nextHop := "0.0.0.0"
	if strings.EqualFold(family, "IPv6") {
		dest = "::/0"
		nextHop = "::"
	}
	return winnet.ApplyRoute(ctx, winnet.Route{
		DestinationPrefix: dest,
		NextHop:           nextHop,
		InterfaceIndex:    ifIndex,
		RouteMetric:       1,
		PolicyStore:       "ActiveStore",
		AddressFamily:     family,
	})
}

func removeWindowsDefaultRoute(ctx context.Context, tunName string, family string) error {
	if strings.TrimSpace(tunName) == "" {
		return nil
	}
	ifIndex, err := winnet.InterfaceIndexByName(ctx, tunName)
	if err != nil {
		return nil
	}
	dest := "0.0.0.0/0"
	nextHop := "0.0.0.0"
	if strings.EqualFold(family, "IPv6") {
		dest = "::/0"
		nextHop = "::"
	}
	return winnet.RemoveRoute(ctx, winnet.Route{
		DestinationPrefix: dest,
		NextHop:           nextHop,
		InterfaceIndex:    ifIndex,
		RouteMetric:       1,
		PolicyStore:       "ActiveStore",
		AddressFamily:     family,
	})
}

func toWindowsRoute(route fullTunnelRoute) winnet.Route {
	family := "IPv4"
	if strings.EqualFold(route.Family, "ipv6") {
		family = "IPv6"
	}
	return winnet.Route{
		DestinationPrefix: route.Destination,
		NextHop:           route.NextHop,
		InterfaceIndex:    route.InterfaceIndex,
		RouteMetric:       route.RouteMetric,
		PolicyStore:       route.PolicyStore,
		AddressFamily:     family,
	}
}

func windowsRouteKey(route fullTunnelRoute) string {
	return strings.ToLower(route.Family) + "|" + strings.ToLower(route.Destination) + "|" + strings.ToLower(route.NextHop) + "|" + fmt.Sprintf("%d", route.InterfaceIndex)
}

func hasFamily(routes []winnet.Route, family string) bool {
	for _, route := range routes {
		if strings.EqualFold(route.AddressFamily, family) {
			return true
		}
	}
	return false
}

func encodeWindowsDefaults(routes []winnet.Route) ([]string, []string) {
	var v4 []string
	var v6 []string
	for _, route := range routes {
		entry := encodeWindowsRoute(route)
		if strings.EqualFold(route.AddressFamily, "IPv6") {
			v6 = append(v6, entry)
		} else {
			v4 = append(v4, entry)
		}
	}
	return v4, v6
}

func decodeWindowsRoutes(state fullTunnelState) []winnet.Route {
	var routes []winnet.Route
	for _, raw := range state.IPv4Defaults {
		if route, ok := decodeWindowsRoute(raw); ok {
			route.AddressFamily = "IPv4"
			routes = append(routes, route)
		}
	}
	for _, raw := range state.IPv6Defaults {
		if route, ok := decodeWindowsRoute(raw); ok {
			route.AddressFamily = "IPv6"
			routes = append(routes, route)
		}
	}
	return routes
}

func encodeWindowsRoute(route winnet.Route) string {
	return fmt.Sprintf("%s|%s|%d|%d|%s",
		route.DestinationPrefix,
		route.NextHop,
		route.InterfaceIndex,
		route.RouteMetric,
		route.PolicyStore,
	)
}

func decodeWindowsRoute(raw string) (winnet.Route, bool) {
	parts := strings.Split(raw, "|")
	if len(parts) < 5 {
		return winnet.Route{}, false
	}
	ifIndex := parseInt(parts[2])
	metric := parseInt(parts[3])
	return winnet.Route{
		DestinationPrefix: parts[0],
		NextHop:           parts[1],
		InterfaceIndex:    ifIndex,
		RouteMetric:       metric,
		PolicyStore:       parts[4],
	}, true
}

func parseInt(value string) int {
	out, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return out
}
