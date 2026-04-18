//go:build windows

package windows

type TunnelManager struct{}

func NewTunnelManager() *TunnelManager {
	return &TunnelManager{}
}
