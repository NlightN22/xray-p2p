package xrayapi

import (
	"context"
	"fmt"
	"time"

	observatorycommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/observatorycommand"
)

type OutboundObservation struct {
	Tag             string
	Alive           bool
	DelayMillis     int64
	LastError       string
	LastSeenUnix    int64
	LastTryUnix     int64
	HealthAll       int64
	HealthFail      int64
	HealthAverageMs int64
}

type ObservatoryOptions struct {
	Address string
	Timeout time.Duration
	Dialer  Dialer
}

func GetOutboundStatuses(ctx context.Context, opts ObservatoryOptions) ([]OutboundObservation, error) {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return nil, fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()
	return client.GetOutboundStatuses(ctx)
}

func (c *Client) GetOutboundStatuses(ctx context.Context) ([]OutboundObservation, error) {
	response, err := c.obs.GetOutboundStatus(ctx, &observatorycommand.GetOutboundStatusRequest{})
	if err != nil {
		return nil, fmt.Errorf("get outbound status: %w", err)
	}
	status := response.GetStatus()
	items := status.GetStatus()
	result := make([]OutboundObservation, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		health := item.GetHealthPing()
		result = append(result, OutboundObservation{
			Tag:             item.GetOutboundTag(),
			Alive:           item.GetAlive(),
			DelayMillis:     item.GetDelay(),
			LastError:       item.GetLastErrorReason(),
			LastSeenUnix:    item.GetLastSeenTime(),
			LastTryUnix:     item.GetLastTryTime(),
			HealthAll:       health.GetAll(),
			HealthFail:      health.GetFail(),
			HealthAverageMs: health.GetAverage(),
		})
	}
	return result, nil
}
