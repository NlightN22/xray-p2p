package server

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

const (
	directRandomTagWindows = "direct-random"
	directUDPTagWindows    = "direct-udp"
)

func buildServerInbounds(cfg xrayconfig.ServerXrayConfig, profile tunnel.Profile, tunEnabled bool, tunName string, tunMTU int, trojanPort int, certPath string, keyPath string, forceAllowInsecure bool, forwards []forward.Rule, clients []trojanClient) map[string]any {
	inbounds := make([]any, 0, 3+len(forwards))
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
	inbounds = append(inbounds, buildTunnelInbound(cfg, profile, trojanPort, certPath, keyPath, forceAllowInsecure, clients))
	for _, rule := range forwards {
		inbounds = append(inbounds, rule.InboundMap())
	}
	return map[string]any{
		"inbounds": inbounds,
	}
}

func buildTunnelInbound(cfg xrayconfig.ServerXrayConfig, profile tunnel.Profile, port int, certPath, keyPath string, forceAllowInsecure bool, clients []trojanClient) map[string]any {
	if profile != tunnel.ProfileVLESSTLSVision {
		return buildTrojanInbound(cfg, port, certPath, keyPath, forceAllowInsecure, clients)
	}
	users := make([]any, 0, len(clients))
	for _, client := range clients {
		if client.Disabled {
			continue
		}
		users = append(users, map[string]any{"id": strings.TrimSpace(client.Password), "email": strings.TrimSpace(client.Email), "flow": "xtls-rprx-vision"})
	}
	return map[string]any{
		"tag":      cfg.Inbounds.Trojan.Tag,
		"port":     port,
		"listen":   cfg.Inbounds.Trojan.Listen,
		"protocol": "vless",
		"settings": map[string]any{"clients": users, "decryption": "none"},
		"streamSettings": map[string]any{
			"network": "tcp", "security": "tls",
			"tlsSettings": map[string]any{"certificates": []map[string]any{{"certificateFile": certPath, "keyFile": keyPath}}},
		},
	}
}

func buildTrojanInbound(cfg xrayconfig.ServerXrayConfig, trojanPort int, certPath string, keyPath string, forceAllowInsecure bool, clients []trojanClient) map[string]any {
	security := strings.TrimSpace(cfg.Inbounds.Trojan.Security)
	if security == "" {
		if certPath == "" || keyPath == "" {
			security = "none"
		} else {
			security = "tls"
		}
	}
	stream := map[string]any{
		"network": cfg.Inbounds.Trojan.Network,
		"tcpSettings": map[string]any{
			"acceptProxyProtocol": false,
			"header": map[string]any{
				"type": "none",
			},
		},
	}
	if strings.EqualFold(security, "tls") {
		stream["security"] = "tls"
		stream["tlsSettings"] = map[string]any{
			"certificates": []map[string]any{
				{
					"certificateFile": certPath,
					"keyFile":         keyPath,
				},
			},
		}
	} else {
		stream["security"] = "none"
	}
	if forceAllowInsecure {
		cfg.Inbounds.Trojan.AllowInsecure = true
	}
	return map[string]any{
		"tag":      cfg.Inbounds.Trojan.Tag,
		"port":     trojanPort,
		"listen":   cfg.Inbounds.Trojan.Listen,
		"protocol": cfg.Inbounds.Trojan.Protocol,
		"settings": map[string]any{
			"clients": clientsToInterfaces(clients),
		},
		"streamSettings": stream,
	}
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
			"services": xrayconfig.SupportedAPIServices(cfg.API.Services),
		},
	}
	statsEnabled := boolValue(cfg.StatsEnabled, true)
	if statsEnabled {
		level := cfg.Policy.Levels["0"]
		doc["stats"] = map[string]any{}
		doc["policy"] = map[string]any{
			"levels": map[string]any{
				"0": map[string]any{
					"statsUserUplink":   boolValue(level.StatsUserUplink, true),
					"statsUserDownlink": boolValue(level.StatsUserDownlink, true),
					"statsUserOnline":   boolValue(level.StatsUserOnline, true),
				},
			},
			"system": map[string]any{
				"statsInboundUplink":    boolValue(cfg.Policy.System.StatsInboundUplink, true),
				"statsInboundDownlink":  boolValue(cfg.Policy.System.StatsInboundDownlink, true),
				"statsOutboundUplink":   boolValue(cfg.Policy.System.StatsOutboundUplink, true),
				"statsOutboundDownlink": boolValue(cfg.Policy.System.StatsOutboundDownlink, true),
			},
		}
	}
	return doc
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
