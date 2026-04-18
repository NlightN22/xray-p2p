//go:build windows || linux

package server

import (
	"strings"
)

func ensureReversePortal(doc map[string]any, channel serverReverseChannel) bool {
	reverse := ensureObject(doc, "reverse")
	portals := extractInterfaceSlice(reverse["portals"])
	lowerTag := strings.ToLower(channel.Tag)
	lowerDomain := strings.ToLower(channel.Domain)
	filtered := make([]any, 0, len(portals))
	replaced := false
	changed := false
	for _, raw := range portals {
		entry, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		tag, _ := entry["tag"].(string)
		domain, _ := entry["domain"].(string)
		if strings.ToLower(strings.TrimSpace(tag)) == lowerTag || strings.ToLower(strings.TrimSpace(domain)) == lowerDomain {
			if replaced {
				changed = true
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(tag), channel.Tag) || !strings.EqualFold(strings.TrimSpace(domain), channel.Domain) {
				changed = true
			}
			filtered = append(filtered, map[string]any{
				"domain": channel.Domain,
				"tag":    channel.Tag,
			})
			replaced = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !replaced {
		filtered = append(filtered, map[string]any{
			"domain": channel.Domain,
			"tag":    channel.Tag,
		})
		changed = true
	}
	reverse["portals"] = filtered
	return changed
}

func removeReversePortal(doc map[string]any, channel serverReverseChannel) bool {
	reverse := ensureObject(doc, "reverse")
	portals := extractInterfaceSlice(reverse["portals"])
	lowerTag := strings.ToLower(channel.Tag)
	lowerDomain := strings.ToLower(channel.Domain)
	filtered := make([]any, 0, len(portals))
	changed := false
	for _, raw := range portals {
		entry, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		tag, _ := entry["tag"].(string)
		domain, _ := entry["domain"].(string)
		if strings.ToLower(strings.TrimSpace(tag)) == lowerTag || strings.ToLower(strings.TrimSpace(domain)) == lowerDomain {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if changed {
		reverse["portals"] = filtered
	}
	return changed
}
