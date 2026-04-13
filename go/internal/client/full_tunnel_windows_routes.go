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
			logFullTunnelVerbose(verbose, "xp2p: full-tunnel bypass route lookup failed", "ip", ip, "err", err)
			filtered = append(filtered, ip)
			continue
		}
		if !ok || tunIfIndex == 0 {
			filtered = append(filtered, ip)
			continue
		}
		if route.InterfaceIndex != tunIfIndex && prefixLen > 0 && !isDefaultRoutePrefix(route.DestinationPrefix) {
			logFullTunnelVerbose(verbose, "xp2p: full-tunnel bypass skipped (direct route exists)", "ip", ip, "route", route.DestinationPrefix, "ifIndex", route.InterfaceIndex)
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
			logFullTunnelVerbose(verbose, "xp2p: full-tunnel bypass route lookup failed", "ip", ip, "err", err)
			filtered = append(filtered, ip)
			continue
		}
		if !ok || tunIfIndex == 0 {
			filtered = append(filtered, ip)
			continue
		}
		if route.InterfaceIndex != tunIfIndex && prefixLen > 0 && !isDefaultRoutePrefix(route.DestinationPrefix) {
			logFullTunnelVerbose(verbose, "xp2p: full-tunnel bypass skipped (direct route exists)", "ip", ip, "route", route.DestinationPrefix, "ifIndex", route.InterfaceIndex)
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
	dest := "0.0.0.0/0"
	nextHop := "0.0.0.0"
	if strings.EqualFold(family, "IPv6") {
		dest = "::/0"
		nextHop = "::"
	}
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		ifIndex, ifLuid, err := resolveWindowsInterface(ctx, tunName, tunAddr, verbose, true)
		if err != nil {
			lastErr = err
		} else {
			if strings.EqualFold(family, "IPv4") {
				if err := waitForWindowsIPv4(ctx, ifIndex, verbose); err != nil {
					lastErr = err
					time.Sleep(500 * time.Millisecond)
					continue
				}
			}
			route := winnet.Route{
				DestinationPrefix: dest,
				NextHop:           nextHop,
				InterfaceIndex:    ifIndex,
				InterfaceLuid:     ifLuid,
				RouteMetric:       1,
				PolicyStore:       "ActiveStore",
				AddressFamily:     family,
			}
			logFullTunnelVerbose(verbose, "xp2p: full-tunnel default route apply", "interface", tunName, "route", route, "attempt", attempt+1)
			if err := winnet.ApplyRoute(ctx, route); err != nil {
				lastErr = err
				if winnet.IsRouteNotFoundError(err) {
					time.Sleep(500 * time.Millisecond)
					continue
				}
				return err
			}
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	if lastErr != nil {
		return lastErr
	}
	return errors.New("xp2p: full-tunnel default route apply failed")
}

func removeWindowsDefaultRoute(ctx context.Context, tunName string, tunAddr string, family string) error {
	if strings.TrimSpace(tunName) == "" {
		return nil
	}
	ifIndex, ifLuid, err := resolveWindowsInterface(ctx, tunName, tunAddr, false, false)
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
		InterfaceLuid:     ifLuid,
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
		InterfaceLuid:     route.InterfaceLuid,
		RouteMetric:       route.RouteMetric,
		InterfaceMetric:   route.InterfaceMetric,
		PolicyStore:       route.PolicyStore,
		AddressFamily:     family,
	}
}

func windowsRouteKey(route fullTunnelRoute) string {
	key := strings.ToLower(route.Family) + "|" + strings.ToLower(route.Destination) + "|" + strings.ToLower(route.NextHop) + "|" + fmt.Sprintf("%d", route.InterfaceIndex)
	if route.InterfaceLuid != 0 {
		key += "|" + fmt.Sprintf("%d", route.InterfaceLuid)
	}
	return key
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
	return fmt.Sprintf("%s|%s|%d|%d|%s|%d|%d",
		route.DestinationPrefix,
		route.NextHop,
		route.InterfaceIndex,
		route.RouteMetric,
		route.PolicyStore,
		route.InterfaceMetric,
		route.InterfaceLuid,
	)
}

