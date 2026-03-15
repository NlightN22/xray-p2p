package xrayconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/pelletier/go-toml"

	"github.com/NlightN22/xray-p2p/go/internal/configio"
)

var ErrConfigParse = errors.New("xrayconfig: parse error")
var ErrConfigMissing = errors.New("xrayconfig: config file not found")
var ErrConfigEmpty = errors.New("xrayconfig: config file is empty")

type ClientXrayConfig struct {
	Inbounds       ClientInboundsConfig `json:"inbounds" toml:"inbounds"`
	Logs           LogsConfig           `json:"logs" toml:"logs"`
	Routing        RoutingConfig        `json:"routing" toml:"routing"`
	DirectOutbound DirectOutboundConfig `json:"direct_outbound" toml:"direct_outbound"`
}

type ServerXrayConfig struct {
	Inbounds       ServerInboundsConfig `json:"inbounds" toml:"inbounds"`
	Logs           LogsConfig           `json:"logs" toml:"logs"`
	Routing        RoutingConfig        `json:"routing" toml:"routing"`
	DirectOutbound DirectOutboundConfig `json:"direct_outbound" toml:"direct_outbound"`
}

type ClientInboundsConfig struct {
	Socks    SocksInboundConfig    `json:"socks" toml:"socks"`
	Dokodemo DokodemoInboundConfig `json:"dokodemo" toml:"dokodemo"`
	Tun      TunInboundConfig      `json:"tun" toml:"tun"`
}

type ServerInboundsConfig struct {
	Socks    SocksInboundConfig    `json:"socks" toml:"socks"`
	Dokodemo DokodemoInboundConfig `json:"dokodemo" toml:"dokodemo"`
	Tun      TunInboundConfig      `json:"tun" toml:"tun"`
	Trojan   TrojanInboundConfig   `json:"trojan" toml:"trojan"`
}

type SocksInboundConfig struct {
	Tag      string `json:"tag" toml:"tag"`
	Protocol string `json:"protocol" toml:"protocol"`
	Listen   string `json:"listen" toml:"listen"`
	Port     int    `json:"port" toml:"port"`
	UDP      *bool  `json:"udp" toml:"udp"`
}

type DokodemoInboundConfig struct {
	Tag            string `json:"tag" toml:"tag"`
	Remark         string `json:"remark" toml:"remark"`
	Protocol       string `json:"protocol" toml:"protocol"`
	Listen         string `json:"listen" toml:"listen"`
	Port           int    `json:"port" toml:"port"`
	Network        string `json:"network" toml:"network"`
	FollowRedirect *bool  `json:"follow_redirect" toml:"follow_redirect"`
}

type TunInboundConfig struct {
	Tag      string `json:"tag" toml:"tag"`
	Protocol string `json:"protocol" toml:"protocol"`
	Port     int    `json:"port" toml:"port"`
}

type TrojanInboundConfig struct {
	Listen        string    `json:"listen" toml:"listen"`
	Protocol      string    `json:"protocol" toml:"protocol"`
	Network       string    `json:"network" toml:"network"`
	Security      string    `json:"security" toml:"security"`
	AllowInsecure bool      `json:"allow_insecure" toml:"allow_insecure"`
	Header        TCPHeader `json:"header" toml:"header"`
}

type TCPHeader struct {
	Type    string     `json:"type" toml:"type"`
	Request TCPRequest `json:"request" toml:"request"`
}

type TCPRequest struct {
	Version string              `json:"version" toml:"version"`
	Method  string              `json:"method" toml:"method"`
	Path    []string            `json:"path" toml:"path"`
	Headers map[string][]string `json:"headers" toml:"headers"`
}

type LogsConfig struct {
	Level        string       `json:"level" toml:"level"`
	Access       string       `json:"access" toml:"access"`
	API          APIConfig    `json:"api" toml:"api"`
	StatsEnabled *bool        `json:"stats_enabled,omitempty" toml:"stats_enabled"`
	Policy       PolicyConfig `json:"policy" toml:"policy"`
}

