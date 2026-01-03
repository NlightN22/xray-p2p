//go:build linux

package client

import "context"

// RunService launches the managed client service loop on Linux.
func RunService(ctx context.Context, opts ServiceOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return runClientServiceCommon(ctx, opts)
}
