//go:build linux

package dnsforward

import (
	"sort"
)

func (m *Manager) List() ([]ListEntry, bool, error) {
	if err := ensureOpenWrt(); err != nil {
		return nil, false, err
	}
	state, err := loadState(m.statePath)
	if err != nil {
		return nil, false, err
	}

	intercept := m.interceptPresent()
	var entries []ListEntry
	for domain, s := range state.Entries {
		entry := ListEntry{
			Domain: domain,
			Server: s.Server,
			Target: s.Target,
			Labels: []string{"xp2p"},
		}
		if s.ForwardListenPort > 0 {
			if s.forwardOwnedByDNSForward() {
				entry.Labels = append(entry.Labels, "forward:auto")
			} else {
				entry.Labels = append(entry.Labels, "forward:recorded")
			}
		}
		if intercept {
			entry.Labels = append(entry.Labels, "intercept")
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Domain < entries[j].Domain })
	return entries, intercept, nil
}
