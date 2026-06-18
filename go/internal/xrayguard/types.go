package xrayguard

import (
	"context"
	"fmt"
	"time"
)

const ReasonFDSpike = "fd_socket_spike"

type Sample struct {
	Timestamp           time.Time
	FDCount             int
	SocketFDCount       int
	EstablishedTCPCount int
}

type Event struct {
	Reason              string
	PID                 int
	Before              Sample
	After               Sample
	Window              time.Duration
	FDDelta             int
	SocketRatioPercent  int
	EstablishedTCPCount int
	Action              string
}

func (e Event) Error() string {
	return fmt.Sprintf(
		"xray-core quarantined: reason=%s pid=%d fd_before=%d fd_after=%d fd_delta=%d window=%s socket_ratio=%d established_tcp=%d action=%s",
		e.Reason,
		e.PID,
		e.Before.FDCount,
		e.After.FDCount,
		e.FDDelta,
		e.Window.Truncate(time.Millisecond),
		e.SocketRatioPercent,
		e.EstablishedTCPCount,
		e.Action,
	)
}

type Collector interface {
	Sample(ctx context.Context, pid int) (Sample, error)
}

type Options struct {
	Interval          time.Duration
	Window            time.Duration
	MinFDDelta        int
	MinFDCount        int
	MinSocketRatio    float64
	MaxEstablishedTCP int
	Action            string
	Collector         Collector
}

func DefaultOptions() Options {
	return Options{
		Interval:          time.Second,
		Window:            3 * time.Second,
		MinFDDelta:        512,
		MinFDCount:        1024,
		MinSocketRatio:    0.80,
		MaxEstablishedTCP: 200,
		Action:            "kill_xray",
		Collector:         DefaultCollector(),
	}
}
