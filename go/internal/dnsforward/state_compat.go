//go:build linux

package dnsforward

import (
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/normalize"
	"github.com/NlightN22/xray-p2p/go/internal/version"
)

var currentAppVersion = version.Current

func dnsForwardStateCompatibilityRules() []normalize.Rule[rawState] {
	return []normalize.Rule[rawState]{
		autoForwardCompatibilityRule(),
	}
}

func autoForwardCompatibilityRule() normalize.Rule[rawState] {
	rule := normalize.Rule[rawState]{
		Name:            "dns-forward-state.auto-forward-owner",
		Description:     "Treat legacy auto_forward as dns-forward ownership.",
		DeprecatedSince: "0.2.7",
		RemovedSince:    "0.2.8",
		RemovalNote:     "Use forward_owner instead.",
	}
	rule.Apply = func(raw *rawState, report *normalize.Report) error {
		return applyAutoForwardCompatibility(raw, report, rule.RemovedSince)
	}
	return rule
}

func applyAutoForwardCompatibility(raw *rawState, report *normalize.Report, removedSince string) error {
	for domain, entry := range raw.Entries {
		if entry.AutoForward == nil {
			continue
		}
		if version.AtLeast(currentAppVersion(), removedSince) {
			return fmt.Errorf("dns-forward state entry %s uses removed field auto_forward; use forward_owner instead", domain)
		}
		report.AddDeprecatedField("entries." + domain + ".auto_forward")
		report.AddAppliedRule("dns-forward-state.auto-forward-owner")

		legacyOwner := ""
		if *entry.AutoForward {
			legacyOwner = forwardOwnerDNSForward
		}
		if entry.ForwardOwner != "" && entry.ForwardOwner != legacyOwner {
			return fmt.Errorf("dns-forward state entry %s has conflicting auto_forward and forward_owner", domain)
		}
		if entry.ForwardOwner == "" {
			entry.ForwardOwner = legacyOwner
		}
		entry.AutoForward = nil
		raw.Entries[domain] = entry
	}
	return nil
}
