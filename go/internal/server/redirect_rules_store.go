//go:build linux || windows

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func openServerRedirectStore(installDir string) (serverRedirectStore, error) {
	path := serverStatePath(installDir)
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return serverRedirectStore{}, err
	}
	return buildServerRedirectStore(path, doc)
}

func openServerRedirectStoreFromPath(path string) (serverRedirectStore, error) {
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return serverRedirectStore{}, err
	}
	return buildServerRedirectStore(path, doc)
}

func buildServerRedirectStore(path string, doc map[string]any) (serverRedirectStore, error) {
	reverseState, err := decodeServerReverseState(doc)
	if err != nil {
		return serverRedirectStore{}, err
	}
	redirects, err := decodeServerRedirectRules(doc)
	if err != nil {
		return serverRedirectStore{}, err
	}
	return serverRedirectStore{
		path:      path,
		doc:       doc,
		reverse:   reverseState,
		redirects: redirects,
	}, nil
}

func decodeServerRedirectRules(doc map[string]any) ([]redirect.Rule, error) {
	raw := doc[serverRedirectRulesKey]
	if raw == nil {
		return []redirect.Rule{}, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode server redirect state: %w", err)
	}
	var rules []redirect.Rule
	if err := json.Unmarshal(buf, &rules); err != nil {
		return nil, fmt.Errorf("parse server redirect state: %w", err)
	}
	return rules, nil
}

func (s *serverRedirectStore) saveRedirects() error {
	if s.doc == nil {
		s.doc = make(map[string]any)
	}
	if len(s.redirects) == 0 {
		s.doc[serverRedirectRulesKey] = nil
	} else {
		s.doc[serverRedirectRulesKey] = s.redirects
	}
	return writeServerStateDoc(s.path, s.doc)
}

func (s *serverRedirectStore) removeRedirectsByTag(tag string) bool {
	updated, removed := redirect.RemoveRulesByTag(s.redirects, tag)
	if removed {
		s.redirects = updated
	}
	return removed
}

func (s serverRedirectStore) bindings() []redirect.Binding {
	if len(s.reverse) == 0 {
		return nil
	}
	result := make([]redirect.Binding, 0, len(s.reverse))
	for _, channel := range s.reverse {
		result = append(result, redirect.Binding{
			Tag:  channel.Tag,
			Host: channel.Host,
		})
	}
	return result
}

func resolveServerRedirectBinding(tag, host string, bindings []redirect.Binding) (redirect.Binding, error) {
	trimmedTag := strings.TrimSpace(tag)
	trimmedHost := strings.TrimSpace(host)
	if trimmedTag != "" {
		var matched redirect.Binding
		found := false
		for _, binding := range bindings {
			if strings.EqualFold(binding.Tag, trimmedTag) {
				matched = binding
				found = true
				break
			}
		}
		if !found {
			return redirect.Binding{}, fmt.Errorf("outbound tag %q is not registered", trimmedTag)
		}
		if trimmedHost != "" && !strings.EqualFold(strings.TrimSpace(matched.Host), trimmedHost) {
			resolvedHost := matched.Host
			if strings.TrimSpace(resolvedHost) == "" {
				resolvedHost = trimmedHost
			}
			return redirect.Binding{}, fmt.Errorf("tag %q does not match reverse host %q", trimmedTag, resolvedHost)
		}
		return matched, nil
	}

	binding, err := redirect.ResolveBinding(tag, host, bindings)
	if err != nil {
		switch {
		case errors.Is(err, redirect.ErrBindingNotSpecified):
			return redirect.Binding{}, errors.New("--tag or --host is required")
		case errors.Is(err, redirect.ErrBindingHostNotFound):
			return redirect.Binding{}, fmt.Errorf("reverse portal host %q not found", strings.TrimSpace(host))
		case errors.Is(err, redirect.ErrBindingTagNotFound):
			return redirect.Binding{}, fmt.Errorf("outbound tag %q is not registered", strings.TrimSpace(tag))
		case errors.Is(err, redirect.ErrBindingTagMismatch):
			resolvedHost := binding.Host
			if strings.TrimSpace(resolvedHost) == "" {
				resolvedHost = strings.TrimSpace(host)
			}
			return redirect.Binding{}, fmt.Errorf("tag %q does not match reverse host %q", tag, resolvedHost)
		default:
			return redirect.Binding{}, err
		}
	}
	return binding, nil
}
