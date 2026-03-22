//go:build windows

package client

import (
	"context"
	"errors"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func syncFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.TunMode))
	if !opts.TunEnabled || mode != "full" {
		if err := syncFullTunnelRouting(paths, desired, opts, nil, false); err != nil {
			return false, err
		}
		if err := restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose); err != nil {
			return false, err
		}
		return false, nil
	}
	if strings.TrimSpace(opts.FullTunnelTag) == "" {
		logging.Warn("xp2p: full-tunnel outbound tag missing; routing rule not added")
	}
	state, err := loadFullTunnelState(paths.fullState)
	if err != nil {
		return false, err
	}
	endpointIPv4, endpointIPv6, resolvedEndpoints, err := resolveEndpointIPs(ctx, desired.Endpoints, state.EndpointIPs)
	if err != nil {
		return false, err
	}
	if err := syncFullTunnelRouting(paths, desired, opts, resolvedEndpoints, true); err != nil {
		return false, err
	}
	return enableFullTunnel(ctx, paths, opts, desired, state, endpointIPv4, endpointIPv6, resolvedEndpoints)
}

func enableFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState, state fullTunnelState, endpointIPv4 []string, endpointIPv6 []string, resolvedEndpoints map[string]fullTunnelEndpointIPs) (bool, error) {

	var defaults []winnet.Route
	var err error
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
		logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes captured", "routes", defaults)
	}

	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel endpoints resolved", "ipv4", endpointIPv4, "ipv6", endpointIPv6)
	bypassRoutes := buildWindowsBypassRoutes(defaults, endpointIPv4, endpointIPv6)
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel bypass routes prepared", "routes", bypassRoutes)

	if !state.Enabled {
		state = fullTunnelState{
			Enabled: true,
			TunName: strings.TrimSpace(opts.TunName),
			TunMode: "full",
		}
		if len(resolvedEndpoints) > 0 {
			state.EndpointIPs = resolvedEndpoints
		}
		state.IPv4Defaults, state.IPv6Defaults = encodeWindowsDefaults(defaults)
		if len(opts.DNSServers) > 0 {
			backup, dnsErr := applyWindowsDNS(ctx, opts.TunName, opts.DNSServers, opts.FullTunnelVerbose)
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

		for _, route := range defaults {
			if err := winnet.RemoveRoute(ctx, route); err != nil {
				_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
				return false, err
			}
		}
		logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes removed", "routes", defaults)
	}

	if err := syncWindowsBypassRoutes(ctx, bypassRoutes, state.BypassRoutes); err != nil {
		_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
		return false, err
	}

	if hasFamily(defaults, "IPv4") {
		if err := ensureWindowsDefaultRoute(ctx, opts.TunName, "IPv4"); err != nil {
			_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
			return false, err
		}
	}
	if hasFamily(defaults, "IPv6") {
		if err := ensureWindowsDefaultRoute(ctx, opts.TunName, "IPv6"); err != nil {
			_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
			return false, err
		}
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes set to tun", "interface", opts.TunName, "ipv4", hasFamily(defaults, "IPv4"), "ipv6", hasFamily(defaults, "IPv6"))

	if state.Enabled {
		if len(resolvedEndpoints) > 0 {
			state.EndpointIPs = resolvedEndpoints
		} else {
			state.EndpointIPs = nil
		}
	}
	state.BypassRoutes = bypassRoutes
	if err := saveFullTunnelState(paths.fullState, state); err != nil {
		_ = restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose)
		return false, err
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel bypass routes applied", "count", len(bypassRoutes))
	return true, nil
}

func restoreFullTunnel(ctx context.Context, paths clientPaths, verbose bool) error {
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
		logFullTunnelVerbose(verbose, "xp2p: full-tunnel default routes removed from tun", "interface", tun)
	}
	if err := removeWindowsBypassRoutes(ctx, state.BypassRoutes); err != nil {
		return err
	}
	logFullTunnelVerbose(verbose, "xp2p: full-tunnel bypass routes removed", "count", len(state.BypassRoutes))

	defaults := decodeWindowsRoutes(state)
	for _, route := range defaults {
		if err := winnet.ApplyRoute(ctx, route); err != nil {
			return err
		}
	}
	logFullTunnelVerbose(verbose, "xp2p: full-tunnel default routes restored", "routes", defaults)
	if err := restoreWindowsDNS(ctx, state.DNSBackup, state.TunName, verbose); err != nil {
		return err
	}
	state.Enabled = false
	state.TunName = ""
	state.TunMode = ""
	state.IPv4Defaults = nil
	state.IPv6Defaults = nil
	state.BypassRoutes = nil
	state.DNSBackup = nil
	return saveFullTunnelState(paths.fullState, state)
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
