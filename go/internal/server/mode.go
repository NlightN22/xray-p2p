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
