package client

import (
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
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

func syncFullTunnelRouting(paths clientPaths, state clientInstallState, opts RunOptions) error {
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
	)
}
