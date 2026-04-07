//go:build !linux && !windows

package server

import "context"

// RunDeploy is not supported on this platform.
func RunDeploy(ctx context.Context, _ DeployRunOptions) error {
	return ErrUnsupported
}
