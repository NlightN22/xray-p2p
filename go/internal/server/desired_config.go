package server

import (
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

type desiredServerConfig struct {
	Reverse   serverReverseState
	Redirects []redirect.Rule
	Forwards  []forward.Rule
}

func loadServerDesiredConfig(installDir string) (desiredServerConfig, error) {
	doc, err := loadServerStateDoc(serverStatePath(installDir))
	if err != nil {
		return desiredServerConfig{}, err
	}
	reverse, err := decodeServerReverseState(doc)
	if err != nil {
		return desiredServerConfig{}, err
	}
	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		return desiredServerConfig{}, err
	}
	forwards, err := decodeServerForwardRules(doc)
	if err != nil {
		return desiredServerConfig{}, err
	}
	return desiredServerConfig{
		Reverse:   reverse,
		Redirects: redirects,
		Forwards:  forwards,
	}, nil
}
