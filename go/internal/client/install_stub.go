//go:build !linux && !windows

package client

import "fmt"

func resolveInstallDir(_ string) (string, error) {
	return "", fmt.Errorf("xp2p: client install is not supported on this platform")
}

func ResolveConfigDir(_, _ string) (string, error) {
	return "", fmt.Errorf("xp2p: client install is not supported on this platform")
}
