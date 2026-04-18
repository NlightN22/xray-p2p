//go:build windows

package client

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func ensureWindowsDefaultRoute(ctx context.Context, tunName string, tunAddr string, family string, verbose bool) error {
	if strings.TrimSpace(tunName) == "" {
		return errors.New("tun name is required for full-tunnel default route")
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
			logFullTunnelVerbose(verbose, "full-tunnel default route apply", "interface", tunName, "route", route, "attempt", attempt+1)
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
	return errors.New("full-tunnel default route apply failed")
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
