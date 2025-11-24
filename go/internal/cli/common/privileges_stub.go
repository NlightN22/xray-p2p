//go:build !unix

package common

// RequireRoot is a no-op on non-Unix platforms.
func RequireRoot() error {
	return nil
}
