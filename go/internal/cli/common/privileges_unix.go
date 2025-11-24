//go:build unix

package common

import (
	"errors"
	"os"
)

var errRootRequired = errors.New("xp2p: administrative privileges required")

// RequireRoot verifies the caller is running with elevated privileges.
func RequireRoot() error {
	if os.Geteuid() != 0 {
		return errRootRequired
	}
	return nil
}
