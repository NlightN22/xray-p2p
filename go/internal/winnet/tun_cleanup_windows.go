//go:build windows

package winnet

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NlightN22/xray-p2p/go/internal/logging"
)

func RemoveTunAdapters(ctx context.Context, prefix string) (int, error) {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		return 0, nil
	}
	escaped := escapePowerShellString(trimmed)
	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$prefix = '%s'
try {
    $adapters = Get-NetAdapter -IncludeHidden -ErrorAction Stop
} catch {
    $adapters = Get-NetAdapter -ErrorAction SilentlyContinue
}
$targets = $adapters | Where-Object {
    $_.Name -like "$prefix*" -or
    $_.Name -like "*Xray Tunnel*" -or
    $_.InterfaceDescription -like "*Xray Tunnel*"
}
$targetsJson = $targets | Select-Object Name, InterfaceDescription, Status, ifIndex | ConvertTo-Json -Compress
if (-not $targetsJson) { $targetsJson = "[]" }
$count = 0
$removed = @()
$failed = @()
foreach ($adapter in $targets) {
    try {
        Remove-NetAdapter -Name $adapter.Name -Confirm:$false -ErrorAction SilentlyContinue | Out-Null
        $count++
        $removed += $adapter.Name
    } catch {
        $failed += $adapter.Name
        continue
    }
}
if ($removed.Count -eq 0) { $removedJson = "[]" } else { $removedJson = $removed | ConvertTo-Json -Compress }
if ($failed.Count -eq 0) { $failedJson = "[]" } else { $failedJson = $failed | ConvertTo-Json -Compress }
Write-Output ("targets=" + $targetsJson)
Write-Output ("removed=" + $removedJson)
Write-Output ("failed=" + $failedJson)
Write-Output ("count=" + $count)
`, escaped)
	out, err := runPowerShell(ctx, script)
	if err != nil {
		if errorsIsPowerShellUnsupported(err) {
			return 0, nil
		}
		return 0, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		logging.Debug("xp2p: tun adapter cleanup trace", "line", trimmedLine)
	}
	if len(lines) == 0 {
		return 0, nil
	}
	count := 0
	for i := len(lines) - 1; i >= 0; i-- {
		trimmedLine := strings.TrimSpace(lines[i])
		if trimmedLine == "" {
			continue
		}
		if strings.HasPrefix(trimmedLine, "count=") {
			value := strings.TrimPrefix(trimmedLine, "count=")
			if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				count = parsed
				break
			}
		}
		if parsed, err := strconv.Atoi(trimmedLine); err == nil {
			count = parsed
			break
		}
	}
	return count, nil
}

func errorsIsPowerShellUnsupported(err error) bool {
	return err != nil && (errors.Is(err, errPowerShellNotFound) || errors.Is(err, errPowerShellUnsupported))
}
