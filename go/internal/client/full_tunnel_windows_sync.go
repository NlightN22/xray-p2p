//go:build windows

package client

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

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
		logFullTunnelVerbose(verbose, "full-tunnel bypass route remove", "route", route)
		if err := winnet.RemoveRoute(ctx, toWindowsRoute(route)); err != nil {
			return err
		}
	}
	for _, route := range desired {
		logFullTunnelVerbose(verbose, "full-tunnel bypass route apply", "route", route)
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
