//go:build !linux && !windows

package server

import "github.com/NlightN22/xray-p2p/go/internal/redirect"

func decodeServerRedirectRules(map[string]any) ([]redirect.Rule, error) {
	return nil, nil
}
