//go:build windows

package client

import (
	"errors"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func applyRedirectRoutes(tunName, tunAddr string, redirects []redirect.Rule) error {
	cidrs := collectRedirectCIDRs(redirects)
	if err := winnet.SyncRedirectRoutes(tunName, tunAddr, cidrs); err != nil {
		if errors.Is(err, winnet.ErrInterfaceMissing) || errors.Is(err, winnet.ErrTunIPv4Missing) {
			logging.Warn("xp2p: redirect route setup skipped", "interface", strings.TrimSpace(tunName), "err", err)
			return nil
		}
		return err
	}
	logging.Info("xp2p: redirect routes applied", "interface", strings.TrimSpace(tunName), "count", len(cidrs))
	return nil
}

func removeRedirectRoutes(tunName, _ string, redirects []redirect.Rule) error {
	cidrs := collectRedirectCIDRs(redirects)
	if err := winnet.RemoveRedirectRoutes(tunName, cidrs); err != nil {
		if errors.Is(err, winnet.ErrInterfaceMissing) {
			return nil
		}
		return err
	}
	return nil
}

func collectRedirectCIDRs(redirects []redirect.Rule) []string {
	seen := make(map[string]struct{}, len(redirects))
	out := make([]string, 0, len(redirects))
	for _, rule := range redirects {
		if rule.Kind() != redirect.KindCIDR {
			continue
		}
		if rule.NoRoutes {
			continue
		}
		value := strings.TrimSpace(rule.Value())
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
