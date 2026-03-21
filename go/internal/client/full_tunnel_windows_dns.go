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

func applyWindowsDNS(ctx context.Context, tunName string, servers []string) (*fullTunnelDNSBackup, error) {
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
	logging.Info("xp2p: full-tunnel DNS servers applied", "interface", tunName)
	return &fullTunnelDNSBackup{
		WindowsIPv4: backup.IPv4,
		WindowsIPv6: backup.IPv6,
	}, nil
}

func restoreWindowsDNS(ctx context.Context, backup *fullTunnelDNSBackup, tunName string) error {
	if backup == nil || strings.TrimSpace(tunName) == "" {
		return nil
	}
	err := winnet.SetDNSServers(ctx, tunName, winnet.DNSServers{
		IPv4: backup.WindowsIPv4,
		IPv6: backup.WindowsIPv6,
	})
	if err == nil {
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
