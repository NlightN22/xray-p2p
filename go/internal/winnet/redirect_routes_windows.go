//go:build windows

package winnet

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

var (
	ErrInterfaceMissing = errors.New("xp2p: tun interface not found")
	ErrTunIPv4Missing   = errors.New("xp2p: tun IPv4 address unavailable")
)

func SyncRedirectRoutes(tunName, tunAddr string, cidrs []string) error {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return nil
	}
	desired := normalizeCIDRs(cidrs)
	assignIP, assignPrefix, assignOK := parseTunAddr(tunAddr)
	script := buildSyncRoutesScript(name, desired, assignIP, assignPrefix, assignOK)
	out, err := runPowerShell(context.Background(), script)
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(out)
	switch strings.ToLower(trimmed) {
	case "missing-interface":
		return ErrInterfaceMissing
	case "missing-ip":
		return ErrTunIPv4Missing
	}
	if strings.HasPrefix(trimmed, "ok:") {
		addr := strings.TrimSpace(strings.TrimPrefix(trimmed, "ok:"))
		if addr != "" {
			ifIndex, _, _, err := InterfaceByNamePrefix(name)
			if err == nil && ifIndex > 0 {
				if details, stateErr := InterfaceIPv4Details(ifIndex); stateErr == nil {
					logging.Info(
						"xp2p: tun IPv4 available",
						"interface", name,
						"addr", addr,
						"operStatus", InterfaceOperStatusName(details.OperStatus),
						"dadState", InterfaceDadStateName(details.DadState),
					)
				} else {
					logging.Info("xp2p: tun IPv4 available", "interface", name, "addr", addr)
				}
			} else {
				logging.Info("xp2p: tun IPv4 available", "interface", name, "addr", addr)
			}
		}
	}
	return nil
}

func RemoveRedirectRoutes(tunName string, cidrs []string) error {
	name := strings.TrimSpace(tunName)
	if name == "" {
		return nil
	}
	desired := normalizeCIDRs(cidrs)
	script := buildRemoveRoutesScript(name, desired)
	out, err := runPowerShell(context.Background(), script)
	if err != nil {
		return err
	}
	if strings.EqualFold(strings.TrimSpace(out), "missing-interface") {
		return ErrInterfaceMissing
	}
	return nil
}

