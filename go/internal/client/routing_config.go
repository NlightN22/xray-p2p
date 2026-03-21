package client

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func updateRoutingConfig(path string, cfg xrayconfig.RoutingConfig, endpoints []clientEndpointRecord, redirects []redirect.Rule, reverse map[string]clientReverseChannel) error {
	document := make(map[string]any)
	routing := ensureObject(document, "routing")
	routing["domainStrategy"] = strings.TrimSpace(cfg.DomainStrategy)
	existing := []any{}
	for _, rule := range cfg.Rules {
		existing = append(existing, rule)
	}
	managed := managedOutboundTags(endpoints, redirects)
	for _, rule := range existing {
		ruleMap, ok := rule.(map[string]any)
		if !ok {
			continue
		}
		outbound, _ := ruleMap["outboundTag"].(string)
		trimmedOutbound := strings.ToLower(strings.TrimSpace(outbound))
		if strings.HasPrefix(trimmedOutbound, "proxy-") {
			managed[trimmedOutbound] = struct{}{}
		}
	}

	filtered := filterManagedRules(existing, managed)
	filtered = filterReverseRules(filtered, reverse)
	filtered = filterEndpointBypassRules(filtered, endpoints)
	ruleBuckets := map[routingRuleClass][]any{
		routingRuleEndpointBypass: {},
		routingRuleSystem:         {},
		routingRuleRedirect:       {},
		routingRuleUser:           filtered,
	}

	for _, ep := range endpoints {
		ruleBuckets[routingRuleEndpointBypass] = append(ruleBuckets[routingRuleEndpointBypass], endpointBypassRule(ep))
	}
	ruleBuckets[routingRuleSystem] = append(ruleBuckets[routingRuleSystem], buildClientReverseRules(reverse)...)
	for idx, ep := range endpoints {
		markerIP, err := markerIPForIndex(idx)
		if err != nil {
			return fmt.Errorf("xp2p: allocate diagnostics marker for %s: %w", ep.Tag, err)
		}
		markerCIDR := markerIP + "/32"
		ruleBuckets[routingRuleSystem] = append(ruleBuckets[routingRuleSystem], map[string]any{
			"type":        "field",
			"ip":          []string{markerCIDR},
			"port":        fmt.Sprintf("%d", DiagnosticsMarkerPort),
			"outboundTag": ep.Tag,
		})
	}
	for _, ep := range endpoints {
		ruleBuckets[routingRuleSystem] = append(ruleBuckets[routingRuleSystem], endpointRouteRule(ep))
	}
	for _, rule := range redirects {
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
		ruleBuckets[routingRuleRedirect] = append(ruleBuckets[routingRuleRedirect], entry)
	}
	if runtime.GOOS == "windows" {
		ruleBuckets[routingRuleUser] = filterWindowsDirectRules(ruleBuckets[routingRuleUser])
		ruleBuckets[routingRuleSystem] = append(ruleBuckets[routingRuleSystem], windowsDirectRules()...)
	}
	orderedClasses := []routingRuleClass{
		routingRuleEndpointBypass,
		routingRuleSystem,
		routingRuleRedirect,
		routingRuleUser,
	}
	orderedRules := make([]any, 0, len(existing))
	for _, class := range orderedClasses {
		orderedRules = append(orderedRules, ruleBuckets[class]...)
	}
	routing["rules"] = orderedRules

	reverseObj := ensureObject(document, "reverse")
	updateReverseBridges(reverseObj, sortedReverseChannels(reverse))

	if err := configio.WriteJSON(path, document, configio.WriteOptions{
		AuditPath:         config.AuditLogPath(),
		KeepLastKnownGood: true,
	}); err != nil {
		return err
	}
	return nil
}
