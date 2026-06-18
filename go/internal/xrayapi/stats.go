package xrayapi

import (
	"context"
	"fmt"
	"time"

	statscommand "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/statscommand"
)

type Stat struct {
	Name  string
	Value int64
}

type StatsQueryOptions struct {
	Address string
	Pattern string
	Reset   bool
	Timeout time.Duration
	Dialer  Dialer
}

func QueryStats(ctx context.Context, opts StatsQueryOptions) ([]Stat, error) {
	client, err := DialWith(ctx, opts.Address, opts.Timeout, opts.Dialer)
	if err != nil {
		return nil, fmt.Errorf("connect xray API: %w", err)
	}
	defer client.Close()

	queryCtx, cancel := context.WithTimeout(ctx, timeoutOrDefault(opts.Timeout))
	defer cancel()

	response, err := client.stats.QueryStats(queryCtx, &statscommand.QueryStatsRequest{
		Pattern: opts.Pattern,
		Reset_:  opts.Reset,
	})
	if err != nil {
		return nil, fmt.Errorf("query xray stats: %w", err)
	}
	stats := make([]Stat, 0, len(response.GetStat()))
	for _, item := range response.GetStat() {
		if item == nil {
			continue
		}
		stats = append(stats, Stat{
			Name:  item.GetName(),
			Value: item.GetValue(),
		})
	}
	return stats, nil
}

func timeoutOrDefault(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	return DefaultTimeout
}
