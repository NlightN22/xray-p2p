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

// Run is not supported on non-Windows platforms.
func Run(_ context.Context, _ RunOptions) error {
	return ErrUnsupported
}

// AddUser is not supported on non-Windows platforms.
func AddUser(_ context.Context, _ AddUserOptions) error {
	return ErrUnsupported
}

// RemoveUser is not supported on non-Windows platforms.
func RemoveUser(_ context.Context, _ RemoveUserOptions) error {
	return ErrUnsupported
}
