//go:build !linux && !windows

package server

func decodeServerReverseState(map[string]any) (serverReverseState, error) {
	state := serverReverseState{}
	state.ensure()
	return state, nil
}
