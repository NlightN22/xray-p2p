//go:build linux

package dnsforward

import (
	"fmt"
	"strings"
)

func (m *Manager) upsertDNSMasq(domain, server string) error {
	section := dnsSectionName(domain)
	if err := runCommand("uci", "set", fmt.Sprintf("%s.%s=server", m.dnsConfig, section)); err != nil {
		return err
	}
	if err := runCommand("uci", "set", fmt.Sprintf("%s.%s.name=%s", m.dnsConfig, section, domain)); err != nil {
		return err
	}
	if err := runCommand("uci", "set", fmt.Sprintf("%s.%s.server=%s", m.dnsConfig, section, server)); err != nil {
		return err
	}
	if err := runCommand("uci", "set", fmt.Sprintf("%s.%s.xp2p=1", m.dnsConfig, section)); err != nil {
		return err
	}
	return nil
}

func (m *Manager) ensureIntercept() error {
	if m.interceptPresent() {
		return nil
	}
	name := "xp2p_dns_intercept"
	if err := runCommand("uci", "set", fmt.Sprintf("firewall.%s=redirect", name)); err != nil {
		return err
	}
	commands := []string{
		fmt.Sprintf("firewall.%s.name=xp2p_dns_intercept", name),
		fmt.Sprintf("firewall.%s.src=lan", name),
		fmt.Sprintf("firewall.%s.src_dport=53", name),
		fmt.Sprintf("firewall.%s.dest_port=53", name),
		fmt.Sprintf("firewall.%s.dest_ip=127.0.0.1", name),
		fmt.Sprintf("firewall.%s.proto=tcpudp", name),
		fmt.Sprintf("firewall.%s.target=DNAT", name),
		fmt.Sprintf("firewall.%s.xp2p=1", name),
	}
	for _, cmd := range commands {
		if err := runCommand("uci", "set", cmd); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) removeIntercept() error {
	name := "xp2p_dns_intercept"
	return runCommand("uci", "delete", fmt.Sprintf("firewall.%s", name))
}

func (m *Manager) interceptPresent() bool {
	out, err := captureCommand("uci", "show", "firewall")
	if err != nil {
		return false
	}
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "firewall.xp2p_dns_intercept") {
			if strings.HasSuffix(line, "=redirect") {
				return true
			}
		}
		if strings.Contains(line, "xp2p_dns_intercept.xp2p='1'") {
			return true
		}
	}
	return false
}

func (m *Manager) commitDNS() error {
	return runCommand("uci", "commit", m.dnsConfig)
}

func (m *Manager) reloadDNS() error {
	return runCommand("/etc/init.d/dnsmasq", "reload")
}

func (m *Manager) commitFirewall() error {
	return runCommand("uci", "commit", "firewall")
}

func (m *Manager) reloadFirewall() error {
	if err := runCommand("fw4", "reload"); err == nil {
		return nil
	}
	return runCommand("/etc/init.d/firewall", "reload")
}

func (m *Manager) readDNSSections() (map[string]uciSection, error) {
	out, err := captureCommand("uci", "show", m.dnsConfig)
	if err != nil {
		return nil, err
	}
	sections := parseUCIShow(out)
	return sections, nil
}
