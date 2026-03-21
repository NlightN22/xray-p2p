package server

import (
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

func ensureServerMarkerRules(configDir string, state serverReverseState) error {
	path := filepath.Join(configDir, "routing.json")
	doc, err := loadServerRouting(path)
	if err != nil {
		return err
	}
	changed := updateServerMarkerRules(doc, state)
	if !changed {
		return nil
	}
	return writeServerRoutingDoc(path, doc)
}

func updateServerMarkerRules(doc map[string]any, state serverReverseState) bool {
	routing := ensureObject(doc, "routing")
	rules := extractInterfaceSlice(routing["rules"])
	tags := sortedReverseTags(state)
	managed := make(map[string]struct{}, len(tags))
	tagToChannel := make(map[string]serverReverseChannel, len(tags))
	for _, tag := range tags {
		managed[strings.ToLower(tag)] = struct{}{}
		tagToChannel[strings.ToLower(tag)] = state[tag]
	}

	filtered := make([]any, 0, len(rules))
	changed := false
	for _, raw := range rules {
		ruleMap, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		if isMarkerRule(ruleMap, managed) {
			changed = true
			continue
		}
		filtered = append(filtered, ruleMap)
	}

	for idx, tag := range tags {
		channel := tagToChannel[strings.ToLower(tag)]
		markerIP, err := markerIPForIndex(idx)
		if err != nil {
			continue
		}
		markerCIDR := markerIP + "/32"
		filtered = append(filtered, map[string]any{
			"type":        "field",
			"ip":          []string{markerCIDR},
			"inboundTag":  []string{"socks-in"},
			"port":        fmt.Sprintf("%d", DiagnosticsMarkerPort),
			"outboundTag": channel.Tag,
		})
		changed = true
	}

	if runtime.GOOS == "windows" {
		updated := applyWindowsDirectRules(filtered)
		if !reflect.DeepEqual(filtered, updated) {
			filtered = updated
			changed = true
		}
	}
	if changed {
		routing["rules"] = filtered
	}
	return changed
}

func isMarkerRule(rule map[string]any, managed map[string]struct{}) bool {
	outbound, _ := rule["outboundTag"].(string)
	if _, ok := managed[strings.ToLower(strings.TrimSpace(outbound))]; !ok {
		return false
	}
	portValue := fmt.Sprintf("%v", rule["port"])
	if portValue != fmt.Sprintf("%d", DiagnosticsMarkerPort) {
		return false
	}
	ips := extractStringSlice(rule["ip"])
	for _, ip := range ips {
		if strings.HasPrefix(strings.TrimSpace(ip), "127.255.") {
			return true
		}
	}
	return false
}
