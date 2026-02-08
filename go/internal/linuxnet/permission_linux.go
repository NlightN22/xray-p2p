//go:build linux

package linuxnet

import "strings"

// IsTunPermissionError reports whether the error suggests missing privileges for TUN setup.
func IsTunPermissionError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "operation not permitted") || strings.Contains(lower, "permission denied")
}
