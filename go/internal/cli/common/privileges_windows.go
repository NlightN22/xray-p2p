//go:build windows

package common

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// RequireRoot verifies the caller is running with elevated privileges.
func RequireRoot() error {
	admin, err := currentUserIsAdmin()
	if err != nil {
		return fmt.Errorf("xp2p: check administrative privileges: %w", err)
	}
	if !admin {
		return errRootRequired
	}
	return nil
}

func currentUserIsAdmin() (bool, error) {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false, err
	}
	defer token.Close()

	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return false, err
	}
	member, err := token.IsMember(adminSID)
	if err != nil {
		return false, err
	}
	return member, nil
}
