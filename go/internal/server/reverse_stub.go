//go:build !linux && !windows

package server

type serverReverseChannel struct {
	UserID string
	Host   string
	Tag    string
	Domain string
}

type serverReverseState map[string]serverReverseChannel
