//go:build linux

package dnsforward

import (
	"context"
	"fmt"
)

func (m *Manager) Add(ctx context.Context, opts AddOptions) (ListEntry, error) {
	if err := ensureOpenWrt(); err != nil {
		return ListEntry{}, err
	}
	domain, err := normalizeDomain(opts.Domain)
	if err != nil {
		return ListEntry{}, err
	}
	rebind := baseDomain(domain)
	targetAddr, targetPort, err := parseTarget(opts.Target)
	if err != nil {
		return ListEntry{}, err
	}

	state, _ := loadState(m.statePath)
	serverIP := targetAddr.String()
	serverPort := targetPort
	labels := []string{"xp2p"}
	stateChanged := false

	if opts.WithForward {
		rule, created, err := m.ensureForward(targetAddr, targetPort, state)
		if err != nil {
			return ListEntry{}, err
		}
		serverIP = rule.ListenAddress
		serverPort = rule.ListenPort
		if created {
			labels = append(labels, "forward:auto")
		} else {
			labels = append(labels, "forward:existing")
		}
		state.record(domain, stateEntry{
			Target:            fmt.Sprintf("%s:%d", targetAddr.String(), targetPort),
			Server:            fmt.Sprintf("%s#%d", serverIP, serverPort),
			ForwardListenPort: rule.ListenPort,
			ForwardTag:        rule.Tag,
			AutoForward:       created,
			RebindDomain:      rebind,
		})
		stateChanged = true
	} else {
		forwards, err := m.listForwards()
		if err != nil {
			return ListEntry{}, err
		}
		rule, found, err := selectForward(forwards, opts.Quiet)
		if err != nil {
			return ListEntry{}, err
		}
		if !found {
			return ListEntry{}, fmt.Errorf("no forwards configured; add one or use --with-forward")
		}
		serverIP = rule.ListenAddress
		serverPort = rule.ListenPort
		labels = append(labels, "forward:existing")
		state.record(domain, stateEntry{
			Target:            fmt.Sprintf("%s:%d", targetAddr.String(), targetPort),
			Server:            fmt.Sprintf("%s#%d", serverIP, serverPort),
			ForwardListenPort: rule.ListenPort,
			ForwardTag:        rule.Tag,
			AutoForward:       false,
			RebindDomain:      rebind,
		})
		stateChanged = true
	}

	serverValue := fmt.Sprintf("%s#%d", serverIP, serverPort)
	if err := m.upsertDNSMasq(domain, serverValue); err != nil {
		return ListEntry{}, err
	}
	if err := m.ensureRebindAllowed(rebind); err != nil {
		return ListEntry{}, err
	}

	if opts.Intercept {
		if err := m.ensureIntercept(); err != nil {
			return ListEntry{}, err
		}
		labels = append(labels, "intercept")
	}

	if err := m.commitDNS(); err != nil {
		return ListEntry{}, err
	}
	if err := m.reloadDNS(); err != nil {
		return ListEntry{}, err
	}
	if opts.Intercept {
		if err := m.commitFirewall(); err != nil {
			return ListEntry{}, err
		}
		if err := m.reloadFirewall(); err != nil {
			return ListEntry{}, err
		}
	}

	if stateChanged {
		if err := state.save(m.statePath); err != nil {
			return ListEntry{}, err
		}
	}

	entry := ListEntry{
		Domain: domain,
		Server: serverValue,
		Target: fmt.Sprintf("%s:%d", targetAddr.String(), targetPort),
		Labels: labels,
	}
	return entry, nil
}
