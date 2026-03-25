//go:build windows

package winnet

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
)

// DefaultSendThroughIPv4 returns the IPv4 address associated with the best default route.
func DefaultSendThroughIPv4(ctx context.Context) (string, error) {
	script := strings.Join([]string{
		`$ProgressPreference = "SilentlyContinue"`,
		`$ErrorActionPreference = "Stop"`,
		`$def = $null`,
		`try { $def = Get-NetRoute -DestinationPrefix "0.0.0.0/0" -AddressFamily IPv4 -ErrorAction Stop | Where-Object { $_.NextHop -ne "0.0.0.0" } | Sort-Object RouteMetric,ifMetric | Select-Object -First 1 } catch { $def = $null }`,
		`if ($null -eq $def) { exit 0 }`,
		`$ip = Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex $def.ifIndex | Where-Object { $_.IPAddress -notlike "169.254.*" -and $_.IPAddress -ne "127.0.0.1" -and $_.IPAddress -ne "0.0.0.0" } | Select-Object -First 1 -ExpandProperty IPAddress`,
		`if ($null -ne $ip) { Write-Output $ip }`,
	}, "; ")

	out, err := runPowerShell(ctx, script)
	if err != nil {
		if errors.Is(err, errPowerShellNotFound) {
			return "", errPowerShellNotFound
		}
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "", nil
	}
	for _, token := range strings.Fields(out) {
		candidate := strings.Trim(token, "\"' \r\n\t")
		if candidate == "" {
			continue
		}
		ip := net.ParseIP(candidate)
		if ip != nil && ip.To4() != nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("xp2p: unexpected IPv4 address output %q", strings.TrimSpace(out))
}
