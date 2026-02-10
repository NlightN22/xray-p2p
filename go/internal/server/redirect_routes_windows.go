//go:build windows

package server

import "github.com/NlightN22/xray-p2p/go/internal/redirect"

func applyRedirectRoutes(string, []redirect.Rule) error {
	return nil
}

func removeRedirectRoutes(string, []redirect.Rule) error {
	return nil
}
