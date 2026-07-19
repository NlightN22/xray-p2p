//go:build windows || linux

package server

const serverReverseStateKey = "reverse_channels"

type serverReverseChannel struct {
	UserID   string `json:"user_id" toml:"user_id"`
	Host     string `json:"host" toml:"host"`
	Tag      string `json:"tag" toml:"tag"`
	Domain   string `json:"domain" toml:"domain"`
	Disabled bool   `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

type serverReverseState map[string]serverReverseChannel

// DesiredReverseChannel exposes the persisted Desired contract to tooling.
type DesiredReverseChannel = serverReverseChannel

func (s *serverReverseState) ensure() {
	if *s == nil {
		*s = make(serverReverseState)
	}
}
