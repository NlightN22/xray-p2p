//go:build windows

package client

import (
	"context"
	"net"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

// ApplyFullTunnelRoutes applies full-tunnel routes using the current configuration.
func ApplyFullTunnelRoutes(ctx context.Context, opts RunOptions, bypassTargets []string, forceDefault bool) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	paths, err := resolveClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return false, err
	}
	paths, err = adjustRunPaths(paths)
	if err != nil {
		return false, err
	}

	desired, err := loadClientInstallState(paths.configFile)
	if err != nil {
		return false, err
	}
	if len(bypassTargets) > 0 {
		desired.Endpoints = appendBypassTargets(desired.Endpoints, bypassTargets)
	}

	return applyFullTunnelRoutes(ctx, paths, opts, desired, forceDefault)
}

// RestoreFullTunnelRoutes reverts full-tunnel routing to the baseline defaults.
func RestoreFullTunnelRoutes(ctx context.Context, installDir, configDir string, verbose bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	paths, err := resolveClientPaths(installDir, configDir)
	if err != nil {
		return err
	}
	return restoreFullTunnel(ctx, paths, verbose)
}

func applyFullTunnelRoutes(ctx context.Context, paths clientPaths, opts RunOptions, desired clientInstallState, forceDefault bool) (bool, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.TunMode))
	logFullTunnelVerbose(opts.FullTunnelVerbose, "xp2p: full-tunnel sync start", "tun_enabled", opts.TunEnabled, "tun_mode", mode, "tun_name", opts.TunName)
	if !opts.TunEnabled || mode != "full" {
		logging.Info("xp2p: full-tunnel sync skipped (not enabled)")
		if windowsRoutesDisabled {
			logging.Info("xp2p: windows route apply disabled; skipping full-tunnel restore")
		} else {
			if err := restoreFullTunnel(ctx, paths, opts.FullTunnelVerbose); err != nil {
				return false, err
			}
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
	if forceDefault {
		state.Enabled = false
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
		logging.Warn("xp2p: full-tunnel route apply disabled on windows")
		return false, nil
	}
	return enableFullTunnel(ctx, paths, opts, desired, state, endpointIPv4, endpointIPv6, resolvedEndpoints)
}

func appendBypassTargets(endpoints []clientEndpointRecord, targets []string) []clientEndpointRecord {
	if len(targets) == 0 {
		return endpoints
	}
	seen := make(map[string]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		host := strings.ToLower(strings.TrimSpace(endpointHost(endpoint)))
		if host == "" {
			continue
		}
		seen[host] = struct{}{}
	}
	for _, target := range targets {
		host := extractTargetHost(target)
		if host == "" {
			continue
		}
		key := strings.ToLower(host)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		endpoints = append(endpoints, clientEndpointRecord{Hostname: host})
	}
	return endpoints
}

func extractTargetHost(target string) string {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		return strings.Trim(host, "[]")
	}
	if strings.Count(trimmed, ":") > 1 {
		return strings.Trim(trimmed, "[]")
	}
	return strings.Trim(trimmed, "[]")
}
