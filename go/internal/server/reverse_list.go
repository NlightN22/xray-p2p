package server

import (
	"path/filepath"
	"sort"
	"strings"
)

// ReverseListOptions controls server reverse enumeration.
type ReverseListOptions struct {
	InstallDir string
	ConfigDir  string
}

// ReverseRecord describes a server reverse tunnel.
type ReverseRecord struct {
	Domain      string
	Host        string
	User        string
	Tag         string
	Portal      bool
	RoutingRule bool
}

// ListReverse enumerates server reverse tunnels and their routing artifacts.
func ListReverse(opts ReverseListOptions) ([]ReverseRecord, error) {
	installDir, err := resolveInstallDir(opts.InstallDir)
	if err != nil {
		return nil, err
	}
	configDir, err := resolveConfigDir(installDir, opts.ConfigDir)
	if err != nil {
		return nil, err
	}

	stateDoc, err := loadServerStateDoc(serverStatePath(installDir))
	if err != nil {
		return nil, err
	}
	reverseState, err := decodeServerReverseState(stateDoc)
	if err != nil {
		return nil, err
	}

	routingPath := filepath.Join(configDir, "routing.json")
	routingDoc, err := loadServerRouting(routingPath)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(reverseState))
	for tag := range reverseState {
		keys = append(keys, tag)
	}
	sort.Strings(keys)

	records := make([]ReverseRecord, 0, len(keys))
	for _, tag := range keys {
		channel := reverseState[tag]
		records = append(records, ReverseRecord{
			Domain:      channel.Domain,
			Host:        channel.Host,
			User:        channel.UserID,
			Tag:         channel.Tag,
			Portal:      hasServerReversePortal(routingDoc, channel),
			RoutingRule: hasServerReverseRule(routingDoc, channel),
		})
	}
	return records, nil
}

func hasServerReversePortal(doc map[string]any, channel serverReverseChannel) bool {
	reverse, _ := doc["reverse"].(map[string]any)
	if reverse == nil {
		return false
	}
	rawEntries, _ := reverse["portals"].([]any)
	if len(rawEntries) == 0 {
		return false
	}
	lowerTag := lowerTrim(channel.Tag)
	lowerDomain := lowerTrim(channel.Domain)
	for _, raw := range rawEntries {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := entry["tag"].(string)
		domain, _ := entry["domain"].(string)
		if lowerTrim(tag) == lowerTag && lowerTrim(domain) == lowerDomain {
			return true
		}
	}
	return false
}

func hasServerReverseRule(doc map[string]any, channel serverReverseChannel) bool {
	routing, _ := doc["routing"].(map[string]any)
	if routing == nil {
		return false
	}
	rules := extractInterfaceSlice(routing["rules"])
	if len(rules) == 0 {
		return false
	}
	trimmedUser := strings.TrimSpace(channel.UserID)
	for _, raw := range rules {
		ruleMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if reverseRuleMatches(ruleMap, channel, trimmedUser) {
			return true
		}
	}
	return false
}

func lowerTrim(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
