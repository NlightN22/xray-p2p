//go:build linux || windows

package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

const serverForwardRulesKey = "forward_rules"

type serverForwardStore struct {
	path      string
	doc       map[string]any
	forwards  []forward.Rule
	redirects []redirect.Rule
}

func openServerForwardStore(installDir string) (serverForwardStore, error) {
	path := serverStatePath(installDir)
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return serverForwardStore{}, err
	}
	return buildServerForwardStore(path, doc)
}

func openServerForwardStoreFromPath(path string) (serverForwardStore, error) {
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return serverForwardStore{}, err
	}
	return buildServerForwardStore(path, doc)
}

func buildServerForwardStore(path string, doc map[string]any) (serverForwardStore, error) {
	forwards, err := decodeServerForwardRules(doc)
	if err != nil {
		return serverForwardStore{}, err
	}
	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		return serverForwardStore{}, err
	}
	return serverForwardStore{
		path:      path,
		doc:       doc,
		forwards:  forwards,
		redirects: redirects,
	}, nil
}

func decodeServerForwardRules(doc map[string]any) ([]forward.Rule, error) {
	raw := doc[serverForwardRulesKey]
	if raw == nil {
		return []forward.Rule{}, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode server forward state: %w", err)
	}
	var rules []forward.Rule
	if err := json.Unmarshal(buf, &rules); err != nil {
		return nil, fmt.Errorf("parse server forward state: %w", err)
	}
	return rules, nil
}

func (s *serverForwardStore) saveForwards() error {
	if len(s.forwards) == 0 {
		s.doc[serverForwardRulesKey] = nil
	} else {
		s.doc[serverForwardRulesKey] = s.forwards
	}
	return writeServerStateDoc(s.path, s.doc)
}

func (s *serverForwardStore) add(rule forward.Rule) error {
	for _, existing := range s.forwards {
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
	s.forwards = append(s.forwards, rule)
	return nil
}

func (s *serverForwardStore) remove(selector forward.Selector) (forward.Rule, int, bool) {
	if len(s.forwards) == 0 {
		return forward.Rule{}, -1, false
	}
	idx := -1
	for i, rule := range s.forwards {
		if selector.Matches(rule) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return forward.Rule{}, -1, false
	}
	removed := s.forwards[idx]
	s.forwards = append(s.forwards[:idx], s.forwards[idx+1:]...)
	return removed, idx, true
}

func (s *serverForwardStore) insertAt(rule forward.Rule, idx int) {
	if idx < 0 || idx > len(s.forwards) {
		s.forwards = append(s.forwards, rule)
		return
	}
	s.forwards = append(s.forwards[:idx], append([]forward.Rule{rule}, s.forwards[idx:]...)...)
}
