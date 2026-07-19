//go:build !linux && !windows

package server

type serverReverseChannel struct {
	UserID string
	Host   string
	Tag    string
	Domain string
}

type serverReverseState map[string]serverReverseChannel

// DesiredReverseChannel exposes the persisted Desired contract to tooling.
type DesiredReverseChannel = serverReverseChannel

func (s *serverReverseState) ensure() {
	if s == nil || *s != nil {
		return
	}
	*s = make(serverReverseState)
}
