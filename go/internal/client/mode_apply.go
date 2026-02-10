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
	if err := applyClientDesiredConfig(paths, state, opts); err != nil {
		return err
	}
	return saveClientAppliedState(paths.stateFile, state, opts.TunEnabled, opts.TunName, opts.TunMTU, opts.TunAddr)
}

func applyClientDesiredConfig(paths clientPaths, state clientInstallState, opts ModeOptions) error {
	xrayCfg, err := ensureClientXrayConfig(paths.configFile)
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
	if opts.TunEnabled {
		return applyRedirectRoutes(opts.TunName, state.Redirects)
	}
	return removeRedirectRoutes(opts.TunName, state.Redirects)
}
