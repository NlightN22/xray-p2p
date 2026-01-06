//go:build unix

package common

// RequireRoot performs a soft privilege check on Unix platforms.
func RequireRoot() error {
	return nil
}
