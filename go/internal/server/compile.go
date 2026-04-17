package server

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/extensions"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/version"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

type runtimeMeta struct {
	Role       string         `json:"role"`
	Version    string         `json:"version"`
	CompiledAt time.Time      `json:"compiled_at"`
	TunEnabled bool           `json:"tun_enabled"`
	TunName    string         `json:"tun_name"`
	TunMTU     int            `json:"tun_mtu"`
	TunAddr    string         `json:"tun_addr"`
	Desired    runtimeDesired `json:"desired"`
	CertPath   string         `json:"cert_path,omitempty"`
	KeyPath    string         `json:"key_path,omitempty"`
}

type runtimeDesired struct {
	Reverse   serverReverseState `json:"reverse,omitempty"`
	Redirects []redirect.Rule    `json:"redirects,omitempty"`
	Forwards  []forward.Rule     `json:"forwards,omitempty"`
}

type compiledArtifacts struct {
	XrayJSON []byte
	MetaJSON []byte
	Extra    map[string][]byte
}

func compileDesired(configPath string, extensionsDir string) (compiledArtifacts, error) {
	cfg, err := config.Load(config.Options{Path: configPath})
	if err != nil {
		return compiledArtifacts{}, err
	}
	desired, err := loadServerDesiredConfigFromPath(configPath)
	if err != nil {
		return compiledArtifacts{}, err
	}
	xrayCfg, err := xrayconfig.LoadServerConfigWithDefaults(configPath)
	if err != nil {
		return compiledArtifacts{}, err
	}
	snips, err := extensions.Load(extensionsDir)
	if err != nil {
		return compiledArtifacts{}, err
	}

	certPath := strings.TrimSpace(cfg.Server.CertificateFile)
	keyPath := strings.TrimSpace(cfg.Server.KeyFile)
	if certPath == "" && keyPath == "" && defaultTLSConfigured() {
		certPath = defaultCertPath()
		keyPath = defaultKeyPath()
	}
	if certPath == "" || keyPath == "" {
		certPath = ""
		keyPath = ""
	}

	doc, err := buildServerXrayDoc(xrayCfg, desired, cfg, certPath, keyPath, snips)
	if err != nil {
		return compiledArtifacts{}, err
	}
	xrayBytes, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return compiledArtifacts{}, fmt.Errorf("encode xray.json: %w", err)
	}
	xrayBytes = append(xrayBytes, '\n')

	meta := runtimeMeta{
		Role:       "server",
		Version:    version.Current(),
		CompiledAt: time.Now().UTC(),
		TunEnabled: cfg.Server.TunEnabled,
		TunName:    strings.TrimSpace(cfg.Server.TunName),
		TunMTU:     cfg.Server.TunMTU,
		TunAddr:    strings.TrimSpace(cfg.Server.TunAddr),
		Desired: runtimeDesired{
			Reverse:   desired.Reverse,
			Redirects: desired.Redirects,
			Forwards:  desired.Forwards,
		},
		CertPath: certPath,
		KeyPath:  keyPath,
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return compiledArtifacts{}, fmt.Errorf("encode runtime metadata: %w", err)
	}
	metaBytes = append(metaBytes, '\n')

	return compiledArtifacts{
		XrayJSON: xrayBytes,
		MetaJSON: metaBytes,
		Extra:    map[string][]byte{},
	}, nil
}

