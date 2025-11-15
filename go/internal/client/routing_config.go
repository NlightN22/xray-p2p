package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func writeOutboundsConfig(path string, endpoints []clientEndpointRecord) error {
	out := struct {
		Outbounds []any `json:"outbounds"`
	}{
		Outbounds: make([]any, 0, len(endpoints)+1),
	}

	for _, ep := range endpoints {
		out.Outbounds = append(out.Outbounds, trojanOutbound(ep))
	}

	out.Outbounds = append(out.Outbounds, freedomOutbound())

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("xp2p: encode outbounds %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure outbounds dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write outbounds %s: %w", path, err)
	}
	return nil
}

func trojanOutbound(ep clientEndpointRecord) any {
	return struct {
		Protocol       string         `json:"protocol"`
		Settings       trojanSettings `json:"settings"`
		StreamSettings streamSettings `json:"streamSettings"`
		Tag            string         `json:"tag"`
	}{
		Protocol: "trojan",
		Settings: trojanSettings{
			Servers: []trojanServer{
				{
					Address:  ep.Address,
					Port:     ep.Port,
					Password: ep.Password,
					Email:    ep.User,
				},
			},
		},
		StreamSettings: streamSettings{
			Network:  "tcp",
			Security: "tls",
			TLSSettings: tlsSettings{
				AllowInsecure: ep.AllowInsecure,
				ServerName:    ep.ServerName,
			},
			TCPSettings: tcpSettings{
				Header: tcpHeader{
					Type: "http",
					Request: tcpRequest{
						Version: "1.1",
						Method:  "GET",
						Path:    []string{"/"},
						Headers: map[string][]string{
							"Host": {
								"www.bing.com",
								"www.apple.com",
							},
							"User-Agent": {
								"Mozilla/5.0",
							},
							"Accept-Encoding": {
								"gzip, deflate",
							},
							"Connection": {
								"keep-alive",
							},
						},
					},
				},
			},
		},
		Tag: ep.Tag,
	}
}

type trojanSettings struct {
	Servers []trojanServer `json:"servers"`
}

type trojanServer struct {
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

type streamSettings struct {
	Network     string      `json:"network"`
	Security    string      `json:"security"`
	TLSSettings tlsSettings `json:"tlsSettings"`
	TCPSettings tcpSettings `json:"tcpSettings"`
}

type tlsSettings struct {
	AllowInsecure bool   `json:"allowInsecure"`
	ServerName    string `json:"serverName"`
}

type tcpSettings struct {
	Header tcpHeader `json:"header"`
}

type tcpHeader struct {
	Type    string     `json:"type"`
	Request tcpRequest `json:"request"`
}

type tcpRequest struct {
	Version string              `json:"version"`
	Method  string              `json:"method"`
	Path    []string            `json:"path"`
	Headers map[string][]string `json:"headers"`
}

func freedomOutbound() any {
	return struct {
		Protocol string          `json:"protocol"`
		Settings freedomSettings `json:"settings"`
		Tag      string          `json:"tag"`
	}{
		Protocol: "freedom",
		Settings: freedomSettings{
			DomainStrategy: "UseIP",
		},
		Tag: "direct",
	}
}

type freedomSettings struct {
	DomainStrategy string `json:"domainStrategy"`
}

func updateRoutingConfig(path string, endpoints []clientEndpointRecord, redirects []redirect.Rule, reverse map[string]clientReverseChannel) error {
	var document map[string]any

	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("xp2p: read routing config %s: %w", path, err)
		}
		document = make(map[string]any)
	} else if len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &document); err != nil {
			return fmt.Errorf("xp2p: parse routing config %s: %w", path, err)
		}
	} else {
		document = make(map[string]any)
	}

	if document == nil {
		document = make(map[string]any)
	}

	routing := ensureObject(document, "routing")
	if strategy, ok := routing["domainStrategy"].(string); !ok || strings.TrimSpace(strategy) == "" {
		routing["domainStrategy"] = "IPOnDemand"
	}

	existing := extractRuleSlice(routing["rules"])
	managed := managedOutboundTags(endpoints, redirects)
	for _, rule := range existing {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		outbound, _ := ruleMap["outboundTag"].(string)
		trimmedOutbound := strings.ToLower(strings.TrimSpace(outbound))
		if strings.HasPrefix(trimmedOutbound, "proxy-") {
			managed[trimmedOutbound] = struct{}{}
		}
	}

	filtered := filterManagedRules(existing, managed)
	filtered = filterReverseRules(filtered, reverse)
	for _, rule := range redirects {
		entry := map[string]any{
			"type":        "field",
			"outboundTag": rule.OutboundTag,
		}
		switch rule.Kind() {
		case redirect.KindDomain:
			entry["domains"] = []string{rule.Value()}
		default:
			entry["ip"] = []string{rule.Value()}
		}
		filtered = append(filtered, entry)
	}
	for _, ep := range endpoints {
		filtered = append(filtered, map[string]any{
			"type":        "field",
			"ip":          []string{ep.Address},
			"outboundTag": ep.Tag,
		})
	}
	reverseRules := buildClientReverseRules(reverse)
	if len(reverseRules) > 0 {
		filtered = append(reverseRules, filtered...)
	}
	routing["rules"] = filtered

	reverseObj := ensureObject(document, "reverse")
	updateReverseBridges(reverseObj, sortedReverseChannels(reverse))

	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("xp2p: encode routing config %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure routing dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return fmt.Errorf("xp2p: write routing config %s: %w", path, err)
	}
	return nil
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