type APIConfig struct {
	Tag      string   `json:"tag" toml:"tag"`
	Listen   string   `json:"listen" toml:"listen"`
	Services []string `json:"services" toml:"services"`
}

type PolicyConfig struct {
	Levels map[string]PolicyLevel `json:"levels" toml:"levels"`
	System PolicySystem           `json:"system" toml:"system"`
}

type PolicyLevel struct {
	StatsUserDownlink *bool `json:"stats_user_downlink" toml:"stats_user_downlink"`
	StatsUserUplink   *bool `json:"stats_user_uplink" toml:"stats_user_uplink"`
	StatsUserOnline   *bool `json:"stats_user_online" toml:"stats_user_online"`
}

type PolicySystem struct {
	StatsInboundDownlink  *bool `json:"stats_inbound_downlink" toml:"stats_inbound_downlink"`
	StatsInboundUplink    *bool `json:"stats_inbound_uplink" toml:"stats_inbound_uplink"`
	StatsOutboundDownlink *bool `json:"stats_outbound_downlink" toml:"stats_outbound_downlink"`
	StatsOutboundUplink   *bool `json:"stats_outbound_uplink" toml:"stats_outbound_uplink"`
}

type RoutingConfig struct {
	DomainStrategy string           `json:"domain_strategy" toml:"domain_strategy"`
	Rules          []map[string]any `json:"rules" toml:"rules"`
}

type DirectOutboundConfig struct {
	Tag            string `json:"tag" toml:"tag"`
	Protocol       string `json:"protocol" toml:"protocol"`
	DomainStrategy string `json:"domain_strategy" toml:"domain_strategy"`
	SendThrough    string `json:"send_through,omitempty" toml:"send_through"`
}

func DefaultClientConfig() ClientXrayConfig {
	udp := true
	followRedirect := true
	return ClientXrayConfig{
		Inbounds: ClientInboundsConfig{
			Socks: SocksInboundConfig{
				Tag:      "socks-in",
				Protocol: "socks",
				Listen:   "127.0.0.1",
				Port:     51180,
				UDP:      &udp,
			},
			Dokodemo: DokodemoInboundConfig{
				Tag:            "in-48054",
				Remark:         "local-input",
				Protocol:       "dokodemo-door",
				Listen:         "0.0.0.0",
				Port:           48054,
				Network:        "tcp",
				FollowRedirect: &followRedirect,
			},
			Tun: TunInboundConfig{
				Tag:      "tun-in",
				Protocol: "tun",
				Port:     0,
			},
		},
		Logs: defaultLogsConfig("127.0.0.1:52180"),
		Routing: RoutingConfig{
			DomainStrategy: "IPOnDemand",
			Rules:          []map[string]any{},
		},
		DirectOutbound: DirectOutboundConfig{
			Tag:            "direct",
			Protocol:       "freedom",
			DomainStrategy: "UseIP",
		},
	}
}

