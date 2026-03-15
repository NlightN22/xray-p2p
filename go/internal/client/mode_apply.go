//go:build linux || windows

package client

import (
	"path/filepath"
)

func applyClientMode(paths clientPaths, opts ModeOptions) error {
	state, err := loadClientInstallState(paths.configFile)
	if err != nil {
		return err
	}
	return applyClientDesiredConfig(paths, state, opts, false)
}

func applyClientDesiredConfig(paths clientPaths, state clientInstallState, opts ModeOptions, applyRoutes bool) error {
	xrayCfg, err := loadClientXrayConfig(paths.configFile)
	if err != nil {
		return err
	}
	if err := writeClientInboundsConfig(paths.configDir, xrayCfg, opts.TunEnabled, opts.TunName, opts.TunMTU, state.Forwards); err != nil {
		return err
	}
	if err := writeOutboundsConfig(filepath.Join(paths.configDir, "outbounds.json"), xrayCfg.DirectOutbound, state.Endpoints); err != nil {
		return err
	}
	if err := updateRoutingConfig(filepath.Join(paths.configDir, "routing.json"), xrayCfg.Routing, state.Endpoints, state.Redirects, state.Reverse); err != nil {
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
