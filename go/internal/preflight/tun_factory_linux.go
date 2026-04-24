//go:build linux

package preflight

func Tun() TunPreflight {
	return linuxTunPreflight{}
}