func DefaultServerConfig() ServerXrayConfig {
	udp := true
	followRedirect := true
	return ServerXrayConfig{
		Inbounds: ServerInboundsConfig{
			Socks: SocksInboundConfig{
				Tag:      "socks-in",
				Protocol: "socks",
				Listen:   "127.0.0.1",
				Port:     51080,
				UDP:      &udp,
			},
			Dokodemo: DokodemoInboundConfig{
				Tag:            "in-48044",
				Remark:         "local-input",
				Protocol:       "dokodemo-door",
				Listen:         "0.0.0.0",
				Port:           48044,
				Network:        "tcp",
				FollowRedirect: &followRedirect,
			},
			Tun: TunInboundConfig{
				Tag:      "tun-in",
				Protocol: "tun",
				Port:     0,
			},
			Trojan: TrojanInboundConfig{
				Listen:        "0.0.0.0",
				Protocol:      "trojan",
				Network:       "tcp",
				Security:      "tls",
				AllowInsecure: false,
				Header: TCPHeader{
					Type: "http",
					Request: TCPRequest{
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
		Logs: defaultLogsConfig("127.0.0.1:52080"),
		Routing: RoutingConfig{
			DomainStrategy: "AsIs",
			Rules:          []map[string]any{},
		},
		DirectOutbound: DirectOutboundConfig{
			Tag:            "direct",
			Protocol:       "freedom",
			DomainStrategy: "UseIP",
		},
	}
}

func defaultLogsConfig(apiListen string) LogsConfig {
	statsEnabled := true
	policyEnabled := true
	return LogsConfig{
		Level:  "warning",
		Access: "none",
		API: APIConfig{
			Tag:      "api",
			Listen:   apiListen,
			Services: []string{"HandlerService", "LoggerService", "StatsService"},
		},
		StatsEnabled: &statsEnabled,
		Policy: PolicyConfig{
			Levels: map[string]PolicyLevel{
				"0": {
					StatsUserDownlink: &policyEnabled,
					StatsUserUplink:   &policyEnabled,
					StatsUserOnline:   &policyEnabled,
				},
			},
			System: PolicySystem{
				StatsInboundDownlink:  &policyEnabled,
				StatsInboundUplink:    &policyEnabled,
				StatsOutboundDownlink: &policyEnabled,
				StatsOutboundUplink:   &policyEnabled,
			},
		},
	}
}

func LoadClientConfig(path string) (ClientXrayConfig, error) {
	cfg := DefaultClientConfig()
	if strings.TrimSpace(path) == "" {
		return ClientXrayConfig{}, errors.New("xrayconfig: config path is empty")
	}
	tree, err := loadExistingToml(path)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"client", "xray"})
	loaded, ok, err := decodeClientConfig(raw)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	if !ok {
		return ClientXrayConfig{}, fmt.Errorf("xrayconfig: client xray config not found in %s", path)
	}
	merged := mergeClientConfig(loaded, cfg)
	if err := validateClientConfig(merged); err != nil {
		return ClientXrayConfig{}, err
	}
	return merged, nil
}

func LoadServerConfig(path string) (ServerXrayConfig, error) {
	cfg := DefaultServerConfig()
	if strings.TrimSpace(path) == "" {
		return ServerXrayConfig{}, errors.New("xrayconfig: config path is empty")
	}
	tree, err := loadExistingToml(path)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"server", "xray"})
	loaded, ok, err := decodeServerConfig(raw)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	if !ok {
		return ServerXrayConfig{}, fmt.Errorf("xrayconfig: server xray config not found in %s", path)
	}
	merged := mergeServerConfig(loaded, cfg)
	if err := validateServerConfig(merged); err != nil {
		return ServerXrayConfig{}, err
	}
	return merged, nil
}

func EnsureClientConfig(path string, auditPath string) (ClientXrayConfig, error) {
	cfg := DefaultClientConfig()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"client", "xray"})
	loaded, ok, err := decodeClientConfig(raw)
	if err != nil {
		return ClientXrayConfig{}, err
	}
	merged := mergeClientConfig(loaded, cfg)
	if err := validateClientConfig(merged); err != nil {
		return ClientXrayConfig{}, err
	}
	if ok && reflect.DeepEqual(loaded, merged) {
		return merged, nil
	}
	if err := writeConfigTree(path, tree, "client", merged, auditPath); err != nil {
		return ClientXrayConfig{}, err
	}
	return merged, nil
}

func EnsureServerConfig(path string, auditPath string) (ServerXrayConfig, error) {
	cfg := DefaultServerConfig()
	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	raw := tree.GetPath([]string{"server", "xray"})
	loaded, ok, err := decodeServerConfig(raw)
	if err != nil {
		return ServerXrayConfig{}, err
	}
	merged := mergeServerConfig(loaded, cfg)
	if err := validateServerConfig(merged); err != nil {
		return ServerXrayConfig{}, err
	}
	if ok && reflect.DeepEqual(loaded, merged) {
		return merged, nil
	}
	if err := writeConfigTree(path, tree, "server", merged, auditPath); err != nil {
		return ServerXrayConfig{}, err
	}
	return merged, nil
}

