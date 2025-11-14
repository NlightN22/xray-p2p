package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type endpointConfig struct {
	Hostname      string
	Port          int
	User          string
	Password      string
	ServerName    string
	AllowInsecure bool
}

type clientInstallState struct {
	Endpoints []clientEndpointRecord `json:"endpoints"`
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

func applyClientEndpointConfig(configDir, stateFile string, endpoint endpointConfig, force bool) error {
	host := strings.TrimSpace(endpoint.Hostname)
	if host == "" {
		return errors.New("xp2p: endpoint hostname is required")
	}

	tag := buildProxyTag(host)
	state, err := loadClientInstallState(stateFile)
	if err != nil {
		return err
	}

	record := clientEndpointRecord{
		Hostname:      host,
		Tag:           tag,
		Address:       host,
		Port:          endpoint.Port,
		User:          endpoint.User,
		Password:      endpoint.Password,
		ServerName:    endpoint.ServerName,
		AllowInsecure: endpoint.AllowInsecure,
	}

	if err := state.upsert(record, force); err != nil {
		return err
	}
	if err := state.save(stateFile); err != nil {
		return err
	}
	if err := writeOutboundsConfig(filepath.Join(configDir, "outbounds.json"), state.Endpoints); err != nil {
		return err
	}
	if err := updateRoutingConfig(filepath.Join(configDir, "routing.json"), state.Endpoints); err != nil {
		return err
	}
	return nil
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

func buildProxyTag(host string) string {
	sanitized := sanitizeHost(host)
	if sanitized == "" {
		sanitized = "endpoint"
	}
	return "proxy-" + sanitized
}

func sanitizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	var b strings.Builder
	lastDash := false
	for _, r := range host {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '-' {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		switch r {
		case '.', ':':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

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

func updateRoutingConfig(path string, endpoints []clientEndpointRecord) error {
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
	filtered := filterManagedRules(existing, endpoints)
	for _, ep := range endpoints {
		filtered = append(filtered, map[string]any{
			"type":        "field",
			"ip":          []string{ep.Address},
			"outboundTag": ep.Tag,
		})
	}
	routing["rules"] = filtered

	reverse := ensureObject(document, "reverse")
	if _, ok := reverse["bridges"]; !ok {
		reverse["bridges"] = []any{}
	}

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

func filterManagedRules(rules []any, endpoints []clientEndpointRecord) []any {
	if len(rules) == 0 {
		return []any{}
	}
	known := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		known[ep.Tag] = struct{}{}
	}

	result := make([]any, 0, len(rules))
	for _, rule := range rules {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			result = append(result, rule)
			continue
		}
		outbound, _ := ruleMap["outboundTag"].(string)
		if _, managed := known[outbound]; managed {
			continue
		}
		result = append(result, ruleMap)
	}
	return result
}
