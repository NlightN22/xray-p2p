//go:build windows

package preflight

func Tun() TunPreflight {
	return windowsTunPreflight{}
}
