package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/naming"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

type clientInstallState struct {
	Endpoints []clientEndpointRecord          `json:"endpoints" toml:"endpoints"`
	Redirects []redirect.Rule                 `json:"redirects,omitempty" toml:"redirects"`
	Reverse   map[string]clientReverseChannel `json:"reverse,omitempty" toml:"reverse"`
	Forwards  []forward.Rule                  `json:"forwards,omitempty" toml:"forwards"`
}

type clientEndpointRecord struct {
	Hostname      string `json:"hostname" toml:"hostname"`
	Tag           string `json:"tag" toml:"tag"`
	Address       string `json:"address" toml:"address"`
	Port          int    `json:"port" toml:"port"`
	User          string `json:"user" toml:"user"`
	Password      string `json:"password" toml:"password"`
	ServerName    string `json:"server_name" toml:"server_name"`
	AllowInsecure bool   `json:"allow_insecure" toml:"allow_insecure"`
}

type clientReverseChannel struct {
	UserID      string `json:"user_id" toml:"user_id"`
	Host        string `json:"host" toml:"host"`
	Tag         string `json:"tag" toml:"tag"`
	Domain      string `json:"domain" toml:"domain"`
	EndpointTag string `json:"endpoint_tag" toml:"endpoint_tag"`
}

func loadClientInstallState(path string) (clientInstallState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return clientInstallState{}, nil
		}
		return clientInstallState{}, fmt.Errorf("xp2p: read client config %s: %w", path, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return clientInstallState{}, nil
	}

	tree, err := toml.LoadBytes(data)
	if err != nil {
		return clientInstallState{}, fmt.Errorf("xp2p: parse client config %s: %w", path, err)
	}
	raw := tree.GetPath([]string{"client"})
	if raw == nil {
		state := clientInstallState{}
		state.normalize()
		return state, nil
	}
	switch value := raw.(type) {
	case *toml.Tree:
		raw = value.ToMap()
	case map[string]any:
		raw = value
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return clientInstallState{}, fmt.Errorf("xp2p: encode client config %s: %w", path, err)
	}
	var state clientInstallState
	if err := json.Unmarshal(buf, &state); err != nil {
		return clientInstallState{}, fmt.Errorf("xp2p: decode client config %s: %w", path, err)
	}
	state.normalize()
	return state, nil
}

func (s *clientInstallState) normalize() {
	if s.Endpoints == nil {
		s.Endpoints = []clientEndpointRecord{}
	}
	if s.Redirects == nil {
		s.Redirects = []redirect.Rule{}
	}
	if s.Reverse == nil {
		s.Reverse = make(map[string]clientReverseChannel)
	}
	if s.Forwards == nil {
		s.Forwards = []forward.Rule{}
	}
}

func (s clientInstallState) save(path string) error {
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return err
	}
	s.normalize()

	if len(s.Endpoints) == 0 {
		tree.DeletePath([]string{"client", "endpoints"})
	} else {
		tree.SetPath([]string{"client", "endpoints"}, s.Endpoints)
	}
	if len(s.Redirects) == 0 {
		tree.DeletePath([]string{"client", "redirects"})
	} else {
		tree.SetPath([]string{"client", "redirects"}, s.Redirects)
	}
	if len(s.Forwards) == 0 {
		tree.DeletePath([]string{"client", "forwards"})
	} else {
		tree.SetPath([]string{"client", "forwards"}, s.Forwards)
	}
	if len(s.Reverse) == 0 {
		tree.DeletePath([]string{"client", "reverse"})
	} else {
		tree.SetPath([]string{"client", "reverse"}, s.Reverse)
	}
	return writeTomlTree(path, tree)
}

func (s *clientInstallState) addRedirect(rule redirect.Rule) error {
	updated, err := redirect.AddRule(s.Redirects, rule)
	if err != nil {
		return err
	}
	s.Redirects = updated
	return nil
}

