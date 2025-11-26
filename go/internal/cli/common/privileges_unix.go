//go:build unix

package common

import "os"

// RequireRoot verifies the caller is running with elevated privileges.
func RequireRoot() error {
	if os.Geteuid() != 0 {
		return errRootRequired
	}
	return nil
}
