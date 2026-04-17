package extensions

import (
	"fmt"
	"strings"
)

func ValidateAppendTags(kind string, items []any, managedTags map[string]struct{}) error {
	for idx, raw := range items {
		obj, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("extensions: %s[%d] is not an object", kind, idx)
		}
		tagRaw, ok := obj["tag"]
		if !ok {
			return fmt.Errorf("extensions: %s[%d] missing tag", kind, idx)
		}
		tag, ok := tagRaw.(string)
		if !ok {
			return fmt.Errorf("extensions: %s[%d] has non-string tag", kind, idx)
		}
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return fmt.Errorf("extensions: %s[%d] has empty tag", kind, idx)
		}
		lower := strings.ToLower(tag)
		if isReservedTag(lower) {
			return fmt.Errorf("extensions: %s[%d] tag %q uses reserved namespace", kind, idx, tag)
		}
		if _, exists := managedTags[lower]; exists {
			return fmt.Errorf("extensions: %s[%d] tag %q collides with managed tag", kind, idx, tag)
		}
		managedTags[lower] = struct{}{}
	}
	return nil
}

func isReservedTag(lower string) bool {
	lower = strings.ToLower(strings.TrimSpace(lower))
	switch {
	case strings.HasPrefix(lower, "xp2p-"):
		return true
	case strings.HasPrefix(lower, "proxy-"):
		return true
	case strings.HasSuffix(lower, ".rev"):
		return true
	default:
		return false
	}
}
