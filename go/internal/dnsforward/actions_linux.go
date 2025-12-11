//go:build linux

package dnsforward

import (
	"fmt"
	"os"
	"strings"
)

func (m *Manager) upsertDNSMasq(domain, server string) error {
	section, err := m.dnsmasqSection()
	if err != nil {
		return err
	}
	value := fmt.Sprintf("/%s/%s", domain, server)
	// ensure idempotency
	_ = runCommand("uci", "del_list", fmt.Sprintf("%s.%s.server=%s", m.dnsConfig, section, value))
	return runCommand("uci", "add_list", fmt.Sprintf("%s.%s.server=%s", m.dnsConfig, section, value))
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
	return runCommand(dnsmasqServicePath(), "reload")
}

func (m *Manager) commitFirewall() error {
	return runCommand("uci", "commit", "firewall")
}

func (m *Manager) reloadFirewall() error {
	if err := runCommand("fw4", "reload"); err == nil {
		return nil
	}
	return runCommand(firewallServicePath(), "reload")
}

func (m *Manager) readDNSSections() (map[string]uciSection, error) {
	out, err := captureCommand("uci", "show", m.dnsConfig)
	if err != nil {
		return nil, err
	}
	sections := parseUCIShow(out)
	return sections, nil
}

func (m *Manager) ensureRebindAllowed(domain string) error {
	if domain == "" {
		return nil
	}
	section, err := m.dnsmasqSection()
	if err != nil {
		return err
	}
	if m.rebindPresent(section, domain) {
		return nil
	}
	if err := runCommand("uci", "set", fmt.Sprintf("%s.%s.rebind_protection=0", m.dnsConfig, section)); err != nil {
		return err
	}
	return runCommand("uci", "add_list", fmt.Sprintf("%s.%s.rebind_domain=%s", m.dnsConfig, section, domain))
}

func (m *Manager) removeRebind(domain string) error {
	if domain == "" {
		return nil
	}
	section, err := m.dnsmasqSection()
	if err != nil {
		return err
	}
	return runCommand("uci", "del_list", fmt.Sprintf("%s.%s.rebind_domain=%s", m.dnsConfig, section, domain))
}

func (m *Manager) rebindPresent(section, domain string) bool {
	out, err := captureCommand("uci", "show", fmt.Sprintf("%s.%s.rebind_domain", m.dnsConfig, section))
	if err != nil {
		return false
	}
	for _, val := range parseUCIValues(out) {
		if strings.EqualFold(val, domain) {
			return true
		}
	}
	return false
}

func (m *Manager) rebindInUse(target string, removing []string, sections map[string]uciSection, state state) bool {
	for _, entry := range sections {
		if !entry.isManagedDNS() {
			continue
		}
		name := entry.option("name")
		if name == "" {
			continue
		}
		base := baseDomain(name)
		if base == "" {
			continue
		}
		if base == target && !contains(removing, name) {
			return true
		}
	}
	for domain, entry := range state.Entries {
		if contains(removing, domain) {
			continue
		}
		if baseDomain(domain) == target {
			return true
		}
		if entry.RebindDomain != "" && strings.EqualFold(entry.RebindDomain, target) {
			return true
		}
	}
	return false
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func (m *Manager) dnsmasqSection() (string, error) {
	sections, err := m.readDNSSections()
	if err == nil {
		for name, sec := range sections {
			if sec.kind == "dnsmasq" {
				return name, nil
			}
		}
	}
	created, err := captureCommand("uci", "add", m.dnsConfig, "dnsmasq")
	if err != nil {
		return "", err
	}
	if name := strings.TrimSpace(created); name != "" {
		sections, err = m.readDNSSections()
		if err == nil {
			if sec, ok := sections[name]; ok && sec.kind == "dnsmasq" {
				return name, nil
			}
		}
	}
	sections, err = m.readDNSSections()
	if err != nil {
		return "", err
	}
	for name, sec := range sections {
		if sec.kind == "dnsmasq" {
			return name, nil
		}
	}
	return "", fmt.Errorf("xp2p: dnsmasq section not found in %s", m.dnsConfig)
}

func dnsmasqServicePath() string {
	if val := strings.TrimSpace(os.Getenv("XP2P_DNSFORWARD_DNSMASQ_SERVICE")); val != "" {
		return val
	}
	return "/etc/init.d/dnsmasq"
}

func firewallServicePath() string {
	if val := strings.TrimSpace(os.Getenv("XP2P_DNSFORWARD_FIREWALL_SERVICE")); val != "" {
		return val
	}
	return "/etc/init.d/firewall"
}
