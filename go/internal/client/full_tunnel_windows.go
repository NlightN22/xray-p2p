//go:build windows

package client

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func syncFullTunnel(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState) (bool, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.TunMode))
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel sync start", "tun_enabled", opts.TunEnabled, "tun_mode", mode, "tun_name", opts.TunName)
	if !opts.TunEnabled || mode != "full" {
		logging.Info("full-tunnel sync skipped (not enabled)")
		if windowsRoutesDisabled {
			logging.Info("windows route apply disabled; skipping full-tunnel restore")
		} else {
			if err := restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose); err != nil {
				return false, err
			}
		}
		return false, nil
	}
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
	if windowsRoutesDisabled {
		logging.Warn("full-tunnel route apply disabled on windows")
		return false, nil
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
			return false, errors.New("xp2p: no default routes found for full-tunnel")
		}
		logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes captured", "routes", defaults)
	}

	endpointIPv4 = filterBypassIPv4(endpointIPv4, ifIndex, opts.FullTunnelVerbose)
	endpointIPv6 = filterBypassIPv6(endpointIPv6, ifIndex, opts.FullTunnelVerbose)
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel endpoints resolved", "ipv4", endpointIPv4, "ipv6", endpointIPv6)
	bypassRoutes := buildWindowsBypassRoutes(defaults, endpointIPv4, endpointIPv6)
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel bypass routes prepared", "routes", bypassRoutes)

	defer func() {
		if err == nil {
			return
		}
		if rollbackErr := rollbackFullTunnel(ctx, paths, opts.FullTunnelVerbose, state, defaults); rollbackErr != nil {
			err = fmt.Errorf("xp2p: full-tunnel apply failed: %w (rollback failed: %v)", err, rollbackErr)
		}
	}()

	removeDefaults := !state.Enabled
	if removeDefaults {
		state = fullTunnelState{
			Enabled: true,
			TunName: strings.TrimSpace(opts.TunName),
			TunMode: "full",
			TunAddr: strings.TrimSpace(opts.TunAddr),
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
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes set to tun", "interface", opts.TunName, "ipv4", hasFamily(defaults, "IPv4"), "ipv6", hasFamily(defaults, "IPv6"))

	if removeDefaults {
		for _, route := range defaults {
			logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default route remove", "route", route)
			if err = winnet.RemoveRoute(ctx, route); err != nil {
				return false, err
			}
		}
		logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel default routes removed", "routes", defaults)
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
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel bypass routes applied", "count", len(bypassRoutes))
	logging.Info("full-tunnel apply complete", "bypass_routes", len(bypassRoutes))
	return true, nil
}

func restoreFullTunnel(ctx context.Context, paths clientPaths, verbose bool) error {
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
		_ = removeWindowsDefaultRoute(ctx, tun, state.TunAddr, "IPv4")
		_ = removeWindowsDefaultRoute(ctx, tun, state.TunAddr, "IPv6")
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

func rollbackFullTunnel(ctx context.Context, paths clientPaths, verbose bool, state fullTunnelState, defaults []winnet.Route) error {
	if err := restoreFullTunnel(ctx, paths, verbose); err == nil {
		return nil
	}
	logging.Warn("full-tunnel restore failed; attempting manual rollback")
	if tun := strings.TrimSpace(state.TunName); tun != "" {
		_ = removeWindowsDefaultRoute(ctx, tun, state.TunAddr, "IPv4")
		_ = removeWindowsDefaultRoute(ctx, tun, state.TunAddr, "IPv6")
	}
	if err := removeWindowsBypassRoutes(ctx, state.BypassRoutes); err != nil {
		logging.Warn("full-tunnel bypass rollback failed", "err", err)
	}
	if len(defaults) == 0 {
		defaults = decodeWindowsRoutes(state)
	}
	for _, route := range defaults {
		if err := winnet.ApplyRoute(ctx, route); err != nil {
			logging.Warn("full-tunnel default rollback failed", "route", route, "err", err)
		}
	}
	if err := restoreWindowsDNS(ctx, state.DNSBackup, state.TunName, verbose); err != nil {
		logging.Warn("full-tunnel DNS rollback failed", "err", err)
	}
	return errors.New("xp2p: full-tunnel rollback attempted after restore failure")
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