func (s *clientInstallState) removeRedirect(target redirect.Target, tagFilter string) ([]redirect.Rule, bool) {
	updated, removed := redirect.RemoveRule(s.Redirects, target, tagFilter)
	if removed {
		s.Redirects = updated
	}
	return updated, removed
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

func (s *clientInstallState) ensureReverseChannel(userID, host, endpointTag string) (clientReverseChannel, error) {
	s.normalize()
	user := strings.TrimSpace(userID)
	trimmedHost := strings.TrimSpace(host)
	if user == "" || trimmedHost == "" {
		return clientReverseChannel{}, fmt.Errorf("xp2p: reverse channels require user and host")
	}
	tag, err := naming.ReverseTag(user, trimmedHost)
	if err != nil {
		return clientReverseChannel{}, err
	}
	channel := clientReverseChannel{
		UserID:      user,
		Host:        trimmedHost,
		Tag:         tag,
		Domain:      tag,
		EndpointTag: endpointTag,
	}
	if existing, ok := s.Reverse[tag]; ok {
		if !strings.EqualFold(existing.UserID, channel.UserID) || !strings.EqualFold(existing.Host, channel.Host) {
			return clientReverseChannel{}, fmt.Errorf("xp2p: reverse tag %s already assigned to %s@%s", tag, existing.UserID, existing.Host)
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
		samePort := existing.Port == record.Port
		if sameHost && samePort {
			if !force {
				return fmt.Errorf("xp2p: endpoint %s:%d already exists (use --force to update)", record.Hostname, record.Port)
			}
			s.Endpoints[idx] = record
			return nil
		}
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

func (s *clientInstallState) applyAllowInsecure(value bool) {
	for idx := range s.Endpoints {
		s.Endpoints[idx].AllowInsecure = value
	}
}

func (s *clientInstallState) addForward(rule forward.Rule) error {
	s.normalize()
	for _, existing := range s.Forwards {
		if existing.ListenPort == rule.ListenPort {
			return fmt.Errorf("xp2p: forward listener on port %d already exists", rule.ListenPort)
		}
		if strings.EqualFold(existing.Tag, rule.Tag) {
			return fmt.Errorf("xp2p: forward tag %s already exists", rule.Tag)
		}
		if strings.EqualFold(existing.Remark, rule.Remark) {
			return fmt.Errorf("xp2p: forward remark %s already exists", rule.Remark)
		}
	}
	s.Forwards = append(s.Forwards, rule)
	return nil
}

func (s *clientInstallState) removeForward(filter forward.Selector) (forward.Rule, int, bool) {
	if len(s.Forwards) == 0 {
		return forward.Rule{}, -1, false
	}
	idx := -1
	for i, rule := range s.Forwards {
		if filter.Matches(rule) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return forward.Rule{}, -1, false
	}
	removed := s.Forwards[idx]
	s.Forwards = append(s.Forwards[:idx], s.Forwards[idx+1:]...)
	return removed, idx, true
}

func (s *clientInstallState) insertForwardAt(rule forward.Rule, idx int) {
	if idx < 0 || idx > len(s.Forwards) {
		s.Forwards = append(s.Forwards, rule)
		return
	}
	s.Forwards = append(s.Forwards[:idx], append([]forward.Rule{rule}, s.Forwards[idx:]...)...)
}

func loadOrCreateToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tree, err := toml.TreeFromMap(map[string]any{})
			if err != nil {
				return nil, fmt.Errorf("xp2p: create empty client config tree: %w", err)
			}
			return tree, nil
		}
		return nil, fmt.Errorf("xp2p: read client config %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		tree, err := toml.TreeFromMap(map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("xp2p: create empty client config tree: %w", err)
		}
		return tree, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("xp2p: parse client config %s: %w", path, err)
	}
	return tree, nil
}

func writeTomlTree(path string, tree *toml.Tree) error {
	if tree == nil {
		return errors.New("xp2p: config tree is nil")
	}
	data, err := toml.Marshal(tree.ToMap())
	if err != nil {
		return fmt.Errorf("xp2p: encode client config %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("xp2p: ensure client config dir %s: %w", filepath.Dir(path), err)
	}
	if err := configio.WriteBytes(path, data, configio.WriteOptions{
		AuditPath: config.ConfigPath(layout.AuditLogFileName),
	}); err != nil {
		return err
	}
	return nil
}
