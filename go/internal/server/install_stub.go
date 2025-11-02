//go:build !windows

package server

import "context"

// Install is not supported on non-Windows platforms.
func Install(_ context.Context, _ InstallOptions) error {
	return ErrUnsupported
}

// Remove is not supported on non-Windows platforms.
func Remove(_ context.Context, _ RemoveOptions) error {
	return ErrUnsupported
}
