//go:build linux || windows

package client

import (
	"context"
	"path/filepath"
	"strings"
)

func applyClientDesiredConfig(paths clientPaths, state clientInstallState, opts ModeOptions, applyRoutes bool) error {
	xrayCfg, err := loadClientXrayConfig(paths.configFile)
	if err != nil {
		return err
	}
	if err := writeClientInboundsConfig(paths.configDir, xrayCfg, opts.TunEnabled, opts.TunName, opts.TunMTU, state.Forwards); err != nil {
		return err
	}
	endpointIPs, err := resolveEndpointIPMapWithCache(context.Background(), state.Endpoints)
	if err != nil {
		return err
	}
	if err := writeOutboundsConfig(filepath.Join(paths.configDir, "outbounds.json"), xrayCfg.DirectOutbound, state.Endpoints, endpointIPs, true); err != nil {
		return err
	}
	fullEnabled := opts.TunEnabled && strings.EqualFold(strings.TrimSpace(opts.TunMode), "full")
	var routeEndpointIPs map[string]fullTunnelEndpointIPs
	if fullEnabled {
		var cacheErr error
		routeEndpointIPs, cacheErr = loadFullTunnelEndpointCache()
		if cacheErr != nil {
			return cacheErr
		}
	}
	if err := updateRoutingConfig(filepath.Join(paths.configDir, "routing.json"), xrayCfg.Routing, state.Endpoints, state.Redirects, state.Reverse, fullEnabled, opts.FullTunnelTag, routeEndpointIPs, false); err != nil {
		return err
	}
	if !applyRoutes {
		return nil
	}
	if opts.TunEnabled {
		return applyRedirectRoutes(opts.TunName, opts.TunAddr, state.Redirects)
	}
	return removeRedirectRoutes(opts.TunName, opts.TunAddr, state.Redirects)
}
