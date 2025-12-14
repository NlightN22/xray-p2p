//go:build !linux && !windows

package server

import "errors"

func hasServerReversePortal(map[string]any, serverReverseChannel) bool {
	return false
}

func hasServerReverseRule(map[string]any, serverReverseChannel) bool {
	return false
}

func loadServerRouting(path string) (map[string]any, error) {
	return nil, errors.New("xp2p: server routing unavailable on this platform")
}
