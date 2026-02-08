//go:build !linux && !windows

package server

import "github.com/NlightN22/xray-p2p/go/internal/layout"

func serverStatePath(string) string {
	return layout.ServerConfigFileName
}

func loadServerStateDoc(string) (map[string]any, error) {
	state := map[string]any{}
	return state, nil
}

func writeServerStateDoc(string, map[string]any) error {
	return nil
}

func decodeServerReverseState(map[string]any) (serverReverseState, error) {
	state := serverReverseState{}
	state.ensure()
	return state, nil
}