func buildServerXrayDoc(xrayCfg xrayconfig.ServerXrayConfig, desired desiredServerConfig, cfg config.Config, certPath, keyPath string, snips extensions.Snippets) (map[string]any, error) {
	doc := make(map[string]any)

	logs := buildLogs(xrayCfg.Logs)
	for k, v := range logs {
		doc[k] = v
	}

	inboundsDoc := buildServerInbounds(xrayCfg, cfg.Server.TunEnabled, cfg.Server.TunName, cfg.Server.TunMTU, parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort), certPath, keyPath, xrayCfg.Inbounds.Trojan.AllowInsecure, desired.Forwards, desired.Users)
	inbounds, _ := inboundsDoc["inbounds"].([]any)

	outbounds := buildServerOutbounds(xrayCfg.DirectOutbound)

	inboundTags := collectTags(inbounds)
	outboundTags := collectTags(outbounds)

	if err := extensions.ValidateAppendTags("inbounds.append", snips.InboundsAppend, inboundTags); err != nil {
		return nil, err
	}
	if err := extensions.ValidateAppendTags("outbounds.append", snips.OutboundsAppend, outboundTags); err != nil {
		return nil, err
	}
	inbounds = append(inbounds, snips.InboundsAppend...)
	outbounds = append(outbounds, snips.OutboundsAppend...)

	routingDoc := buildServerRoutingWithSnips(xrayCfg, desired, snips)

	doc["inbounds"] = inbounds
	doc["outbounds"] = outbounds
	for k, v := range routingDoc {
		doc[k] = v
	}

	return doc, nil
}

func buildServerOutbounds(cfg xrayconfig.DirectOutboundConfig) []any {
	sendThrough := strings.TrimSpace(cfg.SendThrough)
	randomTag := strings.TrimSpace(cfg.Tag)
	udpTag := ""
	if runtime.GOOS == "windows" {
		randomTag = directRandomTagWindows
		udpTag = directUDPTagWindows
	}
	outbounds := make([]any, 0, 2)
	if randomTag != "" {
		ob := map[string]any{
			"protocol": cfg.Protocol,
			"tag":      randomTag,
		}
		if sendThrough != "" && udpTag == "" {
			ob["sendThrough"] = sendThrough
		}
		outbounds = append(outbounds, ob)
	}
	if udpTag != "" {
		ob := map[string]any{
			"protocol": cfg.Protocol,
			"tag":      udpTag,
		}
		if sendThrough != "" {
			ob["sendThrough"] = sendThrough
		}
		outbounds = append(outbounds, ob)
	}
	return outbounds
}

func buildServerRoutingWithSnips(cfg xrayconfig.ServerXrayConfig, desired desiredServerConfig, snips extensions.Snippets) map[string]any {
	reversePortals := make([]any, 0, len(desired.Reverse))
	reverseRules := make([]any, 0, len(desired.Reverse))
	markerRules := make([]any, 0, len(desired.Reverse)*2)
	for _, channel := range desired.Reverse {
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
	tags := sortedReverseTags(desired.Reverse)
	for idx, tag := range tags {
		channel := desired.Reverse[tag]
		markerIP, err := markerIPForIndex(idx)
		if err != nil {
			continue
		}
		markerCIDR := markerIP + "/32"
		markerRules = append(markerRules, map[string]any{
			"type":        "field",
			"ip":          []string{markerCIDR},
			"inboundTag":  []string{cfg.Inbounds.Socks.Tag},
			"port":        fmt.Sprintf("%d", DiagnosticsMarkerPort),
			"outboundTag": channel.Tag,
		})
	}

	managedRules := make([]any, 0, len(cfg.Routing.Rules)+len(desired.Redirects))
	for _, rule := range cfg.Routing.Rules {
		managedRules = append(managedRules, rule)
	}
	for _, rule := range desired.Redirects {
		entry := map[string]any{
			"type":        "field",
			"outboundTag": rule.OutboundTag,
		}
		if rule.Kind() == redirect.KindDomain {
			entry["domains"] = []string{rule.Value()}
		} else {
			entry["ip"] = []string{rule.Value()}
		}
		managedRules = append(managedRules, entry)
	}

	rules := make([]any, 0, len(markerRules)+len(reverseRules)+len(snips.RoutingAfterSystem)+len(managedRules)+len(snips.RoutingAfterManaged))
	rules = append(rules, markerRules...)
	rules = append(rules, reverseRules...)
	rules = append(rules, snips.RoutingAfterSystem...)
	rules = append(rules, managedRules...)
	rules = append(rules, snips.RoutingAfterManaged...)
	if runtime.GOOS == "windows" {
		rules = applyWindowsDirectRules(rules)
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

func collectTags(items []any) map[string]struct{} {
	tags := make(map[string]struct{})
	for _, raw := range items {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := obj["tag"].(string)
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		tags[tag] = struct{}{}
	}
	return tags
}
