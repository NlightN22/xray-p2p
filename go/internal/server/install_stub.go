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

// SetCertificate is not supported on non-Windows platforms.
func SetCertificate(_ context.Context, _ CertificateOptions) error {
	return ErrUnsupported
}

// ListUsers is not supported on non-Windows platforms.
func ListUsers(_ context.Context, _ ListUsersOptions) ([]UserLink, error) {
	return nil, ErrUnsupported
}

// UserLink is not supported on non-Windows platforms.
func GetUserLink(_ context.Context, _ UserLinkOptions) (UserLink, error) {
	return UserLink{}, ErrUnsupported
}
