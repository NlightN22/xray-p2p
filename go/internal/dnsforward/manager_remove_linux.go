//go:build linux

package dnsforward

import (
	"errors"
	"fmt"
	"sort"
)

func (m *Manager) Remove(opts RemoveOptions) ([]string, error) {
	if err := ensureOpenWrt(); err != nil {
		return nil, err
	}
	state, err := loadState(m.statePath)
	if err != nil {
		return nil, err
	}
	stateChanged := false

	var domains []string
	if opts.All {
		for domain := range state.Entries {
			domains = append(domains, domain)
		}
	} else {
		domain, err := normalizeDomain(opts.Domain)
		if err != nil {
			return nil, err
		}
		domains = []string{domain}
	}

	if len(domains) == 0 {
		return nil, errors.New("no dns-forward entries found")
	}

	removedCount := 0
	removedForwards := make(map[int]struct{})
	dnsSection, err := m.dnsmasqSection()
	if err != nil {
		return nil, err
	}
	for _, domain := range domains {
		rebind := baseDomain(domain)
		entry, hasState := state.Entries[domain]

		value := fmt.Sprintf("/%s/%s", domain, entry.Server)
		_ = runCommand("uci", "del_list", fmt.Sprintf("%s.%s.server=%s", m.dnsConfig, dnsSection, value))

		sections, err := m.readDNSSections()
		if err == nil {
			for name, sec := range sections {
				if !sec.isManagedDNS() {
					continue
				}
				if sec.option("name") != domain {
					continue
				}
				_ = runCommand("uci", "delete", fmt.Sprintf("%s.%s", m.dnsConfig, name))
			}
		}

		if shouldRemoveForwardOnDelete(entry, hasState, state, domains) {
			if _, removed := removedForwards[entry.ForwardListenPort]; !removed {
				m.removeForward(entry.ForwardListenPort)
				removedForwards[entry.ForwardListenPort] = struct{}{}
			}
		}
		if hasState {
			state.remove(domain)
			stateChanged = true
			removedCount++
		}
		if !m.rebindInUse(rebind, domains, sections, state) {
			_ = m.removeRebind(rebind)
		}
	}
	if removedCount == 0 && !opts.All {
		return nil, fmt.Errorf("dns-forward entry for %s not found", domains[0])
	}

	if err := m.commitDNS(); err != nil {
		return nil, err
	}
	if err := m.reloadDNS(); err != nil {
		return nil, err
	}

	if opts.Intercept {
		_ = m.removeIntercept()
		if err := m.commitFirewall(); err == nil {
			_ = m.reloadFirewall()
		}
	}

	if stateChanged {
		if err := state.save(m.statePath); err != nil {
			return nil, err
		}
	}

	sort.Strings(domains)
	return domains, nil
}

func forwardInUse(state state, listenPort int, removing []string) bool {
	if listenPort <= 0 {
		return false
	}
	for domain, entry := range state.Entries {
		if contains(removing, domain) {
			continue
		}
		if entry.ForwardListenPort == listenPort {
			return true
		}
	}
	return false
}

func shouldRemoveForwardOnDelete(entry stateEntry, hasState bool, state state, removing []string) bool {
	return hasState &&
		entry.ForwardListenPort > 0 &&
		entry.forwardOwnedByDNSForward() &&
		!forwardInUse(state, entry.ForwardListenPort, removing)
}

func shouldRemoveReplacedForward(previous stateEntry, hadPrevious bool, newListenPort int, state state) bool {
	return hadPrevious &&
		previous.ForwardListenPort > 0 &&
		previous.ForwardListenPort != newListenPort &&
		previous.forwardOwnedByDNSForward() &&
		!forwardInUse(state, previous.ForwardListenPort, nil)
}

func forwardOwnerForCreatedForward(created bool) string {
	if created {
		return forwardOwnerDNSForward
	}
	return ""
}
