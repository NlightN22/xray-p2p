package client

import (
	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
)

func loadFullTunnelEndpointCache() (map[string]fullTunnelEndpointIPs, error) {
	state, err := loadFullTunnelState(config.ConfigPath(layout.ClientFullTunnelStateFileName))
	if err != nil {
		return nil, err
	}
	if state.EndpointIPs == nil {
		return map[string]fullTunnelEndpointIPs{}, nil
	}
	return state.EndpointIPs, nil
}

