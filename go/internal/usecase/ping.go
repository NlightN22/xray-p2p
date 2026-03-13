package usecase

import (
	"context"

	"github.com/NlightN22/xray-p2p/go/internal/ports"
)

type Ping struct {
	client ports.PingClient
}

func NewPing(client ports.PingClient) *Ping {
	return &Ping{client: client}
}

func (p *Ping) Run(ctx context.Context, target string) (ports.PingResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.PingResult{}, err
	}
	return p.client.Ping(ctx, target)
}
