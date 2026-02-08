//go:build linux || windows

package server

// ModeOptions controls inbounds and route updates for mode switches.
type ModeOptions struct {
	InstallDir string
	ConfigDir  string
	TunEnabled bool
	TunName    string
	TunMTU     int
	TunAddr    string
}

// ApplyMode updates inbounds and routes to match the selected mode.
func ApplyMode(opts ModeOptions) error {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return err
	}
	configDir, err := resolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return err
	}
	return applyServerMode(installDir, configDir, opts)
}
