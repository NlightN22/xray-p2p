//go:build linux

package dnsforward

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var domainPattern = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)

func detectDNSConfig() (string, error) {
	if override := strings.TrimSpace(os.Getenv("XP2P_DNSFORWARD_CONFIG")); override != "" {
		return override, nil
	}
	if hasFile("/etc/config/dnsmasq") {
		return "dnsmasq", nil
	}
	if hasFile("/etc/config/dhcp") {
		return "dhcp", nil
	}
	return "", errors.New("xp2p: dnsmasq config not found (expected /etc/config/dnsmasq or /etc/config/dhcp)")
}

func ensureOpenWrt() error {
	if _, err := exec.LookPath("uci"); err != nil {
		return errors.New("xp2p: uci command not found (OpenWrt required)")
	}
	if os.Getenv("XP2P_DNSFORWARD_FORCE_OPENWRT") == "1" {
		return nil
	}
	if hasFile("/etc/openwrt_release") || hasFile("/etc/openwrt_version") {
		return nil
	}
	return nil
}

func hasFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func normalizeDomain(domain string) (string, error) {
	trimmed := strings.TrimSpace(domain)
	if trimmed == "" {
		return "", errors.New("xp2p: --domain is required")
	}
	if strings.HasPrefix(trimmed, ".") || strings.HasSuffix(trimmed, ".") {
		trimmed = strings.Trim(trimmed, ".")
	}
	if !domainPattern.MatchString(trimmed) {
		return "", fmt.Errorf("xp2p: invalid domain %q", domain)
	}
	return strings.ToLower(trimmed), nil
}

func parseTarget(target string) (netip.Addr, int, error) {
	addrPort, err := netip.ParseAddrPort(strings.TrimSpace(target))
	if err != nil {
		return netip.Addr{}, 0, fmt.Errorf("xp2p: invalid --target %q: %w", target, err)
	}
	return addrPort.Addr(), int(addrPort.Port()), nil
}

func dnsSectionName(domain string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r >= 'A' && r <= 'Z':
			return r + 32
		}
		return '_'
	}, domain)
	return "xp2p_dns_" + safe
}

func baseDomain(domain string) string {
	trimmed := strings.TrimSpace(domain)
	if trimmed == "" {
		return ""
	}
	idx := strings.Index(trimmed, ".")
	if idx <= 0 {
		return trimmed
	}
	return trimmed[idx+1:]
}
