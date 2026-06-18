package xrayconfig

import "errors"

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
	Tag           string    `json:"tag" toml:"tag"`
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
