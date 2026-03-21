//go:build windows

package client

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
	"github.com/NlightN22/xray-p2p/go/internal/winnet"
)

func applyWindowsDNS(ctx context.Context, tunName string, servers []string, verbose bool) (*fullTunnelDNSBackup, error) {
	if len(servers) == 0 || strings.TrimSpace(tunName) == "" {
		return nil, nil
	}
	backup, err := winnet.GetDNSServers(ctx, tunName)
	if err != nil {
		return nil, err
	}
	target := splitDNSServers(servers)
	if len(target.IPv4) == 0 && len(target.IPv6) == 0 {
		return nil, errors.New("xp2p: dns servers list is empty")
	}
	if err := winnet.SetDNSServers(ctx, tunName, target); err != nil {
		return nil, err
	}
	logFullTunnelDNSVerbose(verbose, "xp2p: full-tunnel DNS override applied", backup, target, tunName)
	logging.Info("xp2p: full-tunnel DNS servers applied", "interface", tunName)
	return &fullTunnelDNSBackup{
		WindowsIPv4: backup.IPv4,
		WindowsIPv6: backup.IPv6,
	}, nil
}

func restoreWindowsDNS(ctx context.Context, backup *fullTunnelDNSBackup, tunName string, verbose bool) error {
	if backup == nil || strings.TrimSpace(tunName) == "" {
		if verbose {
			logging.Info("xp2p: full-tunnel DNS unchanged (no backup)", "interface", tunName)
		}
		return nil
	}
	before := winnet.DNSServers{}
	if verbose {
		if current, err := winnet.GetDNSServers(ctx, tunName); err == nil {
			before = current
		}
	}
	target := winnet.DNSServers{
		IPv4: backup.WindowsIPv4,
		IPv6: backup.WindowsIPv6,
	}
	err := winnet.SetDNSServers(ctx, tunName, target)
	if err == nil {
		if verbose {
			logFullTunnelDNSVerbose(verbose, "xp2p: full-tunnel DNS restored", before, target, tunName)
		}
		logging.Info("xp2p: full-tunnel DNS servers restored", "interface", tunName)
	}
	return err
}

func splitDNSServers(servers []string) winnet.DNSServers {
	result := winnet.DNSServers{}
	for _, server := range servers {
		trimmed := strings.TrimSpace(server)
		if trimmed == "" {
			continue
		}
		if ip := net.ParseIP(trimmed); ip != nil {
			if ip.To4() != nil {
				result.IPv4 = append(result.IPv4, trimmed)
			} else {
				result.IPv6 = append(result.IPv6, trimmed)
			}
		}
	}
	return result
}

func logFullTunnelDNSVerbose(enabled bool, message string, before winnet.DNSServers, after winnet.DNSServers, tunName string) {
	if !enabled {
		return
	}
	logging.Info(message,
		"interface", tunName,
		"ipv4_before", before.IPv4,
		"ipv6_before", before.IPv6,
		"ipv4_after", after.IPv4,
		"ipv6_after", after.IPv6,
	)
}