func SaveClientConfig(path string, auditPath string, cfg ClientXrayConfig) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := validateClientConfig(cfg); err != nil {
		return err
	}
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return err
	}
	return writeConfigTree(path, tree, "client", cfg, auditPath)
}

func SaveServerConfig(path string, auditPath string, cfg ServerXrayConfig) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := validateServerConfig(cfg); err != nil {
		return err
	}
	tree, err := loadOrCreateToml(path)
	if err != nil {
		return err
	}
	return writeConfigTree(path, tree, "server", cfg, auditPath)
}

func validateClientConfig(cfg ClientXrayConfig) error {
	if err := validateSocks(cfg.Inbounds.Socks, "client"); err != nil {
		return err
	}
	if err := validateDokodemo(cfg.Inbounds.Dokodemo, "client"); err != nil {
		return err
	}
	if err := validateTun(cfg.Inbounds.Tun, "client"); err != nil {
		return err
	}
	if err := validateLogs(cfg.Logs, "client"); err != nil {
		return err
	}
	if err := validateRouting(cfg.Routing, "client"); err != nil {
		return err
	}
	if err := validateDirectOutbound(cfg.DirectOutbound, "client"); err != nil {
		return err
	}
	return nil
}

func validateServerConfig(cfg ServerXrayConfig) error {
	if err := validateSocks(cfg.Inbounds.Socks, "server"); err != nil {
		return err
	}
	if err := validateDokodemo(cfg.Inbounds.Dokodemo, "server"); err != nil {
		return err
	}
	if err := validateTun(cfg.Inbounds.Tun, "server"); err != nil {
		return err
	}
	if err := validateTrojan(cfg.Inbounds.Trojan); err != nil {
		return err
	}
	if err := validateLogs(cfg.Logs, "server"); err != nil {
		return err
	}
	if err := validateRouting(cfg.Routing, "server"); err != nil {
		return err
	}
	if err := validateDirectOutbound(cfg.DirectOutbound, "server"); err != nil {
		return err
	}
	return nil
}

func validateSocks(cfg SocksInboundConfig, scope string) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return fmt.Errorf("xrayconfig: %s socks protocol is required", scope)
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		return fmt.Errorf("xrayconfig: %s socks listen is required", scope)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("xrayconfig: %s socks port is invalid", scope)
	}
	return nil
}

func validateDokodemo(cfg DokodemoInboundConfig, scope string) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return fmt.Errorf("xrayconfig: %s dokodemo protocol is required", scope)
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		return fmt.Errorf("xrayconfig: %s dokodemo listen is required", scope)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("xrayconfig: %s dokodemo port is invalid", scope)
	}
	if strings.TrimSpace(cfg.Network) == "" {
		return fmt.Errorf("xrayconfig: %s dokodemo network is required", scope)
	}
	return nil
}

func validateTun(cfg TunInboundConfig, scope string) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return fmt.Errorf("xrayconfig: %s tun protocol is required", scope)
	}
	if strings.TrimSpace(cfg.Tag) == "" {
		return fmt.Errorf("xrayconfig: %s tun tag is required", scope)
	}
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("xrayconfig: %s tun port is invalid", scope)
	}
	return nil
}

