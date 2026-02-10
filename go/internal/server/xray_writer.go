package server

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/layout"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func writeServerInboundsConfig(configDir string, cfg xrayconfig.ServerXrayConfig, tunEnabled bool, tunName string, tunMTU int, trojanPort int, certPath string, keyPath string, allowInsecure bool, forwards []forward.Rule) error {
	doc := buildServerInbounds(cfg, tunEnabled, tunName, tunMTU, trojanPort, certPath, keyPath, allowInsecure, forwards)
	return configio.WriteJSON(filepath.Join(configDir, "inbounds.json"), doc, configio.WriteOptions{
		AuditPath:         config.ConfigPath(layout.AuditLogFileName),
		KeepLastKnownGood: true,
	})
}

func buildServerInbounds(cfg xrayconfig.ServerXrayConfig, tunEnabled bool, tunName string, tunMTU int, trojanPort int, certPath string, keyPath string, allowInsecure bool, forwards []forward.Rule) map[string]any {
	inbounds := make([]any, 0, 4+len(forwards))
	if tunEnabled {
		inbounds = append(inbounds, map[string]any{
			"tag":      cfg.Inbounds.Tun.Tag,
			"port":     cfg.Inbounds.Tun.Port,
			"protocol": cfg.Inbounds.Tun.Protocol,
			"settings": map[string]any{
				"name": tunName,
				"mtu":  tunMTU,
			},
		})
	}
	inbounds = append(inbounds,
		map[string]any{
			"tag":      cfg.Inbounds.Socks.Tag,
			"protocol": cfg.Inbounds.Socks.Protocol,
			"listen":   cfg.Inbounds.Socks.Listen,
			"port":     cfg.Inbounds.Socks.Port,
			"settings": map[string]any{
				"udp": boolValue(cfg.Inbounds.Socks.UDP, true),
			},
		},
		map[string]any{
			"remark":   cfg.Inbounds.Dokodemo.Remark,
			"tag":      cfg.Inbounds.Dokodemo.Tag,
			"listen":   cfg.Inbounds.Dokodemo.Listen,
			"port":     cfg.Inbounds.Dokodemo.Port,
			"protocol": cfg.Inbounds.Dokodemo.Protocol,
			"settings": map[string]any{
				"network":        cfg.Inbounds.Dokodemo.Network,
				"followRedirect": boolValue(cfg.Inbounds.Dokodemo.FollowRedirect, true),
			},
		},
	)
	inbounds = append(inbounds, buildTrojanInbound(cfg, trojanPort, certPath, keyPath, allowInsecure))
	for _, rule := range forwards {
		inbounds = append(inbounds, rule.InboundMap())
	}
	return map[string]any{
		"inbounds": inbounds,
	}
}

func buildTrojanInbound(cfg xrayconfig.ServerXrayConfig, trojanPort int, certPath string, keyPath string, forceAllowInsecure bool) map[string]any {
	security := strings.TrimSpace(cfg.Inbounds.Trojan.Security)
	if security == "" {
		if certPath == "" {
			security = "none"
		} else {
			security = "tls"
		}
	}
	allowInsecure := cfg.Inbounds.Trojan.AllowInsecure || forceAllowInsecure
	stream := map[string]any{
		"network": cfg.Inbounds.Trojan.Network,
		"tcpSettings": map[string]any{
			"header": map[string]any{
				"type": cfg.Inbounds.Trojan.Header.Type,
				"request": map[string]any{
					"version": cfg.Inbounds.Trojan.Header.Request.Version,
					"method":  cfg.Inbounds.Trojan.Header.Request.Method,
					"path":    cfg.Inbounds.Trojan.Header.Request.Path,
					"headers": cfg.Inbounds.Trojan.Header.Request.Headers,
				},
			},
		},
	}
	if strings.EqualFold(security, "tls") {
		tlsSettings := map[string]any{
			"certificates": []map[string]any{
				{
					"certificateFile": certPath,
					"keyFile":         keyPath,
				},
			},
		}
		if allowInsecure {
			tlsSettings["allowInsecure"] = true
		}
		stream["security"] = "tls"
		stream["tlsSettings"] = tlsSettings
	} else {
		stream["security"] = "none"
	}
	return map[string]any{
		"port":     trojanPort,
		"listen":   cfg.Inbounds.Trojan.Listen,
		"protocol": cfg.Inbounds.Trojan.Protocol,
		"settings": map[string]any{
			"clients": []any{},
		},
		"streamSettings": stream,
	}
}

func writeServerLogs(configDir string, cfg xrayconfig.LogsConfig) error {
	doc := buildLogs(cfg)
	return configio.WriteJSON(filepath.Join(configDir, "logs.json"), doc, configio.WriteOptions{
		AuditPath:         config.ConfigPath(layout.AuditLogFileName),
		KeepLastKnownGood: true,
	})
}

