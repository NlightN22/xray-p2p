package client

import (
	"sort"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/xrayrule"
)

func buildClientReverseRules(reverse map[string]clientReverseChannel) []any {
	channels := sortedReverseChannels(reverse)
	if len(channels) == 0 {
		return nil
	}
	result := make([]any, 0, len(channels)*2)
	for _, channel := range channels {
		inbound := []string{channel.Tag}
		result = append(result, map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.ReverseDomain("client", channel.Tag, channel.EndpointTag, channel.Domain),
			"domain":      []string{"full:" + channel.Domain},
			"inboundTag":  inbound,
			"outboundTag": channel.EndpointTag,
		})
		result = append(result, map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.ReverseDirect("client", channel.Tag),
			"inboundTag":  inbound,
			"outboundTag": directRandomTag(),
		})
	}
	return result
}

func sortedReverseChannels(reverse map[string]clientReverseChannel) []clientReverseChannel {
	if len(reverse) == 0 {
		return []clientReverseChannel{}
	}
	keys := make([]string, 0, len(reverse))
	for key := range reverse {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]clientReverseChannel, 0, len(keys))
	for _, key := range keys {
		result = append(result, reverse[key])
	}
	return result
}

func updateReverseBridges(reverseObj map[string]any, channels []clientReverseChannel) {
	existing, _ := reverseObj["bridges"].([]any)
	managed := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		managed[strings.ToLower(strings.TrimSpace(channel.Tag))] = struct{}{}
	}
	filtered := make([]any, 0, len(existing))
	for _, raw := range existing {
		entry, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		tag, _ := entry["tag"].(string)
		if _, ok := managed[strings.ToLower(strings.TrimSpace(tag))]; ok {
			continue
		}
		filtered = append(filtered, entry)
	}
	for _, channel := range channels {
		filtered = append(filtered, map[string]any{
			"domain": channel.Domain,
			"tag":    channel.Tag,
		})
	}
	if filtered == nil {
		filtered = []any{}
	}
	reverseObj["bridges"] = filtered
}