func validateTrojan(cfg TrojanInboundConfig) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return errors.New("xrayconfig: trojan protocol is required")
	}
	if strings.TrimSpace(cfg.Listen) == "" {
		return errors.New("xrayconfig: trojan listen is required")
	}
	if strings.TrimSpace(cfg.Network) == "" {
		return errors.New("xrayconfig: trojan network is required")
	}
	if strings.TrimSpace(cfg.Security) == "" {
		return errors.New("xrayconfig: trojan security is required")
	}
	if strings.TrimSpace(cfg.Header.Type) == "" {
		return errors.New("xrayconfig: trojan header type is required")
	}
	if strings.TrimSpace(cfg.Header.Request.Method) == "" {
		return errors.New("xrayconfig: trojan header request method is required")
	}
	return nil
}

func validateLogs(cfg LogsConfig, scope string) error {
	if strings.TrimSpace(cfg.Level) == "" {
		return fmt.Errorf("xrayconfig: %s logs level is required", scope)
	}
	if strings.TrimSpace(cfg.API.Listen) == "" {
		return fmt.Errorf("xrayconfig: %s logs api listen is required", scope)
	}
	if strings.TrimSpace(cfg.API.Tag) == "" {
		return fmt.Errorf("xrayconfig: %s logs api tag is required", scope)
	}
	return nil
}

func validateRouting(cfg RoutingConfig, scope string) error {
	if strings.TrimSpace(cfg.DomainStrategy) == "" {
		return fmt.Errorf("xrayconfig: %s routing domain_strategy is required", scope)
	}
	return nil
}

func validateDirectOutbound(cfg DirectOutboundConfig, scope string) error {
	if strings.TrimSpace(cfg.Protocol) == "" {
		return fmt.Errorf("xrayconfig: %s direct outbound protocol is required", scope)
	}
	if strings.TrimSpace(cfg.Tag) == "" {
		return fmt.Errorf("xrayconfig: %s direct outbound tag is required", scope)
	}
	return nil
}

func mergeClientConfig(current, defaults ClientXrayConfig) ClientXrayConfig {
	merged := current
	merged.Inbounds = mergeClientInbounds(current.Inbounds, defaults.Inbounds)
	merged.Logs = mergeLogs(current.Logs, defaults.Logs)
	merged.Routing = mergeRouting(current.Routing, defaults.Routing)
	merged.DirectOutbound = mergeDirectOutbound(current.DirectOutbound, defaults.DirectOutbound)
	return merged
}

func mergeServerConfig(current, defaults ServerXrayConfig) ServerXrayConfig {
	merged := current
	merged.Inbounds = mergeServerInbounds(current.Inbounds, defaults.Inbounds)
	merged.Logs = mergeLogs(current.Logs, defaults.Logs)
	merged.Routing = mergeRouting(current.Routing, defaults.Routing)
	merged.DirectOutbound = mergeDirectOutbound(current.DirectOutbound, defaults.DirectOutbound)
	return merged
}

func mergeClientInbounds(current, defaults ClientInboundsConfig) ClientInboundsConfig {
	return ClientInboundsConfig{
		Socks:    mergeSocks(current.Socks, defaults.Socks),
		Dokodemo: mergeDokodemo(current.Dokodemo, defaults.Dokodemo),
		Tun:      mergeTun(current.Tun, defaults.Tun),
	}
}

func mergeServerInbounds(current, defaults ServerInboundsConfig) ServerInboundsConfig {
	return ServerInboundsConfig{
		Socks:    mergeSocks(current.Socks, defaults.Socks),
		Dokodemo: mergeDokodemo(current.Dokodemo, defaults.Dokodemo),
		Tun:      mergeTun(current.Tun, defaults.Tun),
		Trojan:   mergeTrojan(current.Trojan, defaults.Trojan),
	}
}

func mergeSocks(current, defaults SocksInboundConfig) SocksInboundConfig {
	merged := current
	if strings.TrimSpace(merged.Tag) == "" {
		merged.Tag = defaults.Tag
	}
	if strings.TrimSpace(merged.Protocol) == "" {
		merged.Protocol = defaults.Protocol
	}
	if strings.TrimSpace(merged.Listen) == "" {
		merged.Listen = defaults.Listen
	}
	if merged.Port <= 0 {
		merged.Port = defaults.Port
	}
	if merged.UDP == nil {
		merged.UDP = defaults.UDP
	}
	return merged
}

