package client

import (
	"runtime"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/config"
	"github.com/NlightN22/xray-p2p/go/internal/configio"
	"github.com/NlightN22/xray-p2p/go/internal/redirect"
	"github.com/NlightN22/xray-p2p/go/internal/xrayconfig"
)

func updateRoutingConfig(path string, cfg xrayconfig.RoutingConfig, endpoints []clientEndpointRecord, redirects []redirect.Rule, reverse map[string]clientReverseChannel, fullTunnelEnabled bool, fullTunnelTag string, endpointIPs map[string]fullTunnelEndpointIPs, requireEndpointIPs bool) error {
	document := make(map[string]any)
	routing := ensureObject(document, "routing")
	routing["domainStrategy"] = strings.TrimSpace(cfg.DomainStrategy)
	activeEndpoints := activeClientEndpoints(endpoints)
	activeRedirects := activeClientRedirects(redirects, endpoints)
	activeReverseRules := activeClientReverseForRules(reverse, endpoints)
	existing := []any{}
	for _, rule := range cfg.Rules {
		existing = append(existing, rule)
	}
	managed := managedOutboundTags(activeEndpoints, activeRedirects)
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
	filtered = filterReverseRules(filtered, activeReverseRules)
	filtered, err := filterEndpointBypassRules(filtered, activeEndpoints, endpointIPs, fullTunnelEnabled && requireEndpointIPs)
	if err != nil {
		return err
	}
	ruleBuckets := map[routingRuleClass][]any{
		routingRuleEndpointBypass: {},
		routingRuleSystem:         {},
		routingRuleRedirect:       {},
		routingRuleUser:           filtered,
		routingRuleFullTunnel:     {},
	}

	ensureIPs := fullTunnelEnabled && requireEndpointIPs
	bypassRules, err := endpointBypassRules(activeEndpoints, endpointIPs, ensureIPs)
	if err != nil {
		return err
	}
	ruleBuckets[routingRuleEndpointBypass] = append(ruleBuckets[routingRuleEndpointBypass], bypassRules...)
	ruleBuckets[routingRuleSystem] = append(ruleBuckets[routingRuleSystem], buildClientReverseRules(activeReverseRules)...)
	markerRules, err := diagnosticsMarkerRules(endpoints)
	if err != nil {
		return err
	}
	ruleBuckets[routingRuleSystem] = append(ruleBuckets[routingRuleSystem], markerRules...)
	ruleBuckets[routingRuleRedirect] = append(ruleBuckets[routingRuleRedirect], redirect.BuildXrayRules("client", activeRedirects)...)
	if runtime.GOOS == "windows" {
		ruleBuckets[routingRuleUser] = filterWindowsDirectRules(ruleBuckets[routingRuleUser])
	}
	if fullTunnelEnabled {
		if rule := fullTunnelRule(fullTunnelTag); rule != nil {
			ruleBuckets[routingRuleFullTunnel] = append(ruleBuckets[routingRuleFullTunnel], rule)
		}
	}
	orderedClasses := []routingRuleClass{
		routingRuleEndpointBypass,
		routingRuleSystem,
		routingRuleRedirect,
		routingRuleUser,
		routingRuleFullTunnel,
	}
	orderedRules := make([]any, 0, len(existing))
	for _, class := range orderedClasses {
		orderedRules = append(orderedRules, ruleBuckets[class]...)
	}
	routing["rules"] = orderedRules

	reverseObj := ensureObject(document, "reverse")
	updateReverseBridges(reverseObj, sortedReverseChannels(reverse))

	if same, err := jsonConfigEqual(path, document); err != nil {
		return err
	} else if same {
		return nil
	}
	if err := configio.WriteJSON(path, document, configio.WriteOptions{
		AuditPath:         config.AuditLogPath(),
		KeepLastKnownGood: true,
	}); err != nil {
		return err
	}
	return nil
}
