package xrayconfig

import (
	"reflect"
	"strings"
)

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
