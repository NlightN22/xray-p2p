package client

import "strings"

func ensureObject(root map[string]any, key string) map[string]any {
	if raw, ok := root[key]; ok {
		if obj, ok := raw.(map[string]any); ok {
			return obj
		}
	}
	obj := make(map[string]any)
	root[key] = obj
	return obj
}

func extractRuleSlice(raw any) []any {
	if rules, ok := raw.([]any); ok {
		return rules
	}
	return []any{}
}

func extractStringSlice(raw any) []string {
	switch values := raw.(type) {
	case []string:
		result := make([]string, len(values))
		for i, item := range values {
			result[i] = strings.TrimSpace(item)
		}
		return result
	case []any:
		result := make([]string, 0, len(values))
		for _, item := range values {
			if str, ok := item.(string); ok {
				result = append(result, strings.TrimSpace(str))
			}
		}
		return result
	default:
		if str, ok := raw.(string); ok {
			return []string{strings.TrimSpace(str)}
		}
		return []string{}
	}
}
