//go:build windows

package client

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

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
