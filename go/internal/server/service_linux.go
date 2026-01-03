//go:build linux

package server

import "context"

// RunService launches the managed server service loop on Linux.
func RunService(ctx context.Context, opts ServiceOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return runServerServiceCommon(ctx, opts)
}