func extractRuleSlice(raw any) []any {
	if rules, ok := raw.([]any); ok {
		return rules
	}
	return []any{}
}

func managedOutboundTags(endpoints []clientEndpointRecord, redirects []redirect.Rule) map[string]struct{} {
	total := len(endpoints) + len(redirects)
	if total == 0 {
		return map[string]struct{}{}
	}
	known := make(map[string]struct{}, total)
	for _, ep := range endpoints {
		if tag := strings.TrimSpace(ep.Tag); tag != "" {
			known[strings.ToLower(tag)] = struct{}{}
		}
	}
	for _, rule := range redirects {
		if tag := strings.TrimSpace(rule.OutboundTag); tag != "" {
			known[strings.ToLower(tag)] = struct{}{}
		}
	}
	return known
}

func filterManagedRules(rules []any, managed map[string]struct{}) []any {
	if len(rules) == 0 {
		return []any{}
	}
	if len(managed) == 0 {
		return rules
	}

	result := make([]any, 0, len(rules))
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			result = append(result, rule)
			continue
		}
		outbound, _ := ruleMap["outboundTag"].(string)
		if _, managed := managed[strings.ToLower(outbound)]; managed {
			continue
		}
		result = append(result, ruleMap)
	}
	return result
}

func filterReverseRules(rules []any, reverse map[string]clientReverseChannel) []any {
	if len(rules) == 0 || len(reverse) == 0 {
		return rules
	}
	known := make(map[string]struct{}, len(reverse))
	for _, channel := range reverse {
		known[strings.ToLower(strings.TrimSpace(channel.Tag))] = struct{}{}
	}
	filtered := make([]any, 0, len(rules))
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			filtered = append(filtered, rule)
			continue
		}
		inbound := extractStringSlice(ruleMap["inboundTag"])
		remove := false
		for _, tag := range inbound {
			if _, ok := known[strings.ToLower(strings.TrimSpace(tag))]; ok {
				remove = true
				break
			}
		}
		if remove {
			continue
		}
		filtered = append(filtered, ruleMap)
	}
	return filtered
}

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
			"domain":      []string{"full:" + channel.Domain},
			"inboundTag":  inbound,
			"outboundTag": channel.EndpointTag,
		})
		result = append(result, map[string]any{
			"type":        "field",
			"inboundTag":  inbound,
			"outboundTag": "direct",
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
