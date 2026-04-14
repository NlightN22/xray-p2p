//go:build linux || windows

package client

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

// ModeOptions controls inbounds and route updates for mode switches.
type ModeOptions struct {
	InstallDir string
	ConfigDir  string
	TunEnabled bool
	TunName    string
	TunMTU     int
	TunAddr    string
	TunMode    string
	FullTunnelTag string
}

// ApplyModePending updates pending inbounds and routes to match the selected mode.
func ApplyModePending(opts ModeOptions) error {
	paths, err := resolvePendingClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	livePath := filepath.Clean(config.LiveConfigPath(layout.ClientConfigFileName))
	state, err := loadClientInstallStateWithFallback(paths.configFile, livePath)
	if err != nil {
		return err
	}
	return applyClientDesiredConfig(paths, state, opts, false)
}