func mergeDokodemo(current, defaults DokodemoInboundConfig) DokodemoInboundConfig {
	merged := current
	if strings.TrimSpace(merged.Tag) == "" {
		merged.Tag = defaults.Tag
	}
	if strings.TrimSpace(merged.Remark) == "" {
		merged.Remark = defaults.Remark
	}
	if strings.TrimSpace(merged.Protocol) == "" {
		merged.Protocol = defaults.Protocol
	}
	if strings.TrimSpace(merged.Listen) == "" {
		merged.Listen = defaults.Listen
	}
	if merged.Port <= 0 {
		merged.Port = defaults.Port
	}
	if strings.TrimSpace(merged.Network) == "" {
		merged.Network = defaults.Network
	}
	if merged.FollowRedirect == nil {
		merged.FollowRedirect = defaults.FollowRedirect
	}
	return merged
}

func mergeTun(current, defaults TunInboundConfig) TunInboundConfig {
	merged := current
	if strings.TrimSpace(merged.Tag) == "" {
		merged.Tag = defaults.Tag
	}
	if strings.TrimSpace(merged.Protocol) == "" {
		merged.Protocol = defaults.Protocol
	}
	if merged.Port == 0 {
		merged.Port = defaults.Port
	}
	return merged
}

func mergeTrojan(current, defaults TrojanInboundConfig) TrojanInboundConfig {
	merged := current
	if strings.TrimSpace(merged.Listen) == "" {
		merged.Listen = defaults.Listen
	}
	if strings.TrimSpace(merged.Protocol) == "" {
		merged.Protocol = defaults.Protocol
	}
	if strings.TrimSpace(merged.Network) == "" {
		merged.Network = defaults.Network
	}
	if strings.TrimSpace(merged.Security) == "" {
		merged.Security = defaults.Security
	}
	if !merged.AllowInsecure {
		merged.AllowInsecure = defaults.AllowInsecure
	}
	merged.Header = mergeHeader(merged.Header, defaults.Header)
	return merged
}

func mergeHeader(current, defaults TCPHeader) TCPHeader {
	merged := current
	if strings.TrimSpace(merged.Type) == "" {
		merged.Type = defaults.Type
	}
	merged.Request = mergeRequest(merged.Request, defaults.Request)
	return merged
}

func mergeRequest(current, defaults TCPRequest) TCPRequest {
	merged := current
	if strings.TrimSpace(merged.Version) == "" {
		merged.Version = defaults.Version
	}
	if strings.TrimSpace(merged.Method) == "" {
		merged.Method = defaults.Method
	}
	if len(merged.Path) == 0 {
		merged.Path = defaults.Path
	}
	if len(merged.Headers) == 0 {
		merged.Headers = defaults.Headers
	}
	return merged
}

func mergeLogs(current, defaults LogsConfig) LogsConfig {
	merged := current
	if strings.TrimSpace(merged.Level) == "" {
		merged.Level = defaults.Level
	}
	if strings.TrimSpace(merged.Access) == "" {
		merged.Access = defaults.Access
	}
	merged.API = mergeAPI(merged.API, defaults.API)
	merged.Policy = mergePolicy(merged.Policy, defaults.Policy)
	if merged.StatsEnabled == nil {
		merged.StatsEnabled = defaults.StatsEnabled
	}
	return merged
}

func mergeAPI(current, defaults APIConfig) APIConfig {
	merged := current
	if strings.TrimSpace(merged.Tag) == "" {
		merged.Tag = defaults.Tag
	}
	if strings.TrimSpace(merged.Listen) == "" {
		merged.Listen = defaults.Listen
	}
	if len(merged.Services) == 0 {
		merged.Services = defaults.Services
	}
	return merged
}

