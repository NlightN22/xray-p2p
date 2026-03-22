//go:build windows

package winnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Route struct {
	DestinationPrefix string `json:"DestinationPrefix"`
	NextHop           string `json:"NextHop"`
	InterfaceIndex    int    `json:"InterfaceIndex"`
	RouteMetric       int    `json:"RouteMetric"`
	PolicyStore       string `json:"PolicyStore"`
	AddressFamily     string `json:"AddressFamily"`
}

var ErrInterfaceNotFound = errors.New("xp2p: interface not found")

func DefaultRoutes(ctx context.Context) ([]Route, error) {
	routes, err := defaultRoutesFromIPHelper()
	if err == nil {
		return routes, nil
	}
	fallback, fallbackErr := defaultRoutesFromPowerShell(ctx)
	if fallbackErr != nil {
		return nil, fmt.Errorf("xp2p: route lookup failed: %v: %w", err, fallbackErr)
	}
	return fallback, nil
}

func defaultRoutesFromIPHelper() ([]Route, error) {
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_UNSPEC, &table); err != nil {
		return nil, err
	}
	if table == nil {
		return nil, nil
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))
	routes := make([]Route, 0, 2)
	for _, row := range table.Rows() {
		prefix, family, ok := ipPrefixFromRaw(row.DestinationPrefix)
		if !ok {
			continue
		}
		if prefix != "0.0.0.0/0" && prefix != "::/0" {
			continue
		}
		nextHop, _, _ := ipFromRaw(row.NextHop)
		routes = append(routes, Route{
			DestinationPrefix: prefix,
			NextHop:           nextHop,
			InterfaceIndex:    int(row.InterfaceIndex),
			RouteMetric:       int(row.Metric),
			PolicyStore:       "ActiveStore",
			AddressFamily:     family,
		})
	}
	return routes, nil
}

func ipPrefixFromRaw(prefix windows.IpAddressPrefix) (string, string, bool) {
	ip, family, ok := ipFromRaw(prefix.Prefix)
	if !ok || ip == "" {
		return "", "", false
	}
	return fmt.Sprintf("%s/%d", ip, prefix.PrefixLength), family, true
}

func ipFromRaw(addr windows.RawSockaddrInet) (string, string, bool) {
	switch addr.Family {
	case windows.AF_INET:
		raw := (*windows.RawSockaddrInet4)(unsafe.Pointer(&addr))
		ip := net.IP(raw.Addr[:]).To4()
		if ip == nil {
			return "", "", false
		}
		return ip.String(), "IPv4", true
	case windows.AF_INET6:
		raw := (*windows.RawSockaddrInet6)(unsafe.Pointer(&addr))
		ip := net.IP(raw.Addr[:])
		if ip == nil {
			return "", "", false
		}
		return ip.String(), "IPv6", true
	default:
		return "", "", false
	}
}

func defaultRoutesFromPowerShell(ctx context.Context) ([]Route, error) {
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$routes = @()`,
		`$routes += Get-NetRoute -DestinationPrefix "0.0.0.0/0" -ErrorAction SilentlyContinue | Select-Object DestinationPrefix,NextHop,InterfaceIndex,RouteMetric,PolicyStore,AddressFamily`,
		`$routes += Get-NetRoute -DestinationPrefix "::/0" -ErrorAction SilentlyContinue | Select-Object DestinationPrefix,NextHop,InterfaceIndex,RouteMetric,PolicyStore,AddressFamily`,
		`if ($routes.Count -eq 0) { Write-Output "" } else { $routes | ConvertTo-Json -Compress }`,
	}, "; ")
	out, err := runPowerShell(ctx, script)
	if err != nil {
		return nil, err
	}
	return decodeRoutes(out)
}

func InterfaceIndexByName(ctx context.Context, name string) (int, error) {
	escaped := escapePowerShellString(name)
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$adapter = Get-NetAdapter -Name '` + escaped + `' -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`if ($null -eq $adapter) { Write-Output "" } else { Write-Output $adapter.ifIndex }`,
	}, "; ")
	out, err := runPowerShell(ctx, script)
	if err != nil {
		return 0, err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return 0, fmt.Errorf("%w: %s", ErrInterfaceNotFound, name)
	}
	value, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("xp2p: parse interface index: %w", err)
	}
	return value, nil
}

func ApplyRoute(ctx context.Context, route Route) error {
	dest := strings.TrimSpace(route.DestinationPrefix)
	if dest == "" {
		return nil
	}
	nextHop := strings.TrimSpace(route.NextHop)
	if nextHop == "" {
		return fmt.Errorf("xp2p: next hop required for %s", dest)
	}
	policy := strings.TrimSpace(route.PolicyStore)
	if policy == "" {
		policy = "ActiveStore"
	}
	metric := ""
	if route.RouteMetric > 0 {
		metric = fmt.Sprintf(" -RouteMetric %d", route.RouteMetric)
	}
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$dest = "` + escapePowerShellString(dest) + `"`,
		`$nextHop = "` + escapePowerShellString(nextHop) + `"`,
		`$ifIndex = ` + strconv.Itoa(route.InterfaceIndex),
		`$existing = Get-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -ErrorAction SilentlyContinue | Select-Object -First 1`,
		`if ($null -eq $existing) {`,
		`  New-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -NextHop $nextHop -PolicyStore "` + escapePowerShellString(policy) + `"` + metric + ` -ErrorAction Stop | Out-Null`,
		`} else {`,
		`  Set-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -NextHop $nextHop -PolicyStore "` + escapePowerShellString(policy) + `"` + metric + ` -ErrorAction Stop | Out-Null`,
		`}`,
	}, "; ")
	_, err := runPowerShell(ctx, script)
	return wrapRouteError(err)
}

func RemoveRoute(ctx context.Context, route Route) error {
	dest := strings.TrimSpace(route.DestinationPrefix)
	if dest == "" {
		return nil
	}
	nextHop := strings.TrimSpace(route.NextHop)
	if nextHop == "" {
		nextHop = "0.0.0.0"
	}
	policy := strings.TrimSpace(route.PolicyStore)
	policyArg := ""
	if policy != "" {
		policyArg = ` -PolicyStore "` + escapePowerShellString(policy) + `"`
	}
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$dest = "` + escapePowerShellString(dest) + `"`,
		`$ifIndex = ` + strconv.Itoa(route.InterfaceIndex),
		`$nextHop = "` + escapePowerShellString(nextHop) + `"`,
		`Remove-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -NextHop $nextHop` + policyArg + ` -Confirm:$false -ErrorAction SilentlyContinue | Out-Null`,
	}, "; ")
	_, err := runPowerShell(ctx, script)
	return wrapRouteError(err)
}

func decodeRoutes(raw string) ([]Route, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var list []Route
	if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
		return list, nil
	}
	var single Route
	if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
		return nil, fmt.Errorf("xp2p: parse routes: %w", err)
	}
	return []Route{single}, nil
}

func wrapRouteError(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "access is denied") ||
		strings.Contains(lower, "access denied") ||
		strings.Contains(lower, "requested operation requires elevation") {
		return fmt.Errorf("xp2p: route change requires administrator privileges: %w", err)
	}
	return err
}
