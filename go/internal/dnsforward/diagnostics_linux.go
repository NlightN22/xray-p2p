//go:build linux

package dnsforward

import (
	"strings"
)

func (m *Manager) Diagnostics(includeFirewall bool) map[string]string {
	diag := make(map[string]string)
	if out, err := captureCommand("uci", "show", m.dnsConfig); err == nil {
		diag["uci show "+m.dnsConfig+" (xp2p)"] = filterXP2PLines(out)
	}
	if includeFirewall {
		if out, err := captureCommand("fw4", "status"); err == nil && strings.TrimSpace(out) != "" {
			diag["fw4 status"] = out
		} else if out, err := captureCommand("iptables", "-t", "nat", "-L"); err == nil {
			diag["iptables -t nat -L"] = out
		}
	}
	return diag
}

func filterXP2PLines(output string) string {
	lines := strings.Split(output, "\n")
	var filtered []string
	for _, line := range lines {
		if strings.Contains(line, "xp2p") {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) == 0 {
		return strings.TrimSpace(output)
	}
	return strings.Join(filtered, "\n")
}
