package servercmd

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/preflight"
)

var tunPreflightCheckFunc = func(ctx context.Context, cfg preflight.TunConfig) error {
	return preflight.CheckTun(ctx, cfg)
}
