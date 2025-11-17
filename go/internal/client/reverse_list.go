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
)

// ReverseListOptions controls client reverse enumeration.
type ReverseListOptions struct {
	InstallDir string
	ConfigDir  string
}

// ReverseRecord describes a client reverse tunnel.
type ReverseRecord struct {
	Tag         string
	Host        string
	User        string
	Domain      string
	EndpointTag string
	Bridge      bool
	DirectRule  bool
}

// ListReverse enumerates client reverse tunnels with routing artifacts.
func ListReverse(opts ReverseListOptions) ([]ReverseRecord, error) {
	paths, err := resolveClientPaths(opts.InstallDir, opts.ConfigDir)
	if err != nil {
		return nil, err
	}

	state, err := loadClientInstallState(paths.stateFile)
	if err != nil {
		return nil, err
	}

	routingPath := filepath.Join(paths.configDir, "routing.json")
	routingDoc, err := loadClientRoutingDoc(routingPath)
	if err != nil {
		return nil, err
	}

	bridges := clientBridgeIndex(routingDoc)
	directRules := clientDirectRuleSet(routingDoc)

	keys := make([]string, 0, len(state.Reverse))
	for tag := range state.Reverse {
		keys = append(keys, tag)
	}
	sort.Strings(keys)

	records := make([]ReverseRecord, 0, len(keys))
	for _, key := range keys {
		channel := state.Reverse[key]
		lowerTag := normalizeKey(channel.Tag)
		records = append(records, ReverseRecord{
			Tag:         channel.Tag,
			Host:        channel.Host,
			User:        channel.UserID,
			Domain:      channel.Domain,
			EndpointTag: channel.EndpointTag,
			Bridge:      bridges[lowerTag] == normalizeKey(channel.Domain),
			DirectRule:  hasClientDirectRule(lowerTag, directRules),
		})
	}
	return records, nil
}

func loadClientRoutingDoc(path string) (map[string]any, error) {
	doc := make(map[string]any)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil
		}
		return nil, fmt.Errorf("xp2p: read client routing %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("xp2p: parse client routing %s: %w", path, err)
	}
	return doc, nil
}

func clientBridgeIndex(doc map[string]any) map[string]string {
	reverse, _ := doc["reverse"].(map[string]any)
	if reverse == nil {
		return map[string]string{}
	}
	rawEntries, _ := reverse["bridges"].([]any)
	index := make(map[string]string, len(rawEntries))
	for _, raw := range rawEntries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := entry["tag"].(string)
		domain, _ := entry["domain"].(string)
		lowerTag := normalizeKey(tag)
		lowerDomain := normalizeKey(domain)
		if lowerTag == "" || lowerDomain == "" {
			continue
		}
		index[lowerTag] = lowerDomain
	}
	return index
}

func clientDirectRuleSet(doc map[string]any) map[string]struct{} {
	routing, _ := doc["routing"].(map[string]any)
	if routing == nil {
		return map[string]struct{}{}
	}
	rules := extractRuleSlice(routing["rules"])
	result := make(map[string]struct{})
	for _, raw := range rules {
		ruleMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		outbound, _ := ruleMap["outboundTag"].(string)
		if !strings.EqualFold(strings.TrimSpace(outbound), "direct") {
			continue
		}
		for _, tag := range extractStringSlice(ruleMap["inboundTag"]) {
			if normalized := normalizeKey(tag); normalized != "" {
				result[normalized] = struct{}{}
			}
		}
	}
	return result
}

func hasClientDirectRule(tag string, rules map[string]struct{}) bool {
	if tag == "" {
		return false
	}
	_, ok := rules[tag]
	return ok
}

func normalizeKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
