//go:build linux || windows

package client

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

// ApplyMode updates inbounds and routes to match the selected mode.
func ApplyMode(opts ModeOptions) error {
	paths, err := resolveClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	return applyClientMode(paths, opts)
}
