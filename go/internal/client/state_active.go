package client

import (
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/redirect"
)

func activeClientEndpoints(endpoints []clientEndpointRecord) []clientEndpointRecord {
	active := make([]clientEndpointRecord, 0, len(endpoints))
	for _, ep := range endpoints {
		if !ep.Disabled {
			active = append(active, ep)
		}
	}
	return active
}

func activeClientRedirects(redirects []redirect.Rule, endpoints []clientEndpointRecord) []redirect.Rule {
	return activeClientRedirectsWithGroups(redirects, endpoints, nil)
}

func activeClientRedirectsWithGroups(redirects []redirect.Rule, endpoints []clientEndpointRecord, groups []endpointGroup) []redirect.Rule {
	activeTags := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		if ep.Disabled {
			continue
		}
		activeTags[strings.ToLower(strings.TrimSpace(ep.Tag))] = struct{}{}
	}
	for tag := range endpointGroupTags(groups) {
		activeTags[tag] = struct{}{}
	}
	active := make([]redirect.Rule, 0, len(redirects))
	for _, rule := range redirects {
		if rule.Disabled {
			continue
		}
		if _, ok := activeTags[strings.ToLower(strings.TrimSpace(rule.OutboundTag))]; ok {
			active = append(active, rule)
		}
	}
	return active
}

func activeClientReverseForRules(reverse map[string]clientReverseChannel, endpoints []clientEndpointRecord) map[string]clientReverseChannel {
	activeTags := make(map[string]struct{}, len(endpoints))
	for _, ep := range endpoints {
		if ep.Disabled {
			continue
		}
		activeTags[strings.ToLower(strings.TrimSpace(ep.Tag))] = struct{}{}
	}
	active := make(map[string]clientReverseChannel)
	for key, channel := range reverse {
		if channel.Disabled {
			continue
		}
		if _, ok := activeTags[strings.ToLower(strings.TrimSpace(channel.EndpointTag))]; ok {
			active[key] = channel
		}
	}
	return active
}
