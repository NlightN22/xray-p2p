//go:build windows

package windows

import (
	"context"
	"net"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
	"github.com/NlightN22/xray-p2p/go/internal/ports"
)

type PingClient struct{}

func NewPingClient() *PingClient {
	return &PingClient{}
}

func (p *PingClient) Ping(ctx context.Context, target string) (ports.PingResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.PingResult{}, err
	}

	var rtt time.Duration
	opts := ping.Options{
		Count:   1,
		Timeout: 3 * time.Second,
		Silent:  true,
		Reporter: pingReporter(func(result ping.Result) {
			rtt = result.RTT
		}),
	}

	err := ping.Run(ctx, target, opts)
	if err != nil {
		return ports.PingResult{
			Target:  target,
			Success: false,
			Message: err.Error(),
		}, err
	}

	return ports.PingResult{
		Target:        target,
		LatencyMillis: rtt.Milliseconds(),
		Success:       true,
		Message:       "ok",
	}, nil
}

type pingReporter func(result ping.Result)

func (p pingReporter) Report(_ context.Context, _ net.Conn, result ping.Result) error {
	p(result)
	return nil
}
