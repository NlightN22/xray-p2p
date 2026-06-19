package server

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/xrayrule"
)

func applyWindowsDirectRules(rules []any) []any {
	filtered := make([]any, 0, len(rules)+2)
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
	return append(filtered, windowsDirectRules()...)
}

func windowsDirectRules() []any {
	return []any{
		map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.WindowsDirect("server", directUDPTagWindows, "udp"),
			"network":     "udp",
			"outboundTag": directUDPTagWindows,
		},
		map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.WindowsDirect("server", directRandomTagWindows, "tcp,udp"),
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
