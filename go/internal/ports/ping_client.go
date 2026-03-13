package ports

import "context"

type PingResult struct {
	Target        string
	LatencyMillis int64
	Success       bool
	Message       string
}

type PingClient interface {
	Ping(ctx context.Context, target string) (PingResult, error)
}
