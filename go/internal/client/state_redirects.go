package client

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func (s *clientInstallState) addRedirect(rule redirect.Rule) error {
	updated, err := redirect.AddRule(s.Redirects, rule)
	if err != nil {
		return err
	}
	s.Redirects = updated
	return nil
}

func (s *clientInstallState) removeRedirect(target redirect.Target, tagFilter string) ([]redirect.Rule, bool) {
	updated, removed := redirect.RemoveRule(s.Redirects, target, tagFilter)
	if removed {
		s.Redirects = updated
	}
	return updated, removed
}

func (s *clientInstallState) removeRedirectsByTag(tag string) {
	if len(s.Redirects) == 0 {
		return
	}
	lower := strings.ToLower(strings.TrimSpace(tag))
	if lower == "" {
		return
	}
	filtered := s.Redirects[:0]
	for _, rule := range s.Redirects {
		if strings.ToLower(strings.TrimSpace(rule.OutboundTag)) == lower {
			continue
		}
		filtered = append(filtered, rule)
	}
	s.Redirects = filtered
}
