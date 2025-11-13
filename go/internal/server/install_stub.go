//go:build !windows && !linux

package server

import "context"

// Install is not supported on this platform.
func Install(_ context.Context, _ InstallOptions) error {
	return ErrUnsupported
}

// Remove is not supported on this platform.
func Remove(_ context.Context, _ RemoveOptions) error {
	return ErrUnsupported
}

// Run is not supported on this platform.
func Run(_ context.Context, _ RunOptions) error {
	return ErrUnsupported
}

// AddUser is not supported on this platform.
func AddUser(_ context.Context, _ AddUserOptions) error {
	return ErrUnsupported
}

// RemoveUser is not supported on this platform.
func RemoveUser(_ context.Context, _ RemoveUserOptions) error {
	return ErrUnsupported
}

// SetCertificate is not supported on this platform.
func SetCertificate(_ context.Context, _ CertificateOptions) error {
	return ErrUnsupported
}

// ListUsers is not supported on this platform.
func ListUsers(_ context.Context, _ ListUsersOptions) ([]UserLink, error) {
	return nil, ErrUnsupported
}

// UserLink is not supported on this platform.
func GetUserLink(_ context.Context, _ UserLinkOptions) (UserLink, error) {
	return UserLink{}, ErrUnsupported
}
