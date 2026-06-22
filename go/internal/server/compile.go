package server

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/controlplane"
	"github.com/NlightN22/xray-p2p/go/internal/extensions"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/tunnel"
	"github.com/NlightN22/xray-p2p/go/internal/version"
	"github.com/NlightN22/xray-p2p/go/internal/xrayassets"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
	"github.com/NlightN22/xray-p2p/go/internal/xrayrule"
)

type runtimeMeta struct {
	Role       string                  `json:"role"`
	Version    string                  `json:"version"`
	CompiledAt time.Time               `json:"compiled_at"`
	TunEnabled bool                    `json:"tun_enabled"`
	TunName    string                  `json:"tun_name"`
	TunMTU     int                     `json:"tun_mtu"`
	TunAddr    string                  `json:"tun_addr"`
	XrayAssets config.XrayAssetsConfig `json:"xray_assets,omitempty"`
	Desired    runtimeDesired          `json:"desired"`
	CertPath   string                  `json:"cert_path,omitempty"`
	KeyPath    string                  `json:"key_path,omitempty"`
	Control    controlplane.Runtime    `json:"control,omitempty"`
}

type runtimeDesired struct {
	Reverse          serverReverseState `json:"reverse,omitempty"`
	Redirects        []redirect.Rule    `json:"redirects,omitempty"`
	RedirectStatuses []redirectStatus   `json:"redirect_statuses,omitempty"`
	Forwards         []forward.Rule     `json:"forwards,omitempty"`
}

type redirectStatus struct {
	CIDR             string `json:"cidr,omitempty"`
	Domain           string `json:"domain,omitempty"`
	OutboundTag      string `json:"outbound_tag"`
	DisabledByPolicy bool   `json:"disabled_by_policy,omitempty"`
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
	if _, err := xrayassets.FromConfig(cfg.XrayAssets); err != nil {
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
		XrayAssets: cfg.XrayAssets,
		Desired: runtimeDesired{
			Reverse: desired.Reverse, Redirects: desired.Redirects, RedirectStatuses: redirectStatuses(desired.Redirects), Forwards: desired.Forwards,
		},
		CertPath: certPath,
		KeyPath:  keyPath,
	}
	control, err := buildControlRuntime(cfg, desired, certPath, keyPath)
	if err != nil {
		return compiledArtifacts{}, err
	}
	meta.Control = control
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

func redirectStatuses(rules []redirect.Rule) []redirectStatus {
	statuses := make([]redirectStatus, 0, len(rules))
	for _, rule := range rules {
		policy, err := rule.AccessPolicy.Normalized()
		if err != nil || policy.Access != "restricted" || len(policy.Users) != 0 {
			continue
		}
		statuses = append(statuses, redirectStatus{CIDR: rule.CIDR, Domain: rule.Domain, OutboundTag: rule.OutboundTag, DisabledByPolicy: true})
	}
	return statuses
}

func buildServerXrayDoc(xrayCfg xrayconfig.ServerXrayConfig, desired desiredServerConfig, cfg config.Config, certPath, keyPath string, snips extensions.Snippets) (map[string]any, error) {
	doc := make(map[string]any)

	logs := buildLogs(xrayCfg.Logs)
	for k, v := range logs {
		doc[k] = v
	}

	profile, err := serverProfile(cfg.Server.Profile)
	if err != nil {
		return nil, err
	}
	if profile == tunnel.ProfileVLESSTLSVision && (certPath == "" || keyPath == "") {
		return nil, fmt.Errorf("VLESS TLS Vision requires a server certificate and key")
	}
	inboundsDoc := buildServerInbounds(xrayCfg, profile, cfg.Server.TunEnabled, cfg.Server.TunName, cfg.Server.TunMTU, parsePortOrDefault(cfg.Server.TrojanPort, DefaultTrojanPort), certPath, keyPath, xrayCfg.Inbounds.Trojan.AllowInsecure, desired.Forwards, activeServerUsers(desired.Users))
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

func serverProfile(raw string) (tunnel.Profile, error) {
	endpoint, err := tunnel.DefaultProfile(tunnel.Profile(strings.TrimSpace(raw)))
	if err != nil {
		return "", err
	}
	return endpoint.Profile, nil
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
	reverseTags := sortedReverseTags(desired.Reverse)
	for _, tag := range reverseTags {
		channel := desired.Reverse[tag]
		reversePortals = append(reversePortals, map[string]any{
			"domain": channel.Domain,
			"tag":    channel.Tag,
		})
		if channel.Disabled {
			continue
		}
		rule := map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.ServerReverse("server", channel.Tag, channel.Domain, channel.UserID),
			"domain":      []string{"full:" + channel.Domain},
			"outboundTag": channel.Tag,
		}
		if users := reverseUserIdentities(channel.UserID); len(users) > 0 {
			rule["user"] = users
		}
		reverseRules = append(reverseRules, rule)
	}
	activeReverse := activeServerReverseRules(desired.Reverse)
	tags := sortedReverseTags(activeReverse)
	for idx, tag := range tags {
		channel := activeReverse[tag]
		markerIP, err := markerIPForIndex(idx)
		if err != nil {
			continue
		}
		markerCIDR := markerIP + "/32"
		markerRules = append(markerRules, map[string]any{
			"type":        "field",
			"ruleTag":     xrayrule.DiagnosticsMarker("server", channel.Tag),
			"ip":          []string{markerCIDR},
			"inboundTag":  []string{cfg.Inbounds.Socks.Tag},
			"port":        fmt.Sprintf("%d", DiagnosticsMarkerPort),
			"outboundTag": channel.Tag,
		})
	}

	activeRedirects := activeServerRedirects(desired.Redirects)
	managedRules := make([]any, 0, len(cfg.Routing.Rules)+len(activeRedirects))
	for _, rule := range cfg.Routing.Rules {
		managedRules = append(managedRules, rule)
	}
	managedRules = append(managedRules, redirect.BuildXrayRules("server", activeRedirects)...)

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

func reverseUserIdentities(label string) []string {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil
	}
	return []string{label, previousTrojanEmail(trojanClient{Email: label})}
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
