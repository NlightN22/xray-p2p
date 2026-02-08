//go:build windows || linux

package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

func addServerForwardInbound(configDir string, rule forward.Rule) error {
	path := filepath.Join(configDir, "inbounds.json")
	root, err := loadServerInbounds(path)
	if err != nil {
		return err
	}
	entries, err := extractServerInbounds(root)
	if err != nil {
		return err
	}
	filtered := make([]any, 0, len(entries)+1)
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		tag, _ := entry["tag"].(string)
		remark, _ := entry["remark"].(string)
		if strings.EqualFold(tag, rule.Tag) || strings.EqualFold(remark, rule.Remark) {
			continue
		}
		filtered = append(filtered, entry)
	}
	filtered = append(filtered, rule.InboundMap())
	root["inbounds"] = filtered
	return writeServerInbounds(path, root)
}

func removeServerForwardInbound(configDir string, rule forward.Rule) error {
	path := filepath.Join(configDir, "inbounds.json")
	root, err := loadServerInbounds(path)
	if err != nil {
		return err
	}
	entries, err := extractServerInbounds(root)
	if err != nil {
		return err
	}
	filtered := make([]any, 0, len(entries))
	removed := false
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		tag, _ := entry["tag"].(string)
		remark, _ := entry["remark"].(string)
		if strings.EqualFold(tag, rule.Tag) || strings.EqualFold(remark, rule.Remark) {
			removed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !removed {
		return fmt.Errorf("xp2p: forward inbound %s not found", rule.Tag)
	}
	root["inbounds"] = filtered
	return writeServerInbounds(path, root)
}

func loadServerInbounds(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("xp2p: read %s: %w", path, err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("xp2p: parse %s: %w", path, err)
	}
	return root, nil
}

func writeServerInbounds(path string, root map[string]any) error {
	data, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return fmt.Errorf("xp2p: encode %s: %w", path, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write %s: %w", path, err)
	}
	return nil
}

func extractServerInbounds(root map[string]any) ([]any, error) {
	raw, ok := root["inbounds"]
	if !ok {
		return nil, fmt.Errorf("xp2p: inbounds.json missing \"inbounds\" array")
	}
	entries, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("xp2p: inbounds.json has invalid \"inbounds\" array")
	}
	return entries, nil
}

func syncServerForwardInbounds(configDir string, rules []forward.Rule) error {
	path := filepath.Join(configDir, "inbounds.json")
	root, err := loadServerInbounds(path)
	if err != nil {
		return err
	}
	entries, err := extractServerInbounds(root)
	if err != nil {
		return err
	}

	desiredByTag := make(map[string]forward.Rule, len(rules))
	desiredByRemark := make(map[string]forward.Rule, len(rules))
	remaining := make(map[string]forward.Rule, len(rules))
	for _, rule := range rules {
		tagKey := strings.ToLower(strings.TrimSpace(rule.Tag))
		remarkKey := strings.ToLower(strings.TrimSpace(rule.Remark))
		if tagKey != "" {
			desiredByTag[tagKey] = rule
			remaining[tagKey] = rule
		}
		if remarkKey != "" {
			desiredByRemark[remarkKey] = rule
		}
	}

	filtered := make([]any, 0, len(entries))
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		if !isForwardInbound(entry) {
			filtered = append(filtered, entry)
			continue
		}

		tag, _ := entry["tag"].(string)
		remark, _ := entry["remark"].(string)
		tagKey := strings.ToLower(strings.TrimSpace(tag))
		remarkKey := strings.ToLower(strings.TrimSpace(remark))

		if rule, ok := desiredByTag[tagKey]; ok {
			filtered = append(filtered, rule.InboundMap())
			delete(remaining, tagKey)
			continue
		}
		if rule, ok := desiredByRemark[remarkKey]; ok {
			filtered = append(filtered, rule.InboundMap())
			tagKey = strings.ToLower(strings.TrimSpace(rule.Tag))
			delete(remaining, tagKey)
			continue
		}
	}

	for _, rule := range remaining {
		filtered = append(filtered, rule.InboundMap())
	}
	root["inbounds"] = filtered
	return writeServerInbounds(path, root)
}

func isForwardInbound(entry map[string]any) bool {
	proto, _ := entry["protocol"].(string)
	if !strings.EqualFold(strings.TrimSpace(proto), "dokodemo-door") {
		return false
	}
	remark, _ := entry["remark"].(string)
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(remark)), "forward:") {
		return true
	}
	tag, _ := entry["tag"].(string)
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(tag)), "in_")
}
