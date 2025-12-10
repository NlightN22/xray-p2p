//go:build linux

package dnsforward

import (
	"fmt"
	"net/netip"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
)

func (m *Manager) ensureForward(addr netip.Addr, port int, state state) (forward.Rule, bool, error) {
	forwards, err := client.ListForwards(client.ForwardListOptions{
		InstallDir: m.installDir,
		ConfigDir:  m.configDir,
	})
	if err != nil {
		return forward.Rule{}, false, err
	}

	for _, rule := range forwards {
		if rule.TargetIP == addr.String() && rule.TargetPort == port && rule.Protocol.RequiresUDP() {
			return rule, false, nil
		}
	}

	if existing, ok := stateEntryForTarget(state, addr, port); ok {
		for _, rule := range forwards {
			if rule.ListenPort == existing.ForwardListenPort && rule.TargetIP == addr.String() && rule.Protocol.RequiresUDP() {
				return rule, false, nil
			}
		}
	}

	result, err := client.AddForward(client.ForwardAddOptions{
		InstallDir:    m.installDir,
		ConfigDir:     m.configDir,
		Target:        fmt.Sprintf("%s:%d", addr.String(), port),
		ListenAddress: forward.DefaultListenAddress,
		ListenPort:    0,
		Protocol:      forward.ProtocolBoth,
		BasePort:      forward.DefaultBasePort,
	})
	if err != nil {
		return forward.Rule{}, false, err
	}
	return result.Rule, true, nil
}

func stateEntryForTarget(state state, addr netip.Addr, port int) (stateEntry, bool) {
	for _, entry := range state.Entries {
		if entry.Target == fmt.Sprintf("%s:%d", addr.String(), port) {
			return entry, true
		}
	}
	return stateEntry{}, false
}
