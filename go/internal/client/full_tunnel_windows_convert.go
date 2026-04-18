//go:build windows

package client

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

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
