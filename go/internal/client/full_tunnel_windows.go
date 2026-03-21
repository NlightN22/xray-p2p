//go:build windows

package client

import (
	"context"
	"errors"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func syncFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.TunMode))
	if !opts.TunEnabled || mode != "full" {
		if err := restoreFullTunnel(ctx, paths); err != nil {
			return false, err
		}
		return false, nil
	}
	return enableFullTunnel(ctx, paths, opts, desired)
}

func enableFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	state, err := loadFullTunnelState(paths.fullState)
	if err != nil {
		return false, err
	}

	var defaults []winnet.Route
	if state.Enabled {
		defaults = decodeWindowsRoutes(state)
	} else {
		defaults, err = winnet.DefaultRoutes(ctx)
		if err != nil {
			return false, err
		}
		if len(defaults) == 0 {
			return false, errors.New("xp2p: no default routes found for full-tunnel")
		}
	}

	endpointIPv4, endpointIPv6, err := resolveEndpointIPs(ctx, desired.Endpoints)
	if err != nil {
		return false, err
	}
	bypassRoutes := buildWindowsBypassRoutes(defaults, endpointIPv4, endpointIPv6)

	if !state.Enabled {
		state = fullTunnelState{
			Enabled: true,
			TunName: strings.TrimSpace(opts.TunName),
			TunMode: "full",
		}
		state.IPv4Defaults, state.IPv6Defaults = encodeWindowsDefaults(defaults)
		if len(opts.DNSServers) > 0 {
			backup, dnsErr := applyWindowsDNS(ctx, opts.TunName, opts.DNSServers)
			if dnsErr != nil {
				return false, dnsErr
			}
			state.DNSBackup = backup
		}
		if err := saveFullTunnelState(paths.fullState, state); err != nil {
			return false, err
		}

		for _, route := range defaults {
			if err := winnet.RemoveRoute(ctx, route); err != nil {
				_ = restoreFullTunnel(ctx, paths)
				return false, err
			}
		}
	}

	if err := syncWindowsBypassRoutes(ctx, bypassRoutes, state.BypassRoutes); err != nil {
		_ = restoreFullTunnel(ctx, paths)
		return false, err
	}

	if hasFamily(defaults, "IPv4") {
		if err := ensureWindowsDefaultRoute(ctx, opts.TunName, "IPv4"); err != nil {
			_ = restoreFullTunnel(ctx, paths)
			return false, err
		}
	}
	if hasFamily(defaults, "IPv6") {
		if err := ensureWindowsDefaultRoute(ctx, opts.TunName, "IPv6"); err != nil {
			_ = restoreFullTunnel(ctx, paths)
			return false, err
		}
	}

	state.BypassRoutes = bypassRoutes
	if err := saveFullTunnelState(paths.fullState, state); err != nil {
		_ = restoreFullTunnel(ctx, paths)
		return false, err
	}
	return true, nil
}

func restoreFullTunnel(ctx context.Context, paths clientPaths) error {
	state, err := loadFullTunnelState(paths.fullState)
	if err != nil {
		return err
	}
	if !state.Enabled {
		return nil
	}

	if tun := strings.TrimSpace(state.TunName); tun != "" {
		_ = removeWindowsDefaultRoute(ctx, tun, "IPv4")
		_ = removeWindowsDefaultRoute(ctx, tun, "IPv6")
	}
	if err := removeWindowsBypassRoutes(ctx, state.BypassRoutes); err != nil {
		return err
	}

	defaults := decodeWindowsRoutes(state)
	for _, route := range defaults {
		if err := winnet.ApplyRoute(ctx, route); err != nil {
			return err
		}
	}
	if err := restoreWindowsDNS(ctx, state.DNSBackup, state.TunName); err != nil {
		return err
	}
	return clearFullTunnelState(paths.fullState)
}
