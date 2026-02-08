//go:build linux

package server

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/linuxnet"
	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/openwrt"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func ensureRedirectRoute(tunName, cidr string) error {
	tun := strings.TrimSpace(tunName)
	if tun == "" {
		return nil
	}
	if err := openwrt.EnsureTunRoute(tun, cidr); err != nil {
		return err
	}
	return linuxnet.EnsureRoute(tun, cidr)
}

func removeRedirectRoute(tunName, cidr string) error {
	tun := strings.TrimSpace(tunName)
	if tun == "" {
		return nil
	}
	if err := openwrt.RemoveTunRoute(tun, cidr); err != nil {
		if linuxnet.IsTunPermissionError(err) {
			logging.Warn("xp2p: redirect route cleanup skipped (permission denied)", "cidr", cidr, "err", err)
			return nil
		}
		return err
	}
	if err := linuxnet.RemoveRoute(tun, cidr); err != nil {
		if linuxnet.IsTunPermissionError(err) {
			logging.Warn("xp2p: redirect route cleanup skipped (permission denied)", "cidr", cidr, "err", err)
			return nil
		}
		return err
	}
	return nil
}

func removeRedirectRouteIfUnused(tunName, cidr string, redirects []redirect.Rule) error {
	if hasRedirectCIDR(cidr, redirects) {
		return nil
	}
	return removeRedirectRoute(tunName, cidr)
}

func applyRedirectRoutes(tunName string, redirects []redirect.Rule) error {
	seen := make(map[string]struct{}, len(redirects))
	for _, rule := range redirects {
		if rule.Kind() != redirect.KindCIDR {
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
		if err := ensureRedirectRoute(tunName, value); err != nil {
			if linuxnet.IsTunPermissionError(err) {
				logging.Warn("xp2p: redirect route setup skipped (permission denied)", "cidr", value, "err", err)
				continue
			}
			return err
		}
	}
	return nil
}

func removeRedirectRoutes(tunName string, redirects []redirect.Rule) error {
	seen := make(map[string]struct{}, len(redirects))
	for _, rule := range redirects {
		if rule.Kind() != redirect.KindCIDR {
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
		if err := removeRedirectRoute(tunName, value); err != nil {
			return err
		}
	}
	return nil
}

func hasRedirectCIDR(cidr string, redirects []redirect.Rule) bool {
	trimmed := strings.TrimSpace(cidr)
	if trimmed == "" {
		return false
	}
	for _, rule := range redirects {
		if rule.Kind() != redirect.KindCIDR {
			continue
		}
		if strings.EqualFold(rule.Value(), trimmed) {
			return true
		}
	}
	return false
}
