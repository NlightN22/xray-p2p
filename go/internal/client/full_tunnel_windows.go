//go:build windows

package client

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func syncFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	return ensureFullTunnel(ctx, paths, opts, desired)
}

func ensureFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.TunMode))
	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel sync start", "tun_enabled", opts.TunEnabled, "tun_mode", mode, "tun_name", opts.TunName)
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

func enableFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState, state fullTunnelState, endpointIPv4 []string, endpointIPv6 []string, resolvedEndpoints map[string]fullTunnelEndpointIPs) (enabled bool, err error) {
	logging.Info("full-tunnel apply start", "tun_name", opts.TunName, "tun_addr", opts.TunAddr)
	ifIndex, _, err := resolveWindowsInterface(ctx, opts.TunName, opts.TunAddr, opts.FullTunnelVerbose, true)
	if err != nil {
		return false, err
	}
	if err := waitForWindowsInterfaceUp(ctx, ifIndex, opts.TunName, opts.FullTunnelVerbose); err != nil {
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
			return false, errors.New("no default routes found for full-tunnel")
		}
		logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel default routes captured", "routes", defaults)
	}

	endpointIPv4 = filterBypassIPv4(endpointIPv4, ifIndex, opts.FullTunnelVerbose)
	endpointIPv6 = filterBypassIPv6(endpointIPv6, ifIndex, opts.FullTunnelVerbose)
	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel endpoints resolved", "ipv4", endpointIPv4, "ipv6", endpointIPv6)
	bypassRoutes := buildWindowsBypassRoutes(defaults, endpointIPv4, endpointIPv6)
	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel bypass routes prepared", "routes", bypassRoutes)

	removeDefaults := !state.Enabled
	if removeDefaults {
		state.Enabled = true
		state.TunName = strings.TrimSpace(opts.TunName)
		state.TunMode = "full"
		state.TunAddr = strings.TrimSpace(opts.TunAddr)
		state.BypassRoutes = nil
		state.DNSBackup = nil
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
			logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel DNS unchanged (no servers configured)")
		}
		if err = saveFullTunnelState(paths.fullState, state); err != nil {
			return false, err
		}
	}

	if err = syncWindowsBypassRoutes(ctx, bypassRoutes, state.BypassRoutes, opts.FullTunnelVerbose); err != nil {
		return false, err
	}

	if hasFamily(defaults, "IPv4") {
		if err = ensureWindowsDefaultRoute(ctx, opts.TunName, opts.TunAddr, "IPv4", opts.FullTunnelVerbose); err != nil {
			return false, err
		}
	}
	if hasFamily(defaults, "IPv6") {
		if err = ensureWindowsDefaultRoute(ctx, opts.TunName, opts.TunAddr, "IPv6", opts.FullTunnelVerbose); err != nil {
			return false, err
		}
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel default routes set to tun", "interface", opts.TunName, "ipv4", hasFamily(defaults, "IPv4"), "ipv6", hasFamily(defaults, "IPv6"))

	if removeDefaults {
		for _, route := range defaults {
			logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel default route remove", "route", route)
			if err = winnet.RemoveRoute(ctx, route); err != nil {
				return false, err
			}
		}
		logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel default routes removed", "routes", defaults)
	}

	if state.Enabled {
		if len(resolvedEndpoints) > 0 {
			state.EndpointIPs = resolvedEndpoints
		} else {
			state.EndpointIPs = nil
		}
		state.TunAddr = strings.TrimSpace(opts.TunAddr)
	}
	state.BypassRoutes = bypassRoutes
	if err = saveFullTunnelState(paths.fullState, state); err != nil {
		return false, err
	}
	logFullTunnelVerbose(opts.FullTunnelVerbose, "full-tunnel bypass routes applied", "count", len(bypassRoutes))
	logging.Info("full-tunnel apply complete", "bypass_routes", len(bypassRoutes))
	return true, nil
}

func restoreFullTunnel(ctx context.Context, paths clientPaths, verbose bool) error {
	if ctx != nil && ctx.Err() != nil {
		return nil
	}

	logging.Info("full-tunnel restore start")
	state, err := loadFullTunnelState(paths.fullState)
	if err != nil {
		return err
	}
	if !state.Enabled {
		logging.Info("full-tunnel restore skipped (disabled)")
		return nil
	}

	if tun := strings.TrimSpace(state.TunName); tun != "" {
		if err := removeWindowsDefaultRoute(ctx, tun, state.TunAddr, "IPv4"); err != nil {
			logging.Warn("full-tunnel IPv4 default route removal failed", "interface", tun, "err", err)
		}
		if err := removeWindowsDefaultRoute(ctx, tun, state.TunAddr, "IPv6"); err != nil {
			logging.Warn("full-tunnel IPv6 default route removal failed", "interface", tun, "err", err)
		}
		if err := winnet.ForceRemoveDefaultRoutesByPrefix(ctx, tun, "IPv4"); err != nil {
			logging.Warn("full-tunnel IPv4 default route force removal failed", "interface", tun, "err", err)
		}
		if err := winnet.ForceRemoveDefaultRoutesByPrefix(ctx, tun, "IPv6"); err != nil {
			logging.Warn("full-tunnel IPv6 default route force removal failed", "interface", tun, "err", err)
		}
		logFullTunnelVerbose(verbose, "full-tunnel default routes removed from tun", "interface", tun)
	}
	if err := removeWindowsBypassRoutes(ctx, state.BypassRoutes); err != nil {
		return err
	}
	logFullTunnelVerbose(verbose, "full-tunnel bypass routes removed", "count", len(state.BypassRoutes))

	defaults := decodeWindowsRoutes(state)
	for _, route := range defaults {
		if err := winnet.ApplyRoute(ctx, route); err != nil {
			return err
		}
	}
	logFullTunnelVerbose(verbose, "full-tunnel default routes restored", "routes", defaults)
	if err := restoreWindowsDNS(ctx, state.DNSBackup, state.TunName, verbose); err != nil {
		return err
	}
	state.Enabled = false
	state.Phase = string(OSStatePhaseDisabled)
	state.LastError = ""
	state.LastErrorAt = time.Time{}
	state.PendingReason = ""
	state.PendingSince = time.Time{}
	state.RetryCount = 0
	state.NextRetryAt = time.Time{}
	state.TunName = ""
	state.TunMode = ""
	state.TunAddr = ""
	state.IPv4Defaults = nil
	state.IPv6Defaults = nil
	state.BypassRoutes = nil
	state.DNSBackup = nil
	if err := saveFullTunnelState(paths.fullState, state); err != nil {
		return err
	}
	logging.Info("full-tunnel restore complete")
	return nil
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
