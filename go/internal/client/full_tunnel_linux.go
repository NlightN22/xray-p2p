//go:build linux

package client

import (
	"context"
	"errors"
	"strings"
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

	defaults4 := state.IPv4Defaults
	defaults6 := state.IPv6Defaults
	if !state.Enabled {
		defaults4, err = listDefaultRoutes("-4")
		if err != nil {
			return false, err
		}
		defaults6, err = listDefaultRoutes("-6")
		if err != nil {
			return false, err
		}
		if len(defaults4) == 0 && len(defaults6) == 0 {
			return false, errors.New("xp2p: no default routes found for full-tunnel")
		}
	}

	endpointIPv4, endpointIPv6, err := resolveEndpointIPs(ctx, desired.Endpoints)
	if err != nil {
		return false, err
	}
	bypass4 := buildBypassRoutes(defaults4, endpointIPv4, 32)
	bypass6 := buildBypassRoutes(defaults6, endpointIPv6, 128)

	if !state.Enabled {
		state = fullTunnelState{
			Enabled:      true,
			TunName:      strings.TrimSpace(opts.TunName),
			TunMode:      "full",
			IPv4Defaults: defaults4,
			IPv6Defaults: defaults6,
		}
		if len(opts.DNSServers) > 0 {
			backup, dnsErr := applyDNSOverrides(opts.DNSServers)
			if dnsErr != nil {
				return false, dnsErr
			}
			state.DNSBackup = backup
		}
		if err := saveFullTunnelState(paths.fullState, state); err != nil {
			return false, err
		}

		if err := removeDefaultRoutes(defaults4, "-4"); err != nil {
			_ = restoreFullTunnel(ctx, paths)
			return false, err
		}
		if err := removeDefaultRoutes(defaults6, "-6"); err != nil {
			_ = restoreFullTunnel(ctx, paths)
			return false, err
		}
	}

	addedRoutes := make([]fullTunnelRoute, 0, len(bypass4)+len(bypass6))
	if err := syncBypassRoutes(bypass4, state.BypassRoutes, "-4"); err != nil {
		_ = restoreFullTunnel(ctx, paths)
		return false, err
	}
	for _, route := range bypass4 {
		addedRoutes = append(addedRoutes, fullTunnelRoute{Family: "ipv4", Route: route})
	}
	if err := syncBypassRoutes(bypass6, state.BypassRoutes, "-6"); err != nil {
		_ = restoreFullTunnel(ctx, paths)
		return false, err
	}
	for _, route := range bypass6 {
		addedRoutes = append(addedRoutes, fullTunnelRoute{Family: "ipv6", Route: route})
	}

	if len(defaults4) > 0 {
		if err := ensureDefaultRoute(opts.TunName, "-4"); err != nil {
			_ = restoreFullTunnel(ctx, paths)
			return false, err
		}
	}
	if len(defaults6) > 0 {
		if err := ensureDefaultRoute(opts.TunName, "-6"); err != nil {
			_ = restoreFullTunnel(ctx, paths)
			return false, err
		}
	}

	state.BypassRoutes = addedRoutes
	if err := saveFullTunnelState(paths.fullState, state); err != nil {
		_ = restoreFullTunnel(ctx, paths)
		return false, err
	}
	return true, nil
}

func restoreFullTunnel(_ context.Context, paths clientPaths) error {
	state, err := loadFullTunnelState(paths.fullState)
	if err != nil {
		return err
	}
	if !state.Enabled {
		return nil
	}

	if tun := strings.TrimSpace(state.TunName); tun != "" {
		_ = removeTunDefaultRoute(tun, "-4")
		_ = removeTunDefaultRoute(tun, "-6")
	}

	if err := removeStoredBypassRoutes(state.BypassRoutes); err != nil {
		return err
	}
	if err := restoreDefaultRoutes(state.IPv4Defaults, "-4"); err != nil {
		return err
	}
	if err := restoreDefaultRoutes(state.IPv6Defaults, "-6"); err != nil {
		return err
	}
	if err := restoreDNSOverrides(state.DNSBackup); err != nil {
		return err
	}

	return clearFullTunnelState(paths.fullState)
}
