package client

import "strings"

const (
	directRandomTagWindows = "direct-random"
	directUDPTagWindows    = "direct-udp"
)

func directRandomTag() string {
	return "direct"
}

func filterWindowsDirectRules(rules []any) []any {
	filtered := make([]any, 0, len(rules))
	for _, raw := range rules {
		ruleMap, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		if isManagedWindowsDirectRule(ruleMap) {
			continue
		}
		filtered = append(filtered, ruleMap)
	}
	return filtered
}

func windowsDirectRules() []any {
	return []any{
		map[string]any{
			"type":        "field",
			"network":     "udp",
			"outboundTag": directUDPTagWindows,
		},
		map[string]any{
			"type":        "field",
			"network":     "tcp,udp",
			"outboundTag": directRandomTagWindows,
		},
	}
}

func isManagedWindowsDirectRule(rule map[string]any) bool {
	typ, _ := rule["type"].(string)
	if !strings.EqualFold(strings.TrimSpace(typ), "field") {
		return false
	}
	outbound, _ := rule["outboundTag"].(string)
	trimmed := strings.ToLower(strings.TrimSpace(outbound))
	if trimmed != directRandomTagWindows && trimmed != directUDPTagWindows {
		return false
	}
	network, _ := rule["network"].(string)
	network = strings.ToLower(strings.TrimSpace(network))
	if network == "" {
		return false
	}
	return strings.Contains(network, "udp")
}
