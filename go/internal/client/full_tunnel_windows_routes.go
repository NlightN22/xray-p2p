//go:build windows

package client

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
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

func syncWindowsBypassRoutes(ctx context.Context, desired []fullTunnelRoute, existing []fullTunnelRoute, verbose bool) error {
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
		logFullTunnelVerbose(verbose, "xp2p: full-tunnel bypass route remove", "route", route)
		if err := winnet.RemoveRoute(ctx, toWindowsRoute(route)); err != nil {
			return err
		}
	}
	for _, route := range desired {
		logFullTunnelVerbose(verbose, "xp2p: full-tunnel bypass route apply", "route", route)
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

func ensureWindowsDefaultRoute(ctx context.Context, tunName string, tunAddr string, family string, verbose bool) error {
	if strings.TrimSpace(tunName) == "" {
		return errors.New("xp2p: tun name is required for full-tunnel default route")
	}
	ifIndex, err := resolveWindowsInterfaceIndex(ctx, tunName, tunAddr, verbose, true)
	if err != nil {
		return err
	}
	dest := "0.0.0.0/0"
	nextHop := "0.0.0.0"
	if strings.EqualFold(family, "IPv6") {
		dest = "::/0"
		nextHop = "::"
	}
	route := winnet.Route{
		DestinationPrefix: dest,
		NextHop:           nextHop,
		InterfaceIndex:    ifIndex,
		RouteMetric:       1,
		PolicyStore:       "ActiveStore",
		AddressFamily:     family,
	}
	logFullTunnelVerbose(verbose, "xp2p: full-tunnel default route apply", "interface", tunName, "route", route)
	return winnet.ApplyRoute(ctx, route)
}

func removeWindowsDefaultRoute(ctx context.Context, tunName string, tunAddr string, family string) error {
	if strings.TrimSpace(tunName) == "" {
		return nil
	}
	ifIndex, err := resolveWindowsInterfaceIndex(ctx, tunName, tunAddr, false, false)
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

func resolveWindowsInterfaceIndex(ctx context.Context, tunName string, tunAddr string, verbose bool, wait bool) (int, error) {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return 0, errors.New("xp2p: tun name is required for interface lookup")
	}
	trimmedAddr := strings.TrimSpace(tunAddr)
	deadline := time.Now()
	if wait {
		deadline = time.Now().Add(5 * time.Second)
	}
	attempt := 0
	for {
		attempt++
		index, err := winnet.InterfaceIndexByName(ctx, name)
		if err == nil {
			if verbose {
				logging.Info("xp2p: full-tunnel tun interface resolved", "interface", name, "ifIndex", index, "attempt", attempt)
			}
			return index, nil
		}
		if errors.Is(err, winnet.ErrInterfaceNotFound) && trimmedAddr != "" {
			ifIndex, addrErr := winnet.InterfaceIndexByIP(trimmedAddr)
			if addrErr == nil && ifIndex > 0 {
				if verbose {
					logging.Info("xp2p: full-tunnel tun interface resolved by addr", "interface", name, "addr", trimmedAddr, "ifIndex", ifIndex, "attempt", attempt)
				}
				return ifIndex, nil
			}
		}
		if ctx.Err() != nil {
			return 0, err
		}
		if !errors.Is(err, winnet.ErrInterfaceNotFound) || time.Now().After(deadline) {
			return 0, err
		}
		if verbose {
			logging.Info("xp2p: full-tunnel waiting for tun interface", "interface", name, "attempt", attempt)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
