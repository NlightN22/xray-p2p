//go:build !unix && !windows

package common

// RequireRoot is a no-op on non-Unix platforms.
func RequireRoot() error {
	return nil
}
