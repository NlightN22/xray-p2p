//go:build windows

package winnet

import (
	"context"
	"strings"
)

func ForceRemoveDefaultRoutesByPrefix(ctx context.Context, tunPrefix string, family string) error {
	prefix := strings.TrimSpace(tunPrefix)
	if prefix == "" {
		return nil
	}
	dest := "0.0.0.0/0"
	nextHop := "0.0.0.0"
	if strings.EqualFold(strings.TrimSpace(family), "IPv6") {
		dest = "::/0"
		nextHop = "::"
	}
	script := strings.Join([]string{
		`$ProgressPreference = "SilentlyContinue"`,
		`$ErrorActionPreference = "Stop"`,
		`$prefix = '` + escapePowerShellString(prefix) + `'`,
		`$dest = '` + escapePowerShellString(dest) + `'`,
		`$hop = '` + escapePowerShellString(nextHop) + `'`,
		`$routes = Get-NetRoute -DestinationPrefix $dest -ErrorAction SilentlyContinue | Where-Object { $_.NextHop -eq $hop -and $_.InterfaceAlias -like "$prefix*" }`,
		`foreach ($route in $routes) {`,
		`  $idx = $route.InterfaceIndex`,
		`  if (-not $idx) { continue }`,
		`  Remove-NetRoute -InterfaceIndex $idx -DestinationPrefix $dest -NextHop $hop -PolicyStore ActiveStore -Confirm:$false -ErrorAction SilentlyContinue | Out-Null`,
		`  Remove-NetRoute -InterfaceIndex $idx -DestinationPrefix $dest -NextHop $hop -PolicyStore PersistentStore -Confirm:$false -ErrorAction SilentlyContinue | Out-Null`,
		`}`,
		`Write-Output "ok"`,
	}, "; ")
	_, err := runPowerShell(ctx, script)
	return err
}
