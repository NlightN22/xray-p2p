//go:build windows || linux

package server

const serverReverseStateKey = "reverse_channels"

type serverReverseChannel struct {
	UserID string `json:"user_id" toml:"user_id"`
	Host   string `json:"host" toml:"host"`
	Tag    string `json:"tag" toml:"tag"`
	Domain string `json:"domain" toml:"domain"`
}

type serverReverseState map[string]serverReverseChannel

func (s *serverReverseState) ensure() {
	if *s == nil {
		*s = make(serverReverseState)
	}
}
