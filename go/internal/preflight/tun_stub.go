//go:build !linux && !windows

package preflight

import (
	"context"
	"runtime"
)

type defaultTunPreflight struct{}

func (defaultTunPreflight) Check(ctx context.Context, cfg TunConfig) error {
	_ = ctx
	if !cfg.Enabled {
		return nil
	}
	return ErrTunUnavailable{
		OS:     runtime.GOOS,
		Reason: "tun preflight is not implemented for this platform",
		Hint:   "disable TUN mode",
	}
}
