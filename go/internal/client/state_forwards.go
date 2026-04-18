package client

import (
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

func (s *clientInstallState) addForward(rule forward.Rule) error {
	s.normalize()
	for _, existing := range s.Forwards {
		if existing.ListenPort == rule.ListenPort {
			return fmt.Errorf("forward listener on port %d already exists", rule.ListenPort)
		}
		if strings.EqualFold(existing.Tag, rule.Tag) {
			return fmt.Errorf("forward tag %s already exists", rule.Tag)
		}
		if strings.EqualFold(existing.Remark, rule.Remark) {
			return fmt.Errorf("forward remark %s already exists", rule.Remark)
		}
	}
	s.Forwards = append(s.Forwards, rule)
	return nil
}

func (s *clientInstallState) removeForward(filter forward.Selector) (forward.Rule, int, bool) {
	if len(s.Forwards) == 0 {
		return forward.Rule{}, -1, false
	}
	idx := -1
	for i, rule := range s.Forwards {
		if filter.Matches(rule) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return forward.Rule{}, -1, false
	}
	removed := s.Forwards[idx]
	s.Forwards = append(s.Forwards[:idx], s.Forwards[idx+1:]...)
	return removed, idx, true
}

func (s *clientInstallState) insertForwardAt(rule forward.Rule, idx int) {
	if idx < 0 || idx > len(s.Forwards) {
		s.Forwards = append(s.Forwards, rule)
		return
	}
	s.Forwards = append(s.Forwards[:idx], append([]forward.Rule{rule}, s.Forwards[idx:]...)...)
}
