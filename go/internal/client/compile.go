package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/extensions"
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
	TunMode    string         `json:"tun_mode"`
	DNSServers []string       `json:"dns_servers,omitempty"`
	FullTag    string         `json:"full_tunnel_tag,omitempty"`
	Desired    runtimeDesired `json:"desired"`
}

type runtimeDesired struct {
	Endpoints []runtimeEndpoint `json:"endpoints,omitempty"`
	Redirects []redirect.Rule   `json:"redirects,omitempty"`
}

type runtimeEndpoint struct {
	Hostname string `json:"hostname,omitempty"`
	Address  string `json:"address,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Port     int    `json:"port,omitempty"`
	User     string `json:"user,omitempty"`
}

type compiledArtifacts struct {
	XrayJSON []byte
	MetaJSON []byte
}

func compileDesired(configPath string, extensionsDir string) (compiledArtifacts, error) {
	cfg, err := config.Load(config.Options{Path: configPath})
	if err != nil {
		return compiledArtifacts{}, err
	}
	desired, err := loadClientInstallState(configPath)
	if err != nil {
		return compiledArtifacts{}, err
	}
	xrayCfg, err := xrayconfig.LoadClientConfigWithDefaults(configPath)
	if err != nil {
		return compiledArtifacts{}, err
	}
	snips, err := extensions.Load(extensionsDir)
	if err != nil {
		return compiledArtifacts{}, err
	}

	fullEnabled := cfg.Client.TunEnabled && strings.EqualFold(strings.TrimSpace(cfg.Client.TunMode), "full")
	endpointIPs, err := resolveEndpointIPMapWithCache(context.Background(), desired.Endpoints)
	if err != nil {
		return compiledArtifacts{}, err
	}

	base, err := buildClientXrayDoc(xrayCfg, desired, endpointIPs, cfg, snips, fullEnabled)
	if err != nil {
		return compiledArtifacts{}, err
	}

	xrayBytes, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return compiledArtifacts{}, fmt.Errorf("encode xray.json: %w", err)
	}
	xrayBytes = append(xrayBytes, '\n')

	meta := runtimeMeta{
		Role:       "client",
		Version:    version.Current(),
		CompiledAt: time.Now().UTC(),
		TunEnabled: cfg.Client.TunEnabled,
		TunName:    strings.TrimSpace(cfg.Client.TunName),
		TunMTU:     cfg.Client.TunMTU,
		TunAddr:    strings.TrimSpace(cfg.Client.TunAddr),
		TunMode:    strings.TrimSpace(cfg.Client.TunMode),
		DNSServers: cfg.Client.DNSServers,
		FullTag:    strings.TrimSpace(cfg.Client.FullTunnelTag),
		Desired: runtimeDesired{
			Endpoints: sanitizeRuntimeEndpoints(desired.Endpoints),
			Redirects: desired.Redirects,
		},
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return compiledArtifacts{}, fmt.Errorf("encode runtime metadata: %w", err)
	}
	metaBytes = append(metaBytes, '\n')

	return compiledArtifacts{XrayJSON: xrayBytes, MetaJSON: metaBytes}, nil
}

func sanitizeRuntimeEndpoints(endpoints []clientEndpointRecord) []runtimeEndpoint {
	if len(endpoints) == 0 {
		return nil
	}
	out := make([]runtimeEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		out = append(out, runtimeEndpoint{
			Hostname: strings.TrimSpace(ep.Hostname),
			Address:  strings.TrimSpace(ep.Address),
			Tag:      strings.TrimSpace(ep.Tag),
			Port:     ep.Port,
			User:     strings.TrimSpace(ep.User),
		})
	}
	return out
}

func buildClientXrayDoc(xrayCfg xrayconfig.ClientXrayConfig, desired clientInstallState, endpointIPs map[string]fullTunnelEndpointIPs, cfg config.Config, snips extensions.Snippets, fullTunnelEnabled bool) (map[string]any, error) {
	doc := make(map[string]any)

	logs := buildLogs(xrayCfg.Logs)
	for k, v := range logs {
		doc[k] = v
	}

	inboundsDoc := buildClientInboundsWithForwards(xrayCfg, cfg.Client.TunEnabled, cfg.Client.TunName, cfg.Client.TunMTU, desired.Forwards)
	inbounds, _ := inboundsDoc["inbounds"].([]any)
	outbounds, err := buildClientOutbounds(xrayCfg.DirectOutbound, desired, endpointIPs, fullTunnelEnabled)
	if err != nil {
		return nil, err
	}

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

	routing, reverse, err := buildClientRouting(xrayCfg.Routing, desired, endpointIPs, fullTunnelEnabled, cfg.Client.FullTunnelTag, snips)
	if err != nil {
		return nil, err
	}

	doc["inbounds"] = inbounds
	doc["outbounds"] = outbounds
	if len(reverse) > 0 {
		doc["reverse"] = reverse
	}
	doc["routing"] = routing

	return doc, nil
}

func buildClientOutbounds(direct xrayconfig.DirectOutboundConfig, desired clientInstallState, endpointIPs map[string]fullTunnelEndpointIPs, fullTunnelEnabled bool) ([]any, error) {
	requireEndpointIPs := fullTunnelEnabled
	outbounds := make([]any, 0, len(desired.Endpoints)+1)
	for _, ep := range desired.Endpoints {
		outbound, err := trojanOutbound(ep, endpointIPs, requireEndpointIPs)
		if err != nil {
			return nil, err
		}
		outbounds = append(outbounds, outbound)
	}
	if tag := strings.TrimSpace(direct.Tag); tag != "" {
		outbounds = append(outbounds, freedomOutbound(tag, direct, direct.SendThrough))
	}
	return outbounds, nil
}

func buildClientRouting(cfg xrayconfig.RoutingConfig, desired clientInstallState, endpointIPs map[string]fullTunnelEndpointIPs, fullTunnelEnabled bool, fullTunnelTag string, snips extensions.Snippets) (map[string]any, map[string]any, error) {
	routing := map[string]any{
		"domainStrategy": strings.TrimSpace(cfg.DomainStrategy),
	}

	ensureIPs := fullTunnelEnabled
	bypassRules, err := endpointBypassRules(desired.Endpoints, endpointIPs, ensureIPs)
	if err != nil {
		return nil, nil, err
	}

	systemRules := make([]any, 0, len(desired.Endpoints)+len(desired.Reverse)*2)
	systemRules = append(systemRules, buildClientReverseRules(desired.Reverse)...)
	for idx, ep := range desired.Endpoints {
		markerIP, err := markerIPForIndex(idx)
		if err != nil {
			return nil, nil, fmt.Errorf("allocate diagnostics marker for %s: %w", ep.Tag, err)
		}
		markerCIDR := markerIP + "/32"
		systemRules = append(systemRules, map[string]any{
			"type":        "field",
			"ip":          []string{markerCIDR},
			"port":        fmt.Sprintf("%d", DiagnosticsMarkerPort),
			"outboundTag": ep.Tag,
		})
	}

	managedRules := make([]any, 0, len(cfg.Rules)+len(desired.Redirects))
	for _, rule := range cfg.Rules {
		managedRules = append(managedRules, rule)
	}
	for _, rule := range desired.Redirects {
		entry := map[string]any{
			"type":        "field",
			"outboundTag": rule.OutboundTag,
		}
		switch rule.Kind() {
		case redirect.KindDomain:
			entry["domains"] = []string{rule.Value()}
		default:
			entry["ip"] = []string{rule.Value()}
		}
		managedRules = append(managedRules, entry)
	}

	rules := make([]any, 0, len(bypassRules)+len(systemRules)+len(snips.RoutingAfterSystem)+len(managedRules)+len(snips.RoutingAfterManaged)+1)
	rules = append(rules, bypassRules...)
	rules = append(rules, systemRules...)
	rules = append(rules, snips.RoutingAfterSystem...)
	rules = append(rules, managedRules...)
	rules = append(rules, snips.RoutingAfterManaged...)
	if fullTunnelEnabled {
		if rule := fullTunnelRule(fullTunnelTag); rule != nil {
			rules = append(rules, rule)
		}
	}
	routing["rules"] = rules

	reverseObj := make(map[string]any)
	updateReverseBridges(reverseObj, sortedReverseChannels(desired.Reverse))
	if len(reverseObj) == 0 {
		reverseObj = nil
	}

	return routing, reverseObj, nil
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
