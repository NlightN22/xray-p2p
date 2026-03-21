//go:build windows

package winnet

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Route struct {
	DestinationPrefix string `json:"DestinationPrefix"`
	NextHop           string `json:"NextHop"`
	InterfaceIndex    int    `json:"InterfaceIndex"`
	RouteMetric       int    `json:"RouteMetric"`
	PolicyStore       string `json:"PolicyStore"`
	AddressFamily     string `json:"AddressFamily"`
}

func DefaultRoutes(ctx context.Context) ([]Route, error) {
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
		return 0, fmt.Errorf("xp2p: interface %s not found", name)
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
	return err
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
	script := strings.Join([]string{
		`$ErrorActionPreference = "Stop"`,
		`$dest = "` + escapePowerShellString(dest) + `"`,
		`$ifIndex = ` + strconv.Itoa(route.InterfaceIndex),
		`$nextHop = "` + escapePowerShellString(nextHop) + `"`,
		`Remove-NetRoute -DestinationPrefix $dest -InterfaceIndex $ifIndex -NextHop $nextHop -Confirm:$false -ErrorAction SilentlyContinue | Out-Null`,
	}, "; ")
	_, err := runPowerShell(ctx, script)
	return err
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
