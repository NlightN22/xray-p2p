//go:build linux

package client

import (
	"context"
	"errors"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func syncFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	if err := syncFullTunnelRouting(paths, desired, opts); err != nil {
		return false, err
	}
	mode := strings.ToLower(strings.TrimSpace(opts.TunMode))
	if !opts.TunEnabled || mode != "full" {
		if err := restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose); err != nil {
			return false, err
		}
		return false, nil
	}
	if strings.TrimSpace(opts.FullTunnelTag) == "" {
		logging.Warn("xp2p: full-tunnel outbound tag missing; routing rule not added")
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
		logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes captured", "ipv4", defaults4, "ipv6", defaults6)
	}

	endpointIPv4, endpointIPv6, err := resolveEndpointIPs(ctx, desired.Endpoints)
	if err != nil {
		return false, err
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel endpoints resolved", "ipv4", endpointIPv4, "ipv6", endpointIPv6)
	bypass4 := buildBypassRoutes(defaults4, endpointIPv4, 32)
	bypass6 := buildBypassRoutes(defaults6, endpointIPv6, 128)
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel bypass routes prepared", "ipv4", bypass4, "ipv6", bypass6)

	if !state.Enabled {
		state = fullTunnelState{
			Enabled:      true,
			TunName:      strings.TrimSpace(opts.TunName),
			TunMode:      "full",
			IPv4Defaults: defaults4,
			IPv6Defaults: defaults6,
		}
		if len(opts.DNSServers) > 0 {
			backup, dnsErr := applyDNSOverrides(opts.DNSServers, opts.FullTunnelVerbose)
			if dnsErr != nil {
				return false, dnsErr
			}
			state.DNSBackup = backup
		} else {
			logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel DNS unchanged (no servers configured)")
		}
		if err := saveFullTunnelState(paths.fullState, state); err != nil {
			return false, err
		}

		if err := removeDefaultRoutes(defaults4, "-4"); err != nil {
			_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
			return false, err
		}
		if err := removeDefaultRoutes(defaults6, "-6"); err != nil {
			_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
			return false, err
		}
		logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes removed", "ipv4", defaults4, "ipv6", defaults6)
	}

	addedRoutes := make([]fullTunnelRoute, 0, len(bypass4)+len(bypass6))
	if err := syncBypassRoutes(bypass4, state.BypassRoutes, "-4"); err != nil {
		_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
		return false, err
	}
	for _, route := range bypass4 {
		addedRoutes = append(addedRoutes, fullTunnelRoute{Family: "ipv4", Route: route})
	}
	if err := syncBypassRoutes(bypass6, state.BypassRoutes, "-6"); err != nil {
		_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
		return false, err
	}
	for _, route := range bypass6 {
		addedRoutes = append(addedRoutes, fullTunnelRoute{Family: "ipv6", Route: route})
	}

	if len(defaults4) > 0 {
		if err := ensureDefaultRoute(opts.TunName, "-4"); err != nil {
			_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
			return false, err
		}
	}
	if len(defaults6) > 0 {
		if err := ensureDefaultRoute(opts.TunName, "-6"); err != nil {
			_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
			return false, err
		}
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes set to tun", "interface", opts.TunName, "ipv4", len(defaults4) > 0, "ipv6", len(defaults6) > 0)

	state.BypassRoutes = addedRoutes
	if err := saveFullTunnelState(paths.fullState, state); err != nil {
		_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
		return false, err
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel bypass routes applied", "count", len(addedRoutes))
	return true, nil
}

func restoreFullTunnel(_ context.Context, paths clientPaths, verbose bool) error {
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
		logFullTunnelVerbose(verbose, "xp2p: full-tunnel default routes removed from tun", "interface", tun)
	}

	if err := removeStoredBypassRoutes(state.BypassRoutes); err != nil {
		return err
	}
	logFullTunnelVerbose(verbose, "xp2p: full-tunnel bypass routes removed", "count", len(state.BypassRoutes))
	if err := restoreDefaultRoutes(state.IPv4Defaults, "-4"); err != nil {
		return err
	}
	if err := restoreDefaultRoutes(state.IPv6Defaults, "-6"); err != nil {
		return err
	}
	logFullTunnelVerbose(verbose, "xp2p: full-tunnel default routes restored", "ipv4", state.IPv4Defaults, "ipv6", state.IPv6Defaults)
	if err := restoreDNSOverrides(state.DNSBackup, verbose); err != nil {
		return err
	}

	return clearFullTunnelState(paths.fullState)
}

func logFullTunnelVerbose(enabled bool, message string, args ...any) {
	if !enabled {
		return
	}
	if strings.TrimSpace(message) == "" {
		return
	}
	logging.Info(message, args...)
}
