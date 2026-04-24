//go:build !linux && !windows

package preflight

func Tun() TunPreflight {
	return defaultTunPreflight{}
}
