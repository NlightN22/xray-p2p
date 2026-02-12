package client

import (
	"path/filepath"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func buildClientInbounds(cfg xrayconfig.ClientXrayConfig, tunEnabled bool, tunName string, tunMTU int) map[string]any {
	inbounds := make([]any, 0, 3)
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
	return map[string]any{
		"inbounds": inbounds,
	}
}

func buildClientInboundsWithForwards(cfg xrayconfig.ClientXrayConfig, tunEnabled bool, tunName string, tunMTU int, forwards []forward.Rule) map[string]any {
	doc := buildClientInbounds(cfg, tunEnabled, tunName, tunMTU)
	raw, _ := doc["inbounds"].([]any)
	for _, rule := range forwards {
		raw = append(raw, rule.InboundMap())
	}
	doc["inbounds"] = raw
	return doc
}

func writeClientInboundsConfig(configDir string, cfg xrayconfig.ClientXrayConfig, tunEnabled bool, tunName string, tunMTU int, forwards []forward.Rule) error {
	path := filepath.Join(configDir, "inbounds.json")
	doc := buildClientInboundsWithForwards(cfg, tunEnabled, tunName, tunMTU, forwards)
	return configio.WriteJSON(path, doc, configio.WriteOptions{
		AuditPath:         config.AuditLogPath(),
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

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}
