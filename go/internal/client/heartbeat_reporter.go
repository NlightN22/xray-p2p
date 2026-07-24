package client

import (
	"context"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/diagnostics/ping"
)

type heartbeatPingReporter struct {
	rtt time.Duration
}

func (r *heartbeatPingReporter) Report(_ context.Context, result ping.Result) error {
	r.rtt = result.RTT
	return nil
}

func (r heartbeatPingReporter) rttMillis() int64 {
	millis := r.rtt.Milliseconds()
	if millis <= 0 && r.rtt > 0 {
		return 1
	}
	return millis
}
