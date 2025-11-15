package client

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

type clientInstallState struct {
	Endpoints []clientEndpointRecord          `json:"endpoints"`
	Redirects []clientRedirectRule            `json:"redirects,omitempty"`
	Reverse   map[string]clientReverseChannel `json:"reverse,omitempty"`
}

type clientEndpointRecord struct {
	Hostname      string `json:"hostname"`
	Tag           string `json:"tag"`
	Address       string `json:"address"`
	Port          int    `json:"port"`
	User          string `json:"user"`
	Password      string `json:"password"`
	ServerName    string `json:"server_name"`
	AllowInsecure bool   `json:"allow_insecure"`
}

type clientRedirectRule struct {
	CIDR        string `json:"cidr,omitempty"`
	Domain      string `json:"domain,omitempty"`
	OutboundTag string `json:"outbound_tag"`
}

type clientReverseChannel struct {
	UserID      string `json:"user_id"`
	Tag         string `json:"tag"`
	Domain      string `json:"domain"`
	EndpointTag string `json:"endpoint_tag"`
}

type redirectRuleType int

const (
	redirectRuleTypeCIDR redirectRuleType = iota
	redirectRuleTypeDomain
)

type redirectTarget struct {
	kind  redirectRuleType
	value string
}

func (t redirectRuleType) label() string {
	switch t {
	case redirectRuleTypeDomain:
		return "domain"
	default:
		return "CIDR"
	}
}

func describeRedirect(kind redirectRuleType, value string) string {
	return fmt.Sprintf("%s %s", kind.label(), value)
}

func (t redirectTarget) describe() string {
	return describeRedirect(t.kind, t.value)
}

func (t redirectTarget) matches(rule clientRedirectRule) bool {
	switch t.kind {
	case redirectRuleTypeDomain:
		if strings.TrimSpace(rule.Domain) == "" {
			return false
		}
		return strings.EqualFold(rule.value(), t.value)
	default:
		if strings.TrimSpace(rule.CIDR) == "" {
			return false
		}
		return strings.EqualFold(rule.value(), t.value)
	}
}

func (r clientRedirectRule) kind() redirectRuleType {
	if strings.TrimSpace(r.Domain) != "" {
		return redirectRuleTypeDomain
	}
	return redirectRuleTypeCIDR
}

func (r clientRedirectRule) value() string {
	switch r.kind() {
	case redirectRuleTypeDomain:
		return strings.ToLower(strings.TrimSpace(r.Domain))
	default:
		return strings.TrimSpace(r.CIDR)
	}
}

func loadClientInstallState(path string) (clientInstallState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return clientInstallState{}, nil
		}
		return clientInstallState{}, fmt.Errorf("xp2p: read client state %s: %w", path, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return clientInstallState{}, nil
	}

	var state clientInstallState
	if err := json.Unmarshal(data, &state); err != nil {
		return clientInstallState{}, fmt.Errorf("xp2p: parse client state %s: %w", path, err)
	}
	state.normalize()
	return state, nil
}

func (s *clientInstallState) normalize() {
	if s.Endpoints == nil {
		s.Endpoints = []clientEndpointRecord{}
	}
	if s.Redirects == nil {
		s.Redirects = []clientRedirectRule{}
	}
	if s.Reverse == nil {
		s.Reverse = make(map[string]clientReverseChannel)
	}
}

func (s clientInstallState) save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("xp2p: encode client state %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure client state dir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("xp2p: write client state %s: %w", path, err)
	}
	return nil
}

func (s *clientInstallState) addRedirect(rule clientRedirectRule) error {
	tag := strings.TrimSpace(rule.OutboundTag)
	value := rule.value()
	if tag == "" || value == "" {
		return errors.New("xp2p: redirect CIDR or domain and outbound tag are required")
	}

	kind := rule.kind()
	switch kind {
	case redirectRuleTypeDomain:
		rule.Domain = value
		rule.CIDR = ""
	default:
		rule.CIDR = value
		rule.Domain = ""
	}
	rule.OutboundTag = tag

	for _, existing := range s.Redirects {
		if existing.kind() != kind {
			continue
		}
		if !strings.EqualFold(existing.value(), value) {
			continue
		}
		if strings.EqualFold(existing.OutboundTag, tag) {
			return fmt.Errorf("xp2p: redirect %s via %s already exists", describeRedirect(kind, value), tag)
		}
	}
	s.Redirects = append(s.Redirects, rule)
	return nil
}