func normalizeCIDRs(cidrs []string) []string {
	seen := make(map[string]struct{}, len(cidrs))
	out := make([]string, 0, len(cidrs))
	for _, cidr := range cidrs {
		trimmed := strings.TrimSpace(cidr)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func parseTunAddr(tunAddr string) (string, int, bool) {
	trimmed := strings.TrimSpace(tunAddr)
	if trimmed == "" {
		return "", 0, false
	}
	ip, netmask, err := net.ParseCIDR(trimmed)
	if err != nil || ip == nil {
		return "", 0, false
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return "", 0, false
	}
	ones, _ := netmask.Mask.Size()
	if ones <= 0 {
		return "", 0, false
	}
	return ipv4.String(), ones, true
}

func buildSyncRoutesScript(tunName string, cidrs []string, assignIP string, assignPrefix int, assignOK bool) string {
	escapedName := escapePowerShellString(tunName)
	desired := buildPowerShellArray(cidrs)
	assignFlag := "$false"
	assignLine := ""
	if assignOK {
		assignFlag = "$true"
		assignLine = `New-NetIPAddress -InterfaceIndex $ifIndex -IPAddress '` + escapePowerShellString(assignIP) + `' -PrefixLength ` + strconv.Itoa(assignPrefix) + ` -PolicyStore ActiveStore -ErrorAction Stop | Out-Null`
	}

	lines := []string{
		`$ProgressPreference = "SilentlyContinue"`,
		`$ErrorActionPreference = "Stop"`,
		`$desired = ` + desired,
		`$adapter = Get-NetAdapter -Name '` + escapedName + `' -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`if ($null -eq $adapter) { Write-Output "missing-interface"; exit 0 }`,
		`$ifIndex = $adapter.ifIndex`,
		`$assignAddr = ` + assignFlag,
		`$ip = Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex $ifIndex -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -notlike "169.254.*" -and $_.IPAddress -ne "127.0.0.1" -and $_.IPAddress -ne "0.0.0.0" } | Sort-Object PrefixLength -Descending | Select-Object -First 1`,
	}
	if assignLine != "" {
		lines = append(lines,
			`if ($null -eq $ip -and $assignAddr) {`,
			`  $assignAttempts = 0`,
			`  while ($null -eq $ip -and $assignAttempts -lt 6) {`,
			`    $assignAttempts++`,
			`    try { `+assignLine+` } catch { }`,
			`    $ip = Get-NetIPAddress -AddressFamily IPv4 -InterfaceIndex $ifIndex -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -notlike "169.254.*" -and $_.IPAddress -ne "127.0.0.1" -and $_.IPAddress -ne "0.0.0.0" } | Sort-Object PrefixLength -Descending | Select-Object -First 1`,
			`    if ($null -eq $ip) { Start-Sleep -Milliseconds 500 }`,
			`  }`,
			`}`,
		)
	}
	lines = append(lines,
		`if ($null -eq $ip) { Write-Output "missing-ip"; exit 0 }`,
		`$ifPrefix = "$($ip.IPAddress)/$($ip.PrefixLength)"`,
		`$existing = Get-NetRoute -InterfaceIndex $ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue`,
		`foreach ($route in $existing) {`,
		`  if ($route.DestinationPrefix -eq $ifPrefix) { continue }`,
		`  if ($route.DestinationPrefix -like "169.254.*") { continue }`,
		`  if ($desired -contains $route.DestinationPrefix) { continue }`,
		`  Remove-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix $route.DestinationPrefix -Confirm:$false -ErrorAction SilentlyContinue | Out-Null`,
		`}`,
		`foreach ($cidr in $desired) {`,
		`  if ([string]::IsNullOrWhiteSpace($cidr)) { continue }`,
		`  $current = Get-NetRoute -InterfaceIndex $ifIndex -AddressFamily IPv4 -DestinationPrefix $cidr -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`  if ($null -eq $current) {`,
		`    New-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix $cidr -NextHop "0.0.0.0" -PolicyStore ActiveStore -ErrorAction Stop | Out-Null`,
		`    continue`,
		`  }`,
		`  if ($current.NextHop -ne "0.0.0.0") {`,
		`    Set-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix $cidr -NextHop "0.0.0.0" -ErrorAction SilentlyContinue | Out-Null`,
		`  }`,
		`}`,
		`Write-Output ("ok:" + $ifPrefix)`,
	)
	return strings.Join(lines, "; ")
}

func buildRemoveRoutesScript(tunName string, cidrs []string) string {
	escapedName := escapePowerShellString(tunName)
	desired := buildPowerShellArray(cidrs)
	lines := []string{
		`$ProgressPreference = "SilentlyContinue"`,
		`$ErrorActionPreference = "Stop"`,
		`$targets = ` + desired,
		`$adapter = Get-NetAdapter -Name '` + escapedName + `' -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`if ($null -eq $adapter) { Write-Output "missing-interface"; exit 0 }`,
		`$ifIndex = $adapter.ifIndex`,
		`foreach ($cidr in $targets) {`,
		`  if ([string]::IsNullOrWhiteSpace($cidr)) { continue }`,
		`  Remove-NetRoute -InterfaceIndex $ifIndex -DestinationPrefix $cidr -Confirm:$false -ErrorAction SilentlyContinue | Out-Null`,
		`}`,
		`Write-Output "ok"`,
	}
	return strings.Join(lines, "; ")
}

func buildPowerShellArray(values []string) string {
	if len(values) == 0 {
		return "@()"
	}
	escaped := make([]string, 0, len(values))
	for _, value := range values {
		escaped = append(escaped, "'"+escapePowerShellString(value)+"'")
	}
	return "@(" + strings.Join(escaped, ",") + ")"
}
