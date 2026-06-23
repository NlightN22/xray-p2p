package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

// mergeHAOwnedRedirects replaces only entries previously published by HA. All
// user-managed rules remain in their original order.
func mergeHAOwnedRedirects(doc map[string]any, payload []byte) ([]redirect.Rule, error) {
	current, err := decodeServerRedirectRules(doc)
	if err != nil {
		return nil, err
	}
	owned, err := decodeHARedirectKeys(doc)
	if err != nil {
		return nil, err
	}
	kept := make([]redirect.Rule, 0, len(current))
	for _, rule := range current {
		if _, ok := owned[haRedirectKey(rule)]; !ok {
			kept = append(kept, rule)
		}
	}
	candidate, err := decodeHARedirectPayload(payload)
	if err != nil {
		return nil, err
	}
	return append(kept, candidate...), nil
}

func redirectKeysFromPayload(payload []byte) []string {
	rules, err := decodeHARedirectPayload(payload)
	if err != nil {
		return nil
	}
	keys := make([]string, 0, len(rules))
	for _, rule := range rules {
		keys = append(keys, haRedirectKey(rule))
	}
	return keys
}

func decodeHARedirectPayload(payload []byte) ([]redirect.Rule, error) {
	if len(payload) == 0 {
		return []redirect.Rule{}, nil
	}
	var rules []redirect.Rule
	if err := json.Unmarshal(payload, &rules); err != nil {
		return nil, fmt.Errorf("parse HA redirects: %w", err)
	}
	return rules, nil
}

func decodeHARedirectKeys(doc map[string]any) (map[string]struct{}, error) {
	keys := make(map[string]struct{})
	raw := doc[serverHARedirectKeysKey]
	if raw == nil {
		return keys, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode HA redirect keys: %w", err)
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, fmt.Errorf("parse HA redirect keys: %w", err)
	}
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			keys[value] = struct{}{}
		}
	}
	return keys, nil
}

func haRedirectKey(rule redirect.Rule) string {
	return strings.ToLower(strings.TrimSpace(rule.OutboundTag)) + "\x00" +
		strings.ToLower(strings.TrimSpace(rule.Domain)) + "\x00" + strings.TrimSpace(rule.CIDR)
}
