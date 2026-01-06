//go:build windows

package common

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// RequireRoot verifies the caller is running with elevated privileges.
func RequireRoot() error {
	admin, err := currentUserIsAdmin()
	if err != nil {
		return nil
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

	if user, err := token.GetTokenUser(); err == nil {
		if systemSID, sidErr := windows.CreateWellKnownSid(windows.WinLocalSystemSid); sidErr == nil {
			if windows.EqualSid(user.User.Sid, systemSID) {
				return true, nil
			}
		}
	}

	var isElevated uint32
	var outLen uint32
	err := windows.GetTokenInformation(token, windows.TokenElevation, (*byte)(unsafe.Pointer(&isElevated)), uint32(unsafe.Sizeof(isElevated)), &outLen)
	if err != nil {
		return false, err
	}
	if outLen != uint32(unsafe.Sizeof(isElevated)) {
		return false, fmt.Errorf("unexpected token elevation size: %d", outLen)
	}
	return isElevated != 0, nil
}
