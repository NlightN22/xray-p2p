package client

import (
	"context"
	"net"
)

func init() {
	lookupIPAddrs = func(_ context.Context, host string) ([]net.IPAddr, error) {
		ip := net.ParseIP("203.0.113.10")
		if ip == nil {
			return nil, nil
		}
		return []net.IPAddr{{IP: ip}}, nil
	}
}
