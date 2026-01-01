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
	section, err := m.findInterceptSection()
	if err != nil {
		return err
	}
	if section != "" {
		return nil
	}
	created, err := captureCommand("uci", "add", "firewall", "redirect")
	if err != nil {
		return err
	}
	section = strings.TrimSpace(created)
	if section == "" {
		return fmt.Errorf("xp2p: unable to create firewall redirect section")
	}
	commands := []string{
		fmt.Sprintf("firewall.%s.name=Intercept-DNS", section),
		fmt.Sprintf("firewall.%s.family=any", section),
		fmt.Sprintf("firewall.%s.proto=tcp udp", section),
		fmt.Sprintf("firewall.%s.src=lan", section),
		fmt.Sprintf("firewall.%s.src_dport=53", section),
		fmt.Sprintf("firewall.%s.target=DNAT", section),
		fmt.Sprintf("firewall.%s.dest_port=53", section),
	}
	for _, cmd := range commands {
		if err := runCommand("uci", "set", cmd); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) removeIntercept() error {
	sections, err := m.findInterceptSections()
	if err != nil {
		return err
	}
	for _, section := range sections {
		if err := runCommand("uci", "delete", fmt.Sprintf("firewall.%s", section)); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) interceptPresent() bool {
	section, err := m.findInterceptSection()
	if err != nil {
		return false
	}
	return section != ""
}

func (m *Manager) findInterceptSection() (string, error) {
	sections, err := m.findInterceptSections()
	if err != nil {
		return "", err
	}
	if len(sections) == 0 {
		return "", nil
	}
	return sections[0], nil
}

func (m *Manager) findInterceptSections() ([]string, error) {
	out, err := captureCommand("uci", "show", "firewall")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	sections := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, ".name=") {
			continue
		}
		if !strings.HasSuffix(line, "='Intercept-DNS'") && !strings.HasSuffix(line, "=\"Intercept-DNS\"") {
			continue
		}
		prefix := strings.SplitN(line, ".name=", 2)
		if len(prefix) != 2 {
			continue
		}
		section := strings.TrimPrefix(prefix[0], "firewall.")
		if section != "" {
			sections = append(sections, section)
		}
	}
	return sections, nil
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