func (s *clientInstallState) removeRedirect(target redirectTarget, tagFilter string) ([]clientRedirectRule, bool) {
	if len(s.Redirects) == 0 {
		return s.Redirects, false
	}
	result := make([]clientRedirectRule, 0, len(s.Redirects))
	trimmedTag := strings.TrimSpace(tagFilter)
	removed := false
	for _, rule := range s.Redirects {
		matchValue := target.matches(rule)
		matchTag := trimmedTag == "" || strings.EqualFold(rule.OutboundTag, trimmedTag)
		if matchValue && matchTag {
			removed = true
			continue
		}
		result = append(result, rule)
	}
	return result, removed
}

func (s *clientInstallState) removeEndpoint(target string) (clientEndpointRecord, bool) {
	if len(s.Endpoints) == 0 {
		return clientEndpointRecord{}, false
	}
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return clientEndpointRecord{}, false
	}
	lower := strings.ToLower(trimmed)
	for idx, ep := range s.Endpoints {
		if strings.EqualFold(ep.Hostname, trimmed) || strings.ToLower(ep.Tag) == lower {
			removed := ep
			s.Endpoints = append(s.Endpoints[:idx], s.Endpoints[idx+1:]...)
			return removed, true
		}
	}
	return clientEndpointRecord{}, false
}

func (s *clientInstallState) removeRedirectsByTag(tag string) {
	if len(s.Redirects) == 0 {
		return
	}
	lower := strings.ToLower(strings.TrimSpace(tag))
	if lower == "" {
		return
	}
	filtered := s.Redirects[:0]
	for _, rule := range s.Redirects {
		if strings.ToLower(strings.TrimSpace(rule.OutboundTag)) == lower {
			continue
		}
		filtered = append(filtered, rule)
	}
	s.Redirects = filtered
}

func (s *clientInstallState) ensureReverseChannel(userID, endpointTag string) (clientReverseChannel, error) {
	s.normalize()
	tag, err := naming.ReverseTag(userID)
	if err != nil {
		return clientReverseChannel{}, err
	}
	channel := clientReverseChannel{
		UserID:      strings.TrimSpace(userID),
		Tag:         tag,
		Domain:      tag,
		EndpointTag: endpointTag,
	}
	if existing, ok := s.Reverse[tag]; ok {
		if !strings.EqualFold(existing.UserID, channel.UserID) {
			return clientReverseChannel{}, fmt.Errorf("xp2p: reverse tag %s already assigned to %s", tag, existing.UserID)
		}
		if !strings.EqualFold(existing.EndpointTag, endpointTag) {
			return clientReverseChannel{}, fmt.Errorf("xp2p: reverse tag %s already routed via %s", tag, existing.EndpointTag)
		}
		return existing, nil
	}
	s.Reverse[tag] = channel
	return channel, nil
}

func (s *clientInstallState) removeReverseChannelsByTag(tag string) {
	if len(s.Reverse) == 0 {
		return
	}
	lower := strings.ToLower(strings.TrimSpace(tag))
	if lower == "" {
		return
	}
	for key, channel := range s.Reverse {
		if strings.ToLower(strings.TrimSpace(channel.EndpointTag)) == lower {
			delete(s.Reverse, key)
		}
	}
}

func (s *clientInstallState) upsert(record clientEndpointRecord, force bool) error {
	for idx, existing := range s.Endpoints {
		sameHost := strings.EqualFold(existing.Hostname, record.Hostname)
		if sameHost {
			if !force {
				return fmt.Errorf("xp2p: endpoint %s already exists (use --force to update)", record.Hostname)
			}
			s.Endpoints[idx] = record
			return nil
		}
		if strings.EqualFold(existing.Tag, record.Tag) {
			return fmt.Errorf("xp2p: outbound tag %s is already assigned to %s", record.Tag, existing.Hostname)
		}
	}
	s.Endpoints = append(s.Endpoints, record)
	return nil
}