func decodeWindowsRoute(raw string) (winnet.Route, bool) {
	parts := strings.Split(raw, "|")
	if len(parts) != 7 {
		return winnet.Route{}, false
	}
	ifIndex := parseInt(parts[2])
	metric := parseInt(parts[3])
	ifMetric := parseInt(parts[5])
	ifLuid := parseUint64(parts[6])
	return winnet.Route{
		DestinationPrefix: parts[0],
		NextHop:           parts[1],
		InterfaceIndex:    ifIndex,
		InterfaceLuid:     ifLuid,
		RouteMetric:       metric,
		InterfaceMetric:   ifMetric,
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

func parseUint64(value string) uint64 {
	out, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return out
}

func resolveWindowsInterface(ctx context.Context, tunName string, tunAddr string, verbose bool, wait bool) (int, uint64, error) {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return 0, 0, errors.New("xp2p: tun name is required for interface lookup")
	}
	trimmedAddr := strings.TrimSpace(tunAddr)
	deadline := time.Now()
	if wait {
		deadline = time.Now().Add(10 * time.Second)
	}
	attempt := 0
	var lastErr error
	for {
		attempt++
		if trimmedAddr != "" {
			ifIndex, addrErr := winnet.InterfaceIndexByIP(trimmedAddr)
			if addrErr == nil && ifIndex > 0 {
				luid, luidErr := winnet.InterfaceLuidByIP(trimmedAddr)
				if luidErr != nil {
					luid = 0
				}
				if verbose {
					logging.Info("full-tunnel tun interface resolved by addr", "interface", name, "addr", trimmedAddr, "ifIndex", ifIndex, "ifLuid", luid, "attempt", attempt)
				}
				return ifIndex, luid, nil
			}
			if addrErr != nil && !errors.Is(addrErr, winnet.ErrInterfaceNotFound) {
				lastErr = addrErr
			}
		}
		ifIndex, ifLuid, matched, matchErr := winnet.InterfaceByNamePrefix(name)
		if matchErr == nil && ifIndex > 0 {
			if verbose {
				logging.Info("full-tunnel tun interface resolved by prefix", "interface", name, "match", matched, "ifIndex", ifIndex, "ifLuid", ifLuid, "attempt", attempt)
			}
			return ifIndex, ifLuid, nil
		}
		index, nameErr := winnet.InterfaceIndexByName(ctx, name)
		if nameErr == nil {
			luid, luidErr := winnet.InterfaceLuidByName(name)
			if luidErr != nil {
				luid = 0
			}
			if verbose {
				logging.Info("full-tunnel tun interface resolved", "interface", name, "ifIndex", index, "ifLuid", luid, "attempt", attempt)
			}
			return index, luid, nil
		}
		lastErr = nameErr
		if errors.Is(lastErr, winnet.ErrInterfaceNotFound) {
			hints := []string{"xray tunnel", "wintun"}
			ifIndex, ifLuid, matched, matchErr = winnet.InterfaceByDescriptionContains(hints)
			if matchErr == nil && ifIndex > 0 {
				if verbose {
					logging.Info("full-tunnel tun interface resolved by description", "interface", name, "match", matched, "ifIndex", ifIndex, "ifLuid", ifLuid, "attempt", attempt)
				}
				return ifIndex, ifLuid, nil
			}
		}
		if ctx.Err() != nil {
			return 0, 0, lastErr
		}
		if lastErr != nil && !errors.Is(lastErr, winnet.ErrInterfaceNotFound) {
			return 0, 0, lastErr
		}
		if time.Now().After(deadline) {
			return 0, 0, lastErr
		}
		if verbose {
			logging.Info("full-tunnel waiting for tun interface", "interface", name, "attempt", attempt)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForWindowsIPv4(ctx context.Context, ifIndex int, verbose bool) error {
	deadline := time.Now().Add(10 * time.Second)
	attempt := 0
	for {
		attempt++
		value, err := winnet.InterfaceIPv4(ctx, ifIndex)
		if err != nil {
			return err
		}
		if strings.TrimSpace(value) != "" {
			if verbose {
				logging.Info("full-tunnel tun IPv4 ready", "ifIndex", ifIndex, "ip", value, "attempt", attempt)
			}
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("xp2p: tun IPv4 address unavailable for interface %d", ifIndex)
		}
		if verbose {
			logging.Info("full-tunnel waiting for tun IPv4", "ifIndex", ifIndex, "attempt", attempt)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func waitForWindowsInterfaceUp(ctx context.Context, ifIndex int, tunName string, verbose bool) error {
	deadline := time.Now().Add(20 * time.Second)
	attempt := 0
	logged := false
	for {
		attempt++
		up, err := winnet.InterfaceIsUpByIndex(ifIndex)
		if err != nil {
			return err
		}
		if up {
			return nil
		}
		if !logged {
			logging.Info("full-tunnel apply deferred: adapter not connected", "interface", tunName, "ifIndex", ifIndex)
			logged = true
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("xp2p: tun adapter not connected: %s (%d)", tunName, ifIndex)
		}
		if verbose {
			logging.Info("full-tunnel waiting for tun adapter", "interface", tunName, "ifIndex", ifIndex, "attempt", attempt)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
