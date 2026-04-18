//go:build windows || linux

package server

import (
	"reflect"
	"runtime"
	"strings"
)

func ensureReverseRule(doc map[string]any, channel serverReverseChannel) bool {
	routing := ensureObject(doc, "routing")
	rules := extractInterfaceSlice(routing["rules"])
	lowerTag := strings.ToLower(channel.Tag)
	targetDomain := "full:" + strings.ToLower(channel.Domain)
	trimmedUser := strings.TrimSpace(channel.UserID)

	filtered := make([]any, 0, len(rules))
	kept := false
	changed := false
	for _, raw := range rules {
		ruleMap, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		if !ruleTargetsChannel(ruleMap, lowerTag, targetDomain) {
			filtered = append(filtered, ruleMap)
			continue
		}
		if !kept && reverseRuleMatches(ruleMap, channel, trimmedUser) {
			filtered = append(filtered, ruleMap)
		} else {
			changed = true
		}
		kept = true
	}
	if !kept {
		changed = true
		filtered = append(filtered, desiredReverseRule(channel, trimmedUser))
	}
	if runtime.GOOS == "windows" {
		updated := applyWindowsDirectRules(filtered)
		if !reflect.DeepEqual(filtered, updated) {
			changed = true
			filtered = updated
		}
	}
	routing["rules"] = filtered
	return changed
}

func removeReverseRules(doc map[string]any, channel serverReverseChannel) bool {
	routing := ensureObject(doc, "routing")
	rules := extractInterfaceSlice(routing["rules"])
	lowerTag := strings.ToLower(channel.Tag)
	targetDomain := "full:" + strings.ToLower(channel.Domain)
	filtered := make([]any, 0, len(rules))
	changed := false
	for _, raw := range rules {
		ruleMap, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		if ruleTargetsChannel(ruleMap, lowerTag, targetDomain) {
			changed = true
			continue
		}
		filtered = append(filtered, ruleMap)
	}
	if changed {
		if runtime.GOOS == "windows" {
			updated := applyWindowsDirectRules(filtered)
			if !reflect.DeepEqual(filtered, updated) {
				filtered = updated
			}
		}
		routing["rules"] = filtered
	}
	return changed
}

func reverseRuleMatches(rule map[string]any, channel serverReverseChannel, trimmedUser string) bool {
	if typ, _ := rule["type"].(string); !strings.EqualFold(strings.TrimSpace(typ), "field") {
		return false
	}
	if outbound, _ := rule["outboundTag"].(string); !strings.EqualFold(strings.TrimSpace(outbound), channel.Tag) {
		return false
	}
	if !stringSliceEqual(extractStringSlice(rule["domain"]), []string{"full:" + channel.Domain}) {
		return false
	}
	expectedUser := []string{}
	if trimmedUser != "" {
		expectedUser = []string{trimmedUser}
	}
	if !stringSliceEqual(extractStringSlice(rule["user"]), expectedUser) {
		return false
	}
	return true
}

func desiredReverseRule(channel serverReverseChannel, trimmedUser string) map[string]any {
	rule := map[string]any{
		"type":        "field",
		"domain":      []string{"full:" + channel.Domain},
		"outboundTag": channel.Tag,
	}
	if trimmedUser != "" {
		rule["user"] = []string{trimmedUser}
	}
	return rule
}

func ruleTargetsChannel(rule map[string]any, lowerTag string, lowerDomain string) bool {
	inbound := extractStringSlice(rule["inboundTag"])
	for _, tag := range inbound {
		if strings.ToLower(strings.TrimSpace(tag)) == lowerTag {
			return true
		}
	}
	outbound, _ := rule["outboundTag"].(string)
	if strings.ToLower(strings.TrimSpace(outbound)) == lowerTag {
		return true
	}
	for _, domain := range extractStringSlice(rule["domain"]) {
		if strings.ToLower(strings.TrimSpace(domain)) == lowerDomain {
			return true
		}
	}
	return false
}

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

func extractInterfaceSlice(raw any) []any {
	if arr, ok := raw.([]any); ok {
		return arr
	}
	return []any{}
}

func extractStringSlice(raw any) []string {
	switch values := raw.(type) {
	case []string:
		result := make([]string, len(values))
		for i, v := range values {
			result[i] = strings.TrimSpace(v)
		}
		return result
	case []any:
		result := make([]string, 0, len(values))
		for _, v := range values {
			if str, ok := v.(string); ok {
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

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !strings.EqualFold(strings.TrimSpace(a[i]), strings.TrimSpace(b[i])) {
			return false
		}
	}
	return true
}
