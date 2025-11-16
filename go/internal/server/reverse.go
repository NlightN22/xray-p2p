//go:build windows || linux

package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/naming"
)

const serverReverseStateKey = "reverse_channels"

type serverReverseChannel struct {
	UserID string `json:"user_id"`
	Host   string `json:"host"`
	Tag    string `json:"tag"`
	Domain string `json:"domain"`
}

type serverReverseState map[string]serverReverseChannel

func (s *serverReverseState) ensure() {
	if *s == nil {
		*s = make(serverReverseState)
	}
}

type reverseStore struct {
	path  string
	doc   map[string]any
	state serverReverseState
}

func openReverseStore(installDir string) (reverseStore, error) {
	path := serverStatePath(installDir)
	doc, err := loadServerStateDoc(path)
	if err != nil {
		return reverseStore{}, err
	}
	state, err := decodeServerReverseState(doc)
	if err != nil {
		return reverseStore{}, err
	}
	return reverseStore{
		path:  path,
		doc:   doc,
		state: state,
	}, nil
}

func (s *reverseStore) ensureAvailable(channel serverReverseChannel) error {
	existing, ok := s.state[channel.Tag]
	if !ok {
		return nil
	}
	if strings.EqualFold(existing.UserID, channel.UserID) {
		return nil
	}
	return fmt.Errorf("xp2p: reverse tag %s already assigned to %s", channel.Tag, existing.UserID)
}

func (s *reverseStore) put(channel serverReverseChannel) {
	s.state.ensure()
	s.state[channel.Tag] = channel
}

func (s *reverseStore) delete(tag string) {
	if s.state == nil {
		return
	}
	delete(s.state, tag)
}

func (s *reverseStore) save() error {
	if len(s.state) == 0 {
		delete(s.doc, serverReverseStateKey)
	} else {
		s.doc[serverReverseStateKey] = s.state
	}
	return writeServerStateDoc(s.path, s.doc)
}

func buildServerReverseChannel(userID, host string) (serverReverseChannel, error) {
	trimmed := strings.TrimSpace(userID)
	if trimmed == "" {
		return serverReverseChannel{}, errUserIDRequired
	}
	hostValue := strings.TrimSpace(host)
	tag, err := naming.ReverseTag(trimmed, hostValue)
	if err != nil {
		return serverReverseChannel{}, err
	}
	return serverReverseChannel{
		UserID: trimmed,
		Host:   hostValue,
		Tag:    tag,
		Domain: tag,
	}, nil
}

func applyServerReverseChannel(store *reverseStore, configDir string, channel serverReverseChannel) error {
	if err := ensureServerRoutingConfig(configDir, channel); err != nil {
		return err
	}
	store.put(channel)
	return store.save()
}

func purgeServerReverseChannel(store *reverseStore, configDir string, channel serverReverseChannel) error {
	if err := removeServerRoutingConfig(configDir, channel); err != nil {
		return err
	}
	store.delete(channel.Tag)
	return store.save()
}

func ensureServerRoutingConfig(configDir string, channel serverReverseChannel) error {
	path := filepath.Join(configDir, "routing.json")
	doc, err := loadServerRouting(path)
	if err != nil {
		return err
	}
	changed := ensureReversePortal(doc, channel)
	if ensureReverseRule(doc, channel) {
		changed = true
	}
	if !changed {
		return nil
	}
	return writeServerRouting(path, doc)
}

func removeServerRoutingConfig(configDir string, channel serverReverseChannel) error {
	path := filepath.Join(configDir, "routing.json")
	doc, err := loadServerRouting(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	changed := removeReversePortal(doc, channel)
	if removeReverseRules(doc, channel) {
		changed = true
	}
	if !changed {
		return nil
	}
	return writeServerRouting(path, doc)
}

func loadServerRouting(path string) (map[string]any, error) {
	doc := make(map[string]any)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return doc, nil
		}
		return nil, fmt.Errorf("xp2p: read routing %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("xp2p: parse routing %s: %w", path, err)
	}
	return doc, nil
}

func writeServerRouting(path string, doc map[string]any) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("xp2p: encode routing %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure routing dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write routing %s: %w", path, err)
	}
	return nil
}

func ensureReversePortal(doc map[string]any, channel serverReverseChannel) bool {
	reverse := ensureObject(doc, "reverse")
	portals := extractInterfaceSlice(reverse["portals"])
	lowerTag := strings.ToLower(channel.Tag)
	lowerDomain := strings.ToLower(channel.Domain)
	filtered := make([]any, 0, len(portals))
	replaced := false
	changed := false
	for _, raw := range portals {
		entry, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		tag, _ := entry["tag"].(string)
		domain, _ := entry["domain"].(string)
		if strings.ToLower(strings.TrimSpace(tag)) == lowerTag || strings.ToLower(strings.TrimSpace(domain)) == lowerDomain {
			if replaced {
				changed = true
				continue
			}
			if !strings.EqualFold(strings.TrimSpace(tag), channel.Tag) || !strings.EqualFold(strings.TrimSpace(domain), channel.Domain) {
				changed = true
			}
			filtered = append(filtered, map[string]any{
				"domain": channel.Domain,
				"tag":    channel.Tag,
			})
			replaced = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !replaced {
		filtered = append(filtered, map[string]any{
			"domain": channel.Domain,
			"tag":    channel.Tag,
		})
		changed = true
	}
	reverse["portals"] = filtered
	return changed
}

func removeReversePortal(doc map[string]any, channel serverReverseChannel) bool {
	reverse := ensureObject(doc, "reverse")
	portals := extractInterfaceSlice(reverse["portals"])
	lowerTag := strings.ToLower(channel.Tag)
	lowerDomain := strings.ToLower(channel.Domain)
	filtered := make([]any, 0, len(portals))
	changed := false
	for _, raw := range portals {
		entry, ok := raw.(map[string]any)
		if !ok {
			filtered = append(filtered, raw)
			continue
		}
		tag, _ := entry["tag"].(string)
		domain, _ := entry["domain"].(string)
		if strings.ToLower(strings.TrimSpace(tag)) == lowerTag || strings.ToLower(strings.TrimSpace(domain)) == lowerDomain {
			changed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if changed {
		reverse["portals"] = filtered
	}
	return changed
}

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
	if !stringSliceEqual(extractStringSlice(rule["inboundTag"]), []string{channel.Tag}) {
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
		"inboundTag":  []string{channel.Tag},
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
