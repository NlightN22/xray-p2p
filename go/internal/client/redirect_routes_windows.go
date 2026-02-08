//go:build windows

package client

import "github.com/NlightN22/xray-p2p/go/internal/redirect"

func ensureRedirectRoute(string, string) error {
	return nil
}

func removeRedirectRouteIfUnused(string, string, []redirect.Rule) error {
	return nil
}

func applyRedirectRoutes(string, []redirect.Rule) error {
	return nil
}

func removeRedirectRoutes(string, []redirect.Rule) error {
	return nil
}
