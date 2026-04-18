package client

import (
	"encoding/json"
	"strings"
	"testing"
)

func compileDesiredDoc(t *testing.T, configPath string, extensionsDir string) map[string]any {
	t.Helper()
	artifacts, err := compileDesired(configPath, extensionsDir)
	if err != nil {
		t.Fatalf("compile desired: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(artifacts.XrayJSON, &doc); err != nil {
		t.Fatalf("parse xray.json: %v", err)
	}
	return doc
}

func extractRoutingRules(t *testing.T, doc map[string]any) []any {
	t.Helper()
	routing, ok := doc["routing"].(map[string]any)
	if !ok {
		t.Fatalf("expected routing section, got %T", doc["routing"])
	}
	rules, ok := routing["rules"].([]any)
	if !ok {
		t.Fatalf("expected routing.rules array, got %T", routing["rules"])
	}
	return rules
}

func hasRuleWithIP(rules []any, ip, tag string) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		outbound, _ := rule["outboundTag"].(string)
		if outbound != tag {
			continue
		}
		for _, value := range extractStringSlice(rule["ip"]) {
			if value == ip {
				return true
			}
		}
	}
	return false
}

func hasRuleWithDomains(rules []any, domain, tag string) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		outbound, _ := rule["outboundTag"].(string)
		if outbound != tag {
			continue
		}
		for _, value := range extractStringSlice(rule["domains"]) {
			if value == domain {
				return true
			}
		}
	}
	return false
}

func hasMarkerRule(rules []any) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, value := range extractStringSlice(rule["ip"]) {
			if strings.HasPrefix(value, "127.255.") {
				return true
			}
		}
	}
	return false
}

func hasAnyDomainRedirectRule(rules []any) bool {
	for _, raw := range rules {
		rule, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if len(extractStringSlice(rule["domains"])) > 0 {
			return true
		}
	}
	return false
}
