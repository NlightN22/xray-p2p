package client

import (
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func loadFullTunnelRouteSettings(configFile string) (bool, string, error) {
	cfg, err := config.Load(config.Options{
		Path:         configFile,
		AllowInvalid: true,
	})
	if err != nil {
		return false, "", err
	}
	enabled := cfg.Client.TunEnabled && strings.EqualFold(cfg.Client.TunMode, "full")
	return enabled, strings.TrimSpace(cfg.Client.FullTunnelTag), nil
}

func syncFullTunnelRouting(paths clientPaths, state clientInstallState, opts RunOptions, endpointIPs map[string]fullTunnelEndpointIPs, requireEndpointIPs bool) error {
	xrayCfg, err := loadClientXrayConfig(paths.configFile)
	if err != nil {
		return err
	}
	fullEnabled := opts.TunEnabled && strings.EqualFold(strings.TrimSpace(opts.TunMode), "full")
	return updateRoutingConfig(
		filepath.Join(paths.configDir, "routing.json"),
		xrayCfg.Routing,
		state.Endpoints,
		state.Redirects,
		state.Reverse,
		fullEnabled,
		opts.FullTunnelTag,
		endpointIPs,
		requireEndpointIPs,
	)
}

func syncFullTunnelOutbounds(paths clientPaths, state clientInstallState, endpointIPs map[string]fullTunnelEndpointIPs, requireEndpointIPs bool) error {
	xrayCfg, err := loadClientXrayConfig(paths.configFile)
	if err != nil {
		return err
	}
	return writeOutboundsConfig(filepath.Join(paths.configDir, "outbounds.json"), xrayCfg.DirectOutbound, state.Endpoints, endpointIPs, requireEndpointIPs)
}

func loadFullTunnelEndpointCache() (map[string]fullTunnelEndpointIPs, error) {
	state, err := loadFullTunnelState(config.ConfigPath(layout.ClientFullTunnelStateFileName))
	if err != nil {
		return nil, err
	}
	if len(state.EndpointIPs) == 0 {
		return nil, nil
	}
	return state.EndpointIPs, nil
}