func mergePolicy(current, defaults PolicyConfig) PolicyConfig {
	merged := current
	if len(merged.Levels) == 0 {
		merged.Levels = defaults.Levels
	}
	if reflect.DeepEqual(merged.System, PolicySystem{}) {
		merged.System = defaults.System
	}
	return merged
}

func mergeRouting(current, defaults RoutingConfig) RoutingConfig {
	merged := current
	if strings.TrimSpace(merged.DomainStrategy) == "" {
		merged.DomainStrategy = defaults.DomainStrategy
	}
	if merged.Rules == nil {
		merged.Rules = defaults.Rules
	}
	return merged
}

func mergeDirectOutbound(current, defaults DirectOutboundConfig) DirectOutboundConfig {
	merged := current
	if strings.TrimSpace(merged.Tag) == "" {
		merged.Tag = defaults.Tag
	}
	if strings.TrimSpace(merged.Protocol) == "" {
		merged.Protocol = defaults.Protocol
	}
	if strings.TrimSpace(merged.DomainStrategy) == "" {
		merged.DomainStrategy = defaults.DomainStrategy
	}
	if strings.TrimSpace(merged.SendThrough) == "" {
		merged.SendThrough = defaults.SendThrough
	}
	return merged
}

func decodeClientConfig(raw any) (ClientXrayConfig, bool, error) {
	if raw == nil {
		return ClientXrayConfig{}, false, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return ClientXrayConfig{}, false, fmt.Errorf("xrayconfig: encode client xray config: %w", err)
	}
	var cfg ClientXrayConfig
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return ClientXrayConfig{}, false, fmt.Errorf("xrayconfig: decode client xray config: %w", err)
	}
	return cfg, true, nil
}

func decodeServerConfig(raw any) (ServerXrayConfig, bool, error) {
	if raw == nil {
		return ServerXrayConfig{}, false, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return ServerXrayConfig{}, false, fmt.Errorf("xrayconfig: encode server xray config: %w", err)
	}
	var cfg ServerXrayConfig
	if err := json.Unmarshal(buf, &cfg); err != nil {
		return ServerXrayConfig{}, false, fmt.Errorf("xrayconfig: decode server xray config: %w", err)
	}
	return cfg, true, nil
}

func writeConfigTree(path string, tree *toml.Tree, role string, cfg any, auditPath string) error {
	section := strings.ToLower(strings.TrimSpace(role))
	if section == "" {
		return errors.New("xrayconfig: empty role section")
	}
	cfgMap, err := toMap(cfg)
	if err != nil {
		return err
	}
	tree.SetPath([]string{section, "xray"}, cfgMap)
	data, err := toml.Marshal(tree.ToMap())
	if err != nil {
		return fmt.Errorf("xrayconfig: encode toml %s: %w", path, err)
	}
	return configio.WriteBytes(path, data, configio.WriteOptions{
		AuditPath:         auditPath,
		KeepLastKnownGood: false,
	})
}

func toMap(cfg any) (map[string]any, error) {
	buf, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("xrayconfig: encode config: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf, &out); err != nil {
		return nil, fmt.Errorf("xrayconfig: decode config: %w", err)
	}
	return out, nil
}

func loadOrCreateToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tree, err := toml.TreeFromMap(map[string]any{})
			if err != nil {
				return nil, fmt.Errorf("xrayconfig: create empty config tree: %w", err)
			}
			return tree, nil
		}
		return nil, fmt.Errorf("xrayconfig: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		tree, err := toml.TreeFromMap(map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("xrayconfig: create empty config tree: %w", err)
		}
		return tree, nil
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfigParse, path, err)
	}
	return tree, nil
}

func loadExistingToml(path string) (*toml.Tree, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrConfigMissing, path)
		}
		return nil, fmt.Errorf("xrayconfig: read %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrConfigEmpty, path)
	}
	tree, err := toml.LoadBytes(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrConfigParse, path, err)
	}
	return tree, nil
}
