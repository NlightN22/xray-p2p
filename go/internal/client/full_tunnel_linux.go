//go:build linux

package client

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func syncFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	return ensureFullTunnel(ctx, paths, opts, desired)
}

func ensureFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	if strings.TrimSpace(opts.FullTunnelTag) == "" {
		logging.Warn("full-tunnel outbound tag missing; routing rule not added")
	}
	state, err := loadFullTunnelState(paths.fullState)
	if err != nil {
		return false, err
	}
	endpointIPv4, endpointIPv6, resolvedEndpoints, err := resolveEndpointIPs(ctx, desired.Endpoints, state.EndpointIPs)
	if err != nil {
		return false, err
	}
	if endpointCacheNeedsUpdate(state.EndpointIPs, resolvedEndpoints) {
		state.EndpointIPs = resolvedEndpoints
		if err := saveFullTunnelState(paths.fullState, state); err != nil {
			return false, err
		}
	}
	return enableFullTunnel(ctx, paths, opts, desired, state, endpointIPv4, endpointIPv6, resolvedEndpoints)
}

func enableFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState, state fullTunnelState, endpointIPv4 []string, endpointIPv6 []string, resolvedEndpoints map[string]fullTunnelEndpointIPs) (bool, error) {

	defaults4 := state.IPv4Defaults
	defaults6 := state.IPv6Defaults
	var err error
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
			return false, errors.New("no default routes found for full-tunnel")
		}
		logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel default routes captured", "ipv4", defaults4, "ipv6", defaults6)
	}

	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel endpoints resolved", "ipv4", endpointIPv4, "ipv6", endpointIPv6)
	bypass4 := buildBypassRoutes(defaults4, endpointIPv4, 32)
	bypass6 := buildBypassRoutes(defaults6, endpointIPv6, 128)
	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel bypass routes prepared", "ipv4", bypass4, "ipv6", bypass6)

	if !state.Enabled {
		state.Enabled = true
		state.TunName = strings.TrimSpace(opts.TunName)
		state.TunMode = "full"
		state.TunAddr = ""
		state.IPv4Defaults = defaults4
		state.IPv6Defaults = defaults6
		state.BypassRoutes = nil
		state.DNSBackup = nil
		if len(resolvedEndpoints) > 0 {
			state.EndpointIPs = resolvedEndpoints
		}
		if len(opts.DNSServers) > 0 {
			backup, dnsErr := applyDNSOverrides(opts.DNSServers, opts.FullTunnelVerbose)
			if dnsErr != nil {
				return false, dnsErr
			}
			state.DNSBackup = backup
		} else {
			logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel DNS unchanged (no servers configured)")
		}
		if err := saveFullTunnelState(paths.fullState, state); err != nil {
			return false, err
		}

		if err := removeDefaultRoutes(defaults4, "-4"); err != nil {
			return false, err
		}
		if err := removeDefaultRoutes(defaults6, "-6"); err != nil {
			return false, err
		}
		logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel default routes removed", "ipv4", defaults4, "ipv6", defaults6)
	} else {
		if len(resolvedEndpoints) > 0 {
			state.EndpointIPs = resolvedEndpoints
		} else {
			state.EndpointIPs = nil
		}
	}

	addedRoutes := make([]fullTunnelRoute, 0, len(bypass4)+len(bypass6))
	if err := syncBypassRoutes(bypass4, state.BypassRoutes, "-4"); err != nil {
		return false, err
	}
	for _, route := range bypass4 {
		addedRoutes = append(addedRoutes, fullTunnelRoute{Family: "ipv4", Route: route})
	}
	if err := syncBypassRoutes(bypass6, state.BypassRoutes, "-6"); err != nil {
		return false, err
	}
	for _, route := range bypass6 {
		addedRoutes = append(addedRoutes, fullTunnelRoute{Family: "ipv6", Route: route})
	}

	if len(defaults4) > 0 {
		if err := ensureDefaultRoute(opts.TunName, "-4"); err != nil {
			return false, err
		}
	}
	if len(defaults6) > 0 {
		if err := ensureDefaultRoute(opts.TunName, "-6"); err != nil {
			return false, err
		}
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel default routes set to tun", "interface", opts.TunName, "ipv4", len(defaults4) > 0, "ipv6", len(defaults6) > 0)

	state.BypassRoutes = addedRoutes
	if err := saveFullTunnelState(paths.fullState, state); err != nil {
		return false, err
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel bypass routes applied", "count", len(addedRoutes))
	return true, nil
}

func restoreFullTunnel(ctx context.Context, paths clientPaths, verbose bool) error {
	if ctx != nil && ctx.Err() != nil {
		return nil
	}

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
		logFullTunnelVerbose(verbose, "full-tunnel default routes removed from tun", "interface", tun)
	}

	if err := removeStoredBypassRoutes(state.BypassRoutes); err != nil {
		return err
	}
	logFullTunnelVerbose(verbose, "full-tunnel bypass routes removed", "count", len(state.BypassRoutes))
	if err := restoreDefaultRoutes(state.IPv4Defaults, "-4"); err != nil {
		return err
	}
	if err := restoreDefaultRoutes(state.IPv6Defaults, "-6"); err != nil {
		return err
	}
	logFullTunnelVerbose(verbose, "full-tunnel default routes restored", "ipv4", state.IPv4Defaults, "ipv6", state.IPv6Defaults)
	if err := restoreDNSOverrides(state.DNSBackup, verbose); err != nil {
		return err
	}
	state.Enabled = false
	state.Phase = string(OSStatePhaseDisabled)
	state.LastError = ""
	state.LastErrorAt = time.Time{}
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
