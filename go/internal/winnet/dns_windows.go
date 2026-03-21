//go:build windows

package winnet

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type DNSServers struct {
	IPv4 []string `json:"IPv4"`
	IPv6 []string `json:"IPv6"`
}

func GetDNSServers(ctx context.Context, adapterName string) (DNSServers, error) {
	escaped := escapePowerShellString(adapterName)
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$v4 = Get-DnsClientServerAddress -InterfaceAlias '` + escaped + `' -AddressFamily IPv4 -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`$v6 = Get-DnsClientServerAddress -InterfaceAlias '` + escaped + `' -AddressFamily IPv6 -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`$out = [pscustomobject]@{ IPv4 = @(); IPv6 = @() }`,
		`if ($null -ne $v4) { $out.IPv4 = $v4.ServerAddresses }`,
		`if ($null -ne $v6) { $out.IPv6 = $v6.ServerAddresses }`,
		`$out | ConvertTo-Json -Compress`,
	}, "; ")
	raw, err := runPowerShell(ctx, script)
	if err != nil {
		return DNSServers{}, err
	}
	var servers DNSServers
	if err := json.Unmarshal([]byte(raw), &servers); err != nil {
		return DNSServers{}, fmt.Errorf("xp2p: parse dns servers: %w", err)
	}
	return servers, nil
}

func SetDNSServers(ctx context.Context, adapterName string, servers DNSServers) error {
	escaped := escapePowerShellString(adapterName)
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$alias = "` + escaped + `"`,
		buildDNSSetScript("$alias", "IPv4", servers.IPv4),
		buildDNSSetScript("$alias", "IPv6", servers.IPv6),
	}, "; ")
	_, err := runPowerShell(ctx, script)
	return err
}

func buildDNSSetScript(alias string, family string, servers []string) string {
	if len(servers) == 0 {
		return `Set-DnsClientServerAddress -InterfaceAlias ` + alias + ` -AddressFamily ` + family + ` -ResetServerAddresses -ErrorAction SilentlyContinue | Out-Null`
	}
	parts := make([]string, 0, len(servers))
	for _, server := range servers {
		trimmed := strings.TrimSpace(server)
		if trimmed == "" {
			continue
		}
		parts = append(parts, "'"+escapePowerShellString(trimmed)+"'")
	}
	if len(parts) == 0 {
		return `Set-DnsClientServerAddress -InterfaceAlias ` + alias + ` -AddressFamily ` + family + ` -ResetServerAddresses -ErrorAction SilentlyContinue | Out-Null`
	}
	return `Set-DnsClientServerAddress -InterfaceAlias ` + alias + ` -AddressFamily ` + family + ` -ServerAddresses @(` + strings.Join(parts, ",") + `) -ErrorAction Stop | Out-Null`
}
