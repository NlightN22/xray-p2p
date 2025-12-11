//go:build linux

package dnsforward

import (
	"fmt"
	"net/netip"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/client"
	"github.com/NlightN22/xray-p2p/go/internal/forward"
	"github.com/NlightN22/xray-p2p/go/internal/server"
)

func (m *Manager) ensureForward(addr netip.Addr, port int, state state) (forward.Rule, bool, error) {
	forwards, err := m.listForwards()
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

	result, err := m.addForward(addr, port)
	if err != nil {
		return forward.Rule{}, false, err
	}
	return result, true, nil
}

func stateEntryForTarget(state state, addr netip.Addr, port int) (stateEntry, bool) {
	for _, entry := range state.Entries {
		if entry.Target == fmt.Sprintf("%s:%d", addr.String(), port) {
			return entry, true
		}
	}
	return stateEntry{}, false
}

func (m *Manager) listForwards() ([]forward.Rule, error) {
	switch strings.ToLower(strings.TrimSpace(m.forwardRole)) {
	case "server":
		return server.ListForwards(server.ForwardListOptions{
			InstallDir: m.installDir,
			ConfigDir:  m.configDir,
		})
	default:
		return client.ListForwards(client.ForwardListOptions{
			InstallDir: m.installDir,
			ConfigDir:  m.configDir,
		})
	}
}

func (m *Manager) addForward(addr netip.Addr, port int) (forward.Rule, error) {
	switch strings.ToLower(strings.TrimSpace(m.forwardRole)) {
	case "server":
		result, err := server.AddForward(server.ForwardAddOptions{
			InstallDir:    m.installDir,
			ConfigDir:     m.configDir,
			Target:        fmt.Sprintf("%s:%d", addr.String(), port),
			ListenAddress: forward.DefaultListenAddress,
			ListenPort:    0,
			Protocol:      forward.ProtocolBoth,
			BasePort:      forward.DefaultBasePort,
		})
		if err != nil {
			return forward.Rule{}, err
		}
		return result.Rule, nil
	default:
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
			return forward.Rule{}, err
		}
		return result.Rule, nil
	}
}

func (m *Manager) removeForward(listenPort int) {
	if listenPort <= 0 {
		return
	}
	switch strings.ToLower(strings.TrimSpace(m.forwardRole)) {
	case "server":
		_, _ = server.RemoveForward(server.ForwardRemoveOptions{
			InstallDir: m.installDir,
			ConfigDir:  m.configDir,
			Selector: forward.Selector{
				ListenPort: listenPort,
			},
		})
	default:
		_, _ = client.RemoveForward(client.ForwardRemoveOptions{
			InstallDir: m.installDir,
			ConfigDir:  m.configDir,
			Selector: forward.Selector{
				ListenPort: listenPort,
			},
		})
	}
}