func buildLogs(cfg xrayconfig.LogsConfig) map[string]any {
	doc := map[string]any{
		"log": map[string]any{
			"loglevel": cfg.Level,
			"access":   cfg.Access,
		},
		"api": map[string]any{
			"tag":      cfg.API.Tag,
			"listen":   cfg.API.Listen,
			"services": cfg.API.Services,
		},
	}
	statsEnabled := boolValue(cfg.StatsEnabled, true)
	if statsEnabled {
		level := cfg.Policy.Levels["0"]
		doc["stats"] = map[string]any{}
		doc["policy"] = map[string]any{
			"levels": map[string]any{
				"0": map[string]any{
					"statsUserDownlink": boolValue(level.StatsUserDownlink, true),
					"statsUserUplink":   boolValue(level.StatsUserUplink, true),
					"statsUserOnline":   boolValue(level.StatsUserOnline, true),
				},
			},
			"system": map[string]any{
				"statsInboundDownlink":  boolValue(cfg.Policy.System.StatsInboundDownlink, true),
				"statsInboundUplink":    boolValue(cfg.Policy.System.StatsInboundUplink, true),
				"statsOutboundDownlink": boolValue(cfg.Policy.System.StatsOutboundDownlink, true),
				"statsOutboundUplink":   boolValue(cfg.Policy.System.StatsOutboundUplink, true),
			},
		}
	}
	return doc
}

func writeServerOutbounds(configDir string, cfg xrayconfig.DirectOutboundConfig) error {
	doc := map[string]any{
		"outbounds": []any{
			map[string]any{
				"protocol": cfg.Protocol,
				"tag":      cfg.Tag,
			},
		},
	}
	return configio.WriteJSON(filepath.Join(configDir, "outbounds.json"), doc, configio.WriteOptions{
		AuditPath:         config.ConfigPath(layout.AuditLogFileName),
		KeepLastKnownGood: true,
	})
}

func writeServerRouting(configDir string, cfg xrayconfig.ServerXrayConfig, reverse serverReverseState, redirects []redirect.Rule) error {
	doc := buildServerRouting(cfg, reverse, redirects)
	return configio.WriteJSON(filepath.Join(configDir, "routing.json"), doc, configio.WriteOptions{
		AuditPath:         config.ConfigPath(layout.AuditLogFileName),
		KeepLastKnownGood: true,
	})
}

func buildServerRouting(cfg xrayconfig.ServerXrayConfig, reverse serverReverseState, redirects []redirect.Rule) map[string]any {
	reversePortals := make([]any, 0, len(reverse))
	reverseRules := make([]any, 0, len(reverse))
	markerRules := make([]any, 0, len(reverse)*2)
	for _, channel := range reverse {
		reversePortals = append(reversePortals, map[string]any{
			"domain": channel.Domain,
			"tag":    channel.Tag,
		})
		rule := map[string]any{
			"type":        "field",
			"domain":      []string{"full:" + channel.Domain},
			"outboundTag": channel.Tag,
		}
		if strings.TrimSpace(channel.UserID) != "" {
			rule["user"] = []string{channel.UserID}
		}
		reverseRules = append(reverseRules, rule)
	}
	tags := sortedReverseTags(reverse)
	for idx, tag := range tags {
		channel := reverse[tag]
		markerIP, err := markerIPForIndex(idx)
		if err != nil {
			continue
		}
		markerCIDR := markerIP + "/32"
		markerRules = append(markerRules, map[string]any{
			"type":        "field",
			"domain":      []string{"full:" + markerIP},
			"inboundTag":  []string{cfg.Inbounds.Socks.Tag},
			"port":        fmt.Sprintf("%d", DiagnosticsMarkerPort),
			"outboundTag": channel.Tag,
		})
		markerRules = append(markerRules, map[string]any{
			"type":        "field",
			"ip":          []string{markerCIDR},
			"inboundTag":  []string{cfg.Inbounds.Socks.Tag},
			"port":        fmt.Sprintf("%d", DiagnosticsMarkerPort),
			"outboundTag": channel.Tag,
		})
	}

	rules := make([]any, 0, len(cfg.Routing.Rules)+len(reverseRules)+len(redirects))
	for _, rule := range cfg.Routing.Rules {
		rules = append(rules, rule)
	}
	rules = append(rules, markerRules...)
	rules = append(rules, reverseRules...)
	for _, rule := range redirects {
		entry := map[string]any{
			"type":        "field",
			"outboundTag": rule.OutboundTag,
		}
		if rule.Kind() == redirect.KindDomain {
			entry["domains"] = []string{rule.Value()}
		} else {
			entry["ip"] = []string{rule.Value()}
		}
		rules = append(rules, entry)
	}

	return map[string]any{
		"reverse": map[string]any{
			"portals": reversePortals,
		},
		"routing": map[string]any{
			"domainStrategy": strings.TrimSpace(cfg.Routing.DomainStrategy),
			"rules":          rules,
		},
	}
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
