//go:build linux

package dnsforward

import (
	"fmt"

	"github.com/NlightN22/xray-p2p/go/internal/normalize"
)

func normalizeState(raw rawState) (state, normalize.Report, error) {
	pipeline := normalize.Pipeline[rawState, state]{
		Defaults: defaultState,
		Rules:    dnsForwardStateCompatibilityRules(),
		Validate: validateState,
		Build:    buildState,
	}
	return pipeline.Normalize(raw)
}

func defaultState(raw *rawState) {
	if raw.Entries == nil {
		raw.Entries = make(map[string]rawStateEntry)
	}
}

func validateState(raw rawState) error {
	for domain, entry := range raw.Entries {
		if entry.ForwardOwner != "" && entry.ForwardOwner != forwardOwnerDNSForward {
			return fmt.Errorf("dns-forward state entry %s has invalid forward_owner %q", domain, entry.ForwardOwner)
		}
	}
	return nil
}

func buildState(raw rawState) (state, error) {
	entries := make(map[string]stateEntry, len(raw.Entries))
	for domain, entry := range raw.Entries {
		entries[domain] = stateEntry{
			Target:            entry.Target,
			Server:            entry.Server,
			ForwardListenPort: entry.ForwardListenPort,
			ForwardTag:        entry.ForwardTag,
			ForwardOwner:      entry.ForwardOwner,
			RebindDomain:      entry.RebindDomain,
		}
	}
	return state{Entries: entries}, nil
}
