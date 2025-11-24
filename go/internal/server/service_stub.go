//go:build !linux

package server

import "context"

// RunService is unavailable on non-Linux platforms.
func RunService(_ context.Context, _ ServiceOptions) error {
	return ErrServiceUnsupported
}
